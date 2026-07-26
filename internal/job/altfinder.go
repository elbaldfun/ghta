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
	maxAltPerRun = 600
	altBatchSize = 12
)

// AltFinder infers, for each app in the directory, which commercial products it
// is an open-source alternative to (change 13, LLM). It walks the app corpus
// most-popular-first, so the highest-traffic pages get data soonest.
type AltFinder struct {
	store   *repository.Store
	prov    provider.Provider
	log     *slog.Logger
	running atomic.Bool
}

func NewAltFinder(store *repository.Store, prov provider.Provider, log *slog.Logger) *AltFinder {
	return &AltFinder{store: store, prov: prov, log: log}
}

// appCorpusFilter matches the same set as the /apps directory: apps/CLIs or
// anything that ships platform builds, never a library.
func appCorpusFilter() bson.M {
	return bson.M{
		"type": bson.M{"$ne": "library"},
		"$or": bson.A{
			bson.M{"type": bson.M{"$in": bson.A{"app", "cli"}}},
			bson.M{"sourceData.platforms.0": bson.M{"$exists": true}},
		},
	}
}

// Run infers alternatives for up to maxAltPerRun unprocessed apps. Re-entrant
// runs are rejected. Batches that fail the LLM call are left unprocessed so a
// later run retries them, rather than being recorded as "no alternatives".
func (f *AltFinder) Run(ctx context.Context) {
	if !f.running.CompareAndSwap(false, true) {
		f.log.Warn("altfinder: run already in progress, skipping")
		return
	}
	defer f.running.Store(false)

	filter := appCorpusFilter()
	filter["altStatus"] = bson.M{"$exists": false}

	cur, err := f.store.Items().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).SetLimit(maxAltPerRun))
	if err != nil {
		f.log.Error("altfinder: query failed", "err", err)
		return
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		f.log.Error("altfinder: decode failed", "err", err)
		return
	}
	if len(items) == 0 {
		f.log.Info("altfinder: no unprocessed apps")
		return
	}

	done, withAlts := 0, 0
	for start := 0; start < len(items); start += altBatchSize {
		if ctx.Err() != nil {
			f.log.Warn("altfinder: cancelled", "done", done)
			return
		}
		end := start + altBatchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]

		results, err := service.InferAlternatives(ctx, f.prov, batch)
		if err != nil {
			// Whole-call failure (LLM down/timeout/unparseable) — leave the batch
			// unprocessed for a later run; never record it as "no alternatives".
			f.log.Warn("altfinder: batch call failed, leaving pending", "err", err, "size", len(batch))
			continue
		}
		for _, it := range batch {
			alts := results[it.ExternalID]
			set := bson.M{"altStatus": "done"}
			if len(alts) > 0 {
				set["alternativeTo"] = alts
				withAlts++
			}
			if _, err := f.store.Items().UpdateByID(ctx, it.ID, bson.M{"$set": set}); err != nil {
				f.log.Error("altfinder: update failed", "item", it.ExternalID, "err", err)
				continue
			}
			done++
		}
	}
	f.log.Info("altfinder done", "processed", done, "withAlternatives", withAlts)
}
