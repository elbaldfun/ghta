package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
)

// UpsertItems writes a page of fetched items in a single bulkWrite, matching on
// (source, externalId). Only fetch-owned fields are $set so that categorization
// results (categoryId/categoryPath/analysisStatus) and computed trend metrics
// set by other jobs are preserved across re-fetches.
func (s *Store) UpsertItems(ctx context.Context, items []domain.TrackedItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	models := make([]mongo.WriteModel, 0, len(items))
	for _, it := range items {
		set := bson.M{
			"source":          it.Source,
			"externalId":      it.ExternalID,
			"name":            it.Name,
			"description":     it.Description,
			"language":        it.Language,
			"primaryMetric":   it.PrimaryMetric,
			"metricDirection": it.MetricDirection,
			"metrics":         it.Metrics,
			"sourceData":      it.SourceData,
			"fetchedAt":       now,
			"updatedAt":       now,
		}
		setOnInsert := bson.M{
			"categoryId":        []string{},
			"categoryPath":      []string{},
			"analysisStatus":    domain.AnalysisPending,
			"analysisFailCount": 0,
			"stale":             false, // live by default; the reconciler flips it to true
			"createdAt":         now,
		}
		// Sources that classify at fetch time (HF: categoryPath = "hf/<task>"
		// from the official pipeline tag) persist their own classification and
		// are marked done so the LLM categorizer never touches them. GitHub's
		// adapter sets no CategoryPath, so its categorizer-owned fields keep the
		// insert-only behavior above.
		if len(it.CategoryPath) > 0 {
			set["categoryPath"] = it.CategoryPath
			set["analysisStatus"] = domain.AnalysisDone
			delete(setOnInsert, "categoryPath")
			delete(setOnInsert, "analysisStatus")
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"source": it.Source, "externalId": it.ExternalID}).
			SetUpdate(bson.M{"$set": set, "$setOnInsert": setOnInsert}).
			SetUpsert(true))
	}

	res, err := s.Items().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, fmt.Errorf("bulk upsert items: %w", err)
	}
	return int(res.UpsertedCount + res.ModifiedCount), nil
}

// AppendSnapshots inserts one metric snapshot per item, skipping items that
// already have a snapshot for the current UTC day. Items are assumed to share a
// single source (an adapter fetches one source).
func (s *Store) AppendSnapshots(ctx context.Context, items []domain.TrackedItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	startOfDay := startOfUTCDay(time.Now())

	// Group externalIds by source and find which already have a snapshot today.
	bySource := map[domain.Source][]string{}
	for _, it := range items {
		bySource[it.Source] = append(bySource[it.Source], it.ExternalID)
	}
	existing := map[string]struct{}{}
	for src, ids := range bySource {
		cur, err := s.Snapshots().Find(ctx,
			bson.M{
				"capturedAt":      bson.M{"$gte": startOfDay},
				"meta.source":     src,
				"meta.externalId": bson.M{"$in": ids},
			},
			options.Find().SetProjection(bson.M{"meta.externalId": 1}),
		)
		if err != nil {
			return 0, fmt.Errorf("query today snapshots: %w", err)
		}
		var rows []domain.MetricSnapshot
		if err := cur.All(ctx, &rows); err != nil {
			return 0, fmt.Errorf("decode today snapshots: %w", err)
		}
		for _, r := range rows {
			// Key by the loop's source: meta.source may be unprojected here.
			existing[snapKey(src, r.Meta.ExternalID)] = struct{}{}
		}
	}

	now := time.Now().UTC()
	docs := make([]interface{}, 0, len(items))
	for _, it := range items {
		if _, seen := existing[snapKey(it.Source, it.ExternalID)]; seen {
			continue
		}
		docs = append(docs, domain.MetricSnapshot{
			Meta:       domain.SnapshotMeta{Source: it.Source, ExternalID: it.ExternalID},
			Metrics:    it.Metrics,
			CapturedAt: now,
		})
	}
	if len(docs) == 0 {
		return 0, nil
	}
	if _, err := s.Snapshots().InsertMany(ctx, docs, options.InsertMany().SetOrdered(false)); err != nil {
		return 0, fmt.Errorf("insert snapshots: %w", err)
	}
	return len(docs), nil
}

func startOfUTCDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func snapKey(s domain.Source, id string) string { return string(s) + "\x00" + id }

// StaleCandidates returns non-stale items of one source whose fetchedAt is
// older than cutoff, oldest first — the reconciler's work queue. A renamed or
// deleted repo stops appearing in search-based fetch shards, so its record
// silently freezes; this surfaces those records for verification.
func (s *Store) StaleCandidates(ctx context.Context, source domain.Source, cutoff time.Time, limit int) ([]domain.TrackedItem, error) {
	cur, err := s.Items().Find(ctx,
		bson.M{"source": source, "fetchedAt": bson.M{"$lt": cutoff}, "stale": false},
		options.Find().
			SetSort(bson.D{{Key: "fetchedAt", Value: 1}}).
			SetLimit(int64(limit)).
			SetProjection(bson.M{"externalId": 1, "name": 1, "fetchedAt": 1}))
	if err != nil {
		return nil, fmt.Errorf("stale candidates: %w", err)
	}
	items := []domain.TrackedItem{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// MarkItemStale tombstones a record the reconciler confirmed is gone from its
// source (deleted upstream, or a rename ghost whose new name is already
// tracked). Stale items stay in the collection for history but are excluded
// from every ranking.
func (s *Store) MarkItemStale(ctx context.Context, source domain.Source, externalID, reason string) error {
	now := time.Now().UTC()
	_, err := s.Items().UpdateOne(ctx,
		bson.M{"source": source, "externalId": externalID},
		bson.M{"$set": bson.M{"stale": true, "staleReason": reason, "staleAt": now, "updatedAt": now}})
	if err != nil {
		return fmt.Errorf("mark stale %s: %w", externalID, err)
	}
	return nil
}

// RenameItem moves a record to its new upstream identity (GitHub rename with
// no existing record under the new name), refreshing the live metrics in the
// same write. Returns mongo's duplicate-key error if the new externalId
// appeared concurrently; callers then tombstone the old record instead.
func (s *Store) RenameItem(ctx context.Context, source domain.Source, oldID, newID, newName string, metrics map[string]float64) error {
	now := time.Now().UTC()
	_, err := s.Items().UpdateOne(ctx,
		bson.M{"source": source, "externalId": oldID},
		bson.M{"$set": bson.M{
			"externalId": newID,
			"name":       newName,
			"metrics":    metrics,
			"fetchedAt":  now,
			"updatedAt":  now,
		}})
	if err != nil {
		return fmt.Errorf("rename %s -> %s: %w", oldID, newID, err)
	}
	return nil
}

// RefreshItemMetrics updates live metrics + fetchedAt for a record the
// reconciler verified still exists under the same name (it merely fell out of
// the fetch shards, e.g. below the star cutoff).
func (s *Store) RefreshItemMetrics(ctx context.Context, source domain.Source, externalID string, metrics map[string]float64) error {
	now := time.Now().UTC()
	_, err := s.Items().UpdateOne(ctx,
		bson.M{"source": source, "externalId": externalID},
		bson.M{"$set": bson.M{"metrics": metrics, "fetchedAt": now, "updatedAt": now}})
	if err != nil {
		return fmt.Errorf("refresh metrics %s: %w", externalID, err)
	}
	return nil
}

// ItemExists reports whether a record exists for (source, externalId).
func (s *Store) ItemExists(ctx context.Context, source domain.Source, externalID string) (bool, error) {
	err := s.Items().FindOne(ctx,
		bson.M{"source": source, "externalId": externalID},
		options.FindOne().SetProjection(bson.M{"_id": 1})).Err()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return false, err
}
