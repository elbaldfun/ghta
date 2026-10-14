package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// Collections the fetch job shares with the old system, unchanged
// (docs/REBUILD.md §4.3): the metric time series and the per-shard progress.
const (
	CollSnapshots = "metric_snapshots"
	CollFetchRuns = "fetch_runs"
)

// fetchRunTTL is how long shard progress records are kept.
const fetchRunTTL = 30 * 24 * time.Hour

// Shard status values.
const (
	ShardRunning = "running"
	ShardDone    = "done"
	ShardFailed  = "failed"
)

func (s *Store) Snapshots() *mongo.Collection { return s.DB.Collection(CollSnapshots) }
func (s *Store) FetchRuns() *mongo.Collection { return s.DB.Collection(CollFetchRuns) }

func (s *Store) ensureFetchIndexes(ctx context.Context) error {
	_, err := s.FetchRuns().Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Same key and name as the old system's index, so both can ensure it.
		{Keys: bson.D{{Key: "source", Value: 1}, {Key: "date", Value: 1}, {Key: "shard", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_source_date_shard")},
		{Keys: bson.D{{Key: "startedAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(int32(fetchRunTTL / time.Second))},
	})
	if err != nil {
		return fmt.Errorf("fetch_runs indexes: %w", err)
	}
	return nil
}

// ShardStatus returns the recorded status of one shard on one UTC date, or ""
// when it has not been attempted.
func (s *Store) ShardStatus(ctx context.Context, source, date, shard string) (string, error) {
	var run struct {
		Status string `bson:"status"`
	}
	err := s.FetchRuns().FindOne(ctx, bson.M{"source": source, "date": date, "shard": shard}).Decode(&run)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("shard status: %w", err)
	}
	return run.Status, nil
}

// SetShardStatus records the progress of one shard.
func (s *Store) SetShardStatus(ctx context.Context, source, date, shard, status, errMsg string, at time.Time) error {
	set := bson.M{"status": status, "error": errMsg}
	if status == ShardRunning {
		set["startedAt"] = at.UTC()
	} else {
		set["endedAt"] = at.UTC()
	}
	_, err := s.FetchRuns().UpdateOne(ctx,
		bson.M{"source": source, "date": date, "shard": shard},
		bson.M{"$set": set},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("set shard status: %w", err)
	}
	return nil
}

// FetchStats counts what one WriteFetched call changed.
type FetchStats struct {
	Items     int // distinct items written
	New       int // of those, first seen
	Snapshots int // metric snapshots appended
	Blobs     int // README/release blobs written because their content changed
}

func (a *FetchStats) Add(b FetchStats) {
	a.Items += b.Items
	a.New += b.New
	a.Snapshots += b.Snapshots
	a.Blobs += b.Blobs
}

// snapshot is one metric_snapshots document.
type snapshot struct {
	Meta struct {
		Source     string `bson:"source"`
		ExternalID string `bson:"externalId"`
	} `bson:"meta"`
	Metrics    map[string]float64 `bson:"metrics"`
	CapturedAt time.Time          `bson:"capturedAt"`
}

func newSnapshot(source, externalID string, metrics map[string]float64, at time.Time) snapshot {
	snap := snapshot{Metrics: metrics, CapturedAt: at}
	snap.Meta.Source, snap.Meta.ExternalID = source, externalID
	return snap
}

// observe records today's value of the primary metric in a history ring and
// derives growth from it. firstToday reports that the ring had no point for
// today yet, which is when a metric snapshot is due.
func observe(history []item.Point, value float64, today string) (next []item.Point, growth item.Growth, firstToday bool) {
	firstToday = len(history) == 0 || history[len(history)-1].D != today
	next = item.SetPoint(history, item.Point{D: today, V: value})
	return next, item.ComputeGrowth(next, value, today), firstToday
}

const ageBatch = 500

// AgeGrowth re-measures the growth of the live items of one source that no
// fetch observed today, taking their last observed value as today's: an item
// out of the fetch range is counted as not growing, as the old metrics job
// counted it. Otherwise its growth would stay at what it was the last day it
// was seen, and an item that fell out of range would keep its place in the
// growth rankings until reconcile looks at it a week later. No history point
// is added — only an observation does that. Returns how many items changed.
func (s *Store) AgeGrowth(ctx context.Context, source, today string) (int, error) {
	cur, err := s.Items().Find(ctx,
		bson.M{"source": source, "stale": bson.M{"$ne": true}, "history.0": bson.M{"$exists": true}, "history.d": bson.M{"$ne": today}},
		options.Find().SetProjection(bson.M{"history": 1, "growth": 1}))
	if err != nil {
		return 0, fmt.Errorf("age growth: %w", err)
	}
	defer cur.Close(ctx)

	var models []mongo.WriteModel
	changed := 0
	flush := func() error {
		if len(models) == 0 {
			return nil
		}
		if _, err := s.Items().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
			return fmt.Errorf("age growth: %w", err)
		}
		changed += len(models)
		models = models[:0]
		return nil
	}
	for cur.Next(ctx) {
		var it struct {
			ID      any          `bson:"_id"`
			History []item.Point `bson:"history"`
			Growth  item.Growth  `bson:"growth"`
		}
		if err := cur.Decode(&it); err != nil {
			return changed, fmt.Errorf("age growth: %w", err)
		}
		g := item.ComputeGrowth(it.History, it.History[len(it.History)-1].V, today)
		if g.Equal(it.Growth) {
			continue
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": it.ID}).SetUpdate(bson.M{"$set": bson.M{"growth": g}}))
		if len(models) == ageBatch {
			if err := flush(); err != nil {
				return changed, err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return changed, fmt.Errorf("age growth: %w", err)
	}
	return changed, flush()
}

// WriteFetched stores one batch of freshly fetched items, all from one source.
//
// It writes only what the fetcher owns. On an existing item that is the
// descriptive fields, metrics, the gh/hf block, fetchedAt, and the history
// ring with the growth derived from it; labels, assets, ranks and job state
// are left as they are. A new item also gets its empty label set and a
// pending judge state — unless the source classified it, in which case its
// labels are stored and the judge is marked done.
//
// The first time an item is seen on a UTC day a metric snapshot is appended;
// later fetches the same day (overlapping shards) only refresh the item. A
// blob is written only when its content hash changed.
func (s *Store) WriteFetched(ctx context.Context, batch []item.Fetched, now time.Time) (FetchStats, error) {
	var stats FetchStats
	if len(batch) == 0 {
		return stats, nil
	}
	now = now.UTC()
	today := item.DayOf(now)
	source := batch[0].Item.Source

	// A shard can list the same item twice (search pages shift while paging);
	// the last occurrence wins.
	latest := make(map[string]int, len(batch))
	ids := make([]string, 0, len(batch))
	for i, f := range batch {
		if f.Item.Source != source {
			return stats, fmt.Errorf("write fetched: mixed sources %q and %q", source, f.Item.Source)
		}
		if _, dup := latest[f.Item.ExternalID]; !dup {
			ids = append(ids, f.Item.ExternalID)
		}
		latest[f.Item.ExternalID] = i
	}
	key := bson.M{"source": source, "externalId": bson.M{"$in": ids}}

	// What is already stored: each item's history ring and each blob's hash.
	histories := make(map[string][]item.Point, len(ids))
	cur, err := s.Items().Find(ctx, key, options.Find().SetProjection(bson.M{"externalId": 1, "history": 1}))
	if err != nil {
		return stats, fmt.Errorf("read histories: %w", err)
	}
	var known []struct {
		ExternalID string       `bson:"externalId"`
		History    []item.Point `bson:"history"`
	}
	if err := cur.All(ctx, &known); err != nil {
		return stats, fmt.Errorf("read histories: %w", err)
	}
	for _, k := range known {
		if k.History == nil {
			k.History = []item.Point{}
		}
		histories[k.ExternalID] = k.History
	}
	hashes := map[string]string{}
	cur, err = s.Blobs().Find(ctx, key, options.Find().SetProjection(bson.M{"externalId": 1, "hash": 1}))
	if err != nil {
		return stats, fmt.Errorf("read blob hashes: %w", err)
	}
	var blobs []struct {
		ExternalID string `bson:"externalId"`
		Hash       string `bson:"hash"`
	}
	if err := cur.All(ctx, &blobs); err != nil {
		return stats, fmt.Errorf("read blob hashes: %w", err)
	}
	for _, b := range blobs {
		hashes[b.ExternalID] = b.Hash
	}

	var itemWrites, blobWrites []mongo.WriteModel
	var snaps []any
	for _, id := range ids {
		f := batch[latest[id]]
		it := f.Item
		history, existed := histories[id]
		if !existed {
			stats.New++
		}

		set := bson.M{
			"name": it.Name, "description": it.Description, "topics": nonNil(it.Topics),
			"primaryMetric": it.PrimaryMetric, "metricDirection": it.MetricDirection,
			"metrics": it.Metrics, "fetchedAt": now, "updatedAt": now,
		}
		unset := bson.M{}
		optional := func(field, v string) {
			if v == "" {
				unset[field] = ""
			} else {
				set[field] = v
			}
		}
		optional("language", it.Language)
		optional("homepage", it.Homepage)
		optional("license", it.License)
		if it.Gh != nil {
			set["gh"] = it.Gh
		}
		if it.Hf != nil {
			set["hf"] = it.Hf
		}

		onInsert := bson.M{"createdAt": now}
		if len(it.Labels.Domain) > 0 {
			// Classified by the source itself: the labels are the fetcher's.
			set["labels.domain"] = it.Labels.Domain
			set["labels.by"] = it.Labels.By
			set["jobs.judge.status"] = item.StatusDone
		} else {
			onInsert["labels.domain"] = []string{}
			onInsert["jobs.judge.status"] = item.StatusPending
		}

		if value, ok := it.Metrics[it.PrimaryMetric]; ok {
			next, growth, firstToday := observe(history, value, today)
			set["history"], set["growth"] = next, growth
			if firstToday {
				snaps = append(snaps, newSnapshot(source, id, it.Metrics, now))
			}
		} else if !existed {
			onInsert["history"] = []item.Point{}
			onInsert["growth"] = item.Growth{}
		}

		update := bson.M{"$set": set, "$setOnInsert": onInsert}
		if len(unset) > 0 {
			update["$unset"] = unset
		}
		itemKey := bson.M{"source": source, "externalId": id}
		itemWrites = append(itemWrites, mongo.NewUpdateOneModel().SetFilter(itemKey).SetUpdate(update).SetUpsert(true))

		if b := f.Blob; b != nil && b.Hash != hashes[id] {
			blobWrites = append(blobWrites, mongo.NewReplaceOneModel().SetFilter(itemKey).SetUpsert(true).
				SetReplacement(item.Blob{
					Source: source, ExternalID: id, Readme: b.Readme, Releases: b.Releases, Hash: b.Hash, UpdatedAt: now,
				}))
		}
	}

	// Snapshots first: if the item write then fails, the shard is retried and
	// at worst appends a second snapshot for the day, which readers tolerate.
	// The other order could lose the day's snapshot for good.
	unordered := options.BulkWrite().SetOrdered(false)
	if len(snaps) > 0 {
		if _, err := s.Snapshots().InsertMany(ctx, snaps, options.InsertMany().SetOrdered(false)); err != nil {
			return stats, fmt.Errorf("append snapshots: %w", err)
		}
		stats.Snapshots = len(snaps)
	}
	if _, err := s.Items().BulkWrite(ctx, itemWrites, unordered); err != nil {
		return stats, fmt.Errorf("write items: %w", err)
	}
	stats.Items = len(itemWrites)
	if len(blobWrites) > 0 {
		if _, err := s.Blobs().BulkWrite(ctx, blobWrites, unordered); err != nil {
			return stats, fmt.Errorf("write blobs: %w", err)
		}
		stats.Blobs = len(blobWrites)
	}
	return stats, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
