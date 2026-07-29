package job

import (
	"context"
	"log/slog"
	"sync/atomic"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/provider"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
)

const (
	// Small batches (review H2): the four-field output is 3–5× longer than the
	// alternatives prompt, and a truncated reply fails the whole call.
	storeBatchSize = 8
	// Bounded daily run; the backfill drains via RunUntilDone (admin-triggered).
	maxStorePerRun = 2000
	// A batch that fails this many whole calls (or an item omitted/invalid this
	// many times) is marked failed and parked until a version bump re-queues it —
	// the poison-batch guard. Transient relay outages cost one count, not a
	// permanent burial, and the counter is $inc (atomic), not read-modify-write.
	maxStoreFail = 5
)

// StoreFinder runs the app-store judgement (change 15): shelf category, zh/en
// taglines, GUI verdict, and the excluded cleanse — most-popular-first so the
// visible head of every shelf is judged earliest.
type StoreFinder struct {
	store   *repository.Store
	prov    provider.Provider
	log     *slog.Logger
	running atomic.Bool
}

func NewStoreFinder(store *repository.Store, prov provider.Provider, log *slog.Logger) *StoreFinder {
	return &StoreFinder{store: store, prov: prov, log: log}
}

// pendingFilter matches candidates not yet judged by the current store version:
// never judged, judged by an older version, or mid-failure-counting. done/failed
// at the current version are skipped (failed stays parked until a version bump).
func (f *StoreFinder) pendingFilter() bson.M {
	filter := repository.AppCandidateFilter()
	filter["$nor"] = bson.A{bson.M{
		"store.status":  bson.M{"$in": bson.A{"done", "failed"}},
		"store.version": bson.M{"$gte": service.CurrentStoreVersion},
	}}
	return filter
}

// Run judges up to maxStorePerRun pending items (the daily incremental pass).
// Returns how many items were processed (0 = nothing pending).
func (f *StoreFinder) Run(ctx context.Context) int {
	if !f.running.CompareAndSwap(false, true) {
		f.log.Warn("storefinder: run already in progress, skipping")
		return -1
	}
	defer f.running.Store(false)
	return f.runOnce(ctx)
}

// RunUntilDone drains the whole pending corpus — the backfill mechanism
// (review H1). Serial batches by design: the grok relay's concurrency ceiling is
// low and 429 storms are a documented failure mode here. Expected wall clock for
// the initial ~18k corpus at ~20–40s/call ≈ 12–25h; progress is logged every
// pass so an operator can watch it drain.
func (f *StoreFinder) RunUntilDone(ctx context.Context) {
	if !f.running.CompareAndSwap(false, true) {
		f.log.Warn("storefinder: run already in progress, skipping")
		return
	}
	defer f.running.Store(false)

	total := 0
	for {
		if ctx.Err() != nil {
			f.log.Warn("storefinder: drain cancelled", "processed", total)
			return
		}
		n := f.runOnce(ctx)
		if n <= 0 {
			break
		}
		total += n
		f.log.Info("storefinder: drain progress", "processedSoFar", total)
	}
	f.log.Info("storefinder: drain complete", "processed", total)
}

func (f *StoreFinder) runOnce(ctx context.Context) int {
	cur, err := f.store.Items().Find(ctx, f.pendingFilter(),
		options.Find().
			SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).
			SetLimit(maxStorePerRun))
	if err != nil {
		f.log.Error("storefinder: query failed", "err", err)
		return 0
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		f.log.Error("storefinder: decode failed", "err", err)
		return 0
	}
	if len(items) == 0 {
		return 0
	}

	done, excluded := 0, 0
	for start := 0; start < len(items); start += storeBatchSize {
		if ctx.Err() != nil {
			return done
		}
		end := start + storeBatchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]

		results, err := service.InferStore(ctx, f.prov, batch)
		if err != nil {
			// Whole-call failure: count it against every batch item, never
			// record verdicts (categorizer lesson: an outage is not a verdict).
			f.log.Warn("storefinder: batch call failed", "err", err, "size", len(batch))
			f.bumpFail(ctx, batch)
			continue
		}
		for _, it := range batch {
			r, ok := results[it.ExternalID]
			if !ok {
				// Omitted or invalid slug — item-level failure; batchmates proceed.
				f.bumpFail(ctx, []domain.TrackedItem{it})
				continue
			}
			set := bson.M{
				"store.category":  r.Category,
				"store.taglineZh": r.TaglineZh,
				"store.taglineEn": r.TaglineEn,
				"store.hasGui":    r.HasGui,
				"store.status":    "done",
				"store.version":   service.CurrentStoreVersion,
			}
			// Dotted paths on purpose: a whole-document $set on `store` would
			// wipe a manually-set categoryOverride.
			update := bson.M{"$set": set, "$unset": bson.M{"store.failCount": ""}}
			if _, err := f.store.Items().UpdateByID(ctx, it.ID, update); err != nil {
				f.log.Error("storefinder: update failed", "item", it.ExternalID, "err", err)
				continue
			}
			done++
			if r.Category == service.StoreExcluded {
				excluded++
			}
		}
	}
	f.log.Info("storefinder: pass done", "judged", done, "excluded", excluded, "of", len(items))
	return len(items)
}

// bumpFail atomically increments failCount for the items and parks any that
// crossed the threshold (status=failed at the current version).
func (f *StoreFinder) bumpFail(ctx context.Context, items []domain.TrackedItem) {
	ids := make([]any, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	if _, err := f.store.Items().UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		bson.M{"$inc": bson.M{"store.failCount": 1}},
	); err != nil {
		f.log.Error("storefinder: failCount inc failed", "err", err)
		return
	}
	if _, err := f.store.Items().UpdateMany(ctx,
		bson.M{
			"_id":             bson.M{"$in": ids},
			"store.failCount": bson.M{"$gte": maxStoreFail},
			"store.status":    bson.M{"$exists": false},
		},
		bson.M{"$set": bson.M{"store.status": "failed", "store.version": service.CurrentStoreVersion}},
	); err != nil {
		f.log.Error("storefinder: park failed items failed", "err", err)
	}
}
