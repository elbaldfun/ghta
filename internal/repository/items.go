package repository

import (
	"context"
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
			"createdAt":         now,
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
