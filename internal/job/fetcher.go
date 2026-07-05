// Package job holds the scheduled background jobs (fetch, categorize).
package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/source"
)

// Fetcher drives every registered source adapter, shard by shard, persisting
// results and tracking per-shard progress so a run can resume after a crash.
type Fetcher struct {
	store    *repository.Store
	registry *source.Registry
	log      *slog.Logger
}

func NewFetcher(store *repository.Store, registry *source.Registry, log *slog.Logger) *Fetcher {
	return &Fetcher{store: store, registry: registry, log: log}
}

// Run executes one full fetch pass. Completed shards for today are skipped;
// failed shards are retried. A single shard's failure never aborts the run.
func (f *Fetcher) Run(ctx context.Context) {
	date := time.Now().UTC().Format("2006-01-02")
	for _, adapter := range f.registry.All() {
		src := adapter.Source()
		shards := adapter.Shards()
		f.log.Info("fetch source starting", "source", src, "shards", len(shards))

		for _, shard := range shards {
			if ctx.Err() != nil {
				f.log.Warn("fetch cancelled", "source", src)
				return
			}
			status, err := f.store.ShardStatus(ctx, src, date, shard)
			if err != nil {
				f.log.Error("shard status lookup failed", "source", src, "shard", shard, "err", err)
				continue
			}
			if status == domain.FetchDone {
				continue // already done today
			}
			f.runShard(ctx, adapter, date, shard)
		}
		f.log.Info("fetch source done", "source", src)
	}
}

// RunShard fetches a single shard for one source (used for manual/test runs).
func (f *Fetcher) RunShard(ctx context.Context, src domain.Source, shard string) error {
	adapter, ok := f.registry.Get(src)
	if !ok {
		return fmt.Errorf("no adapter registered for source %q", src)
	}
	date := time.Now().UTC().Format("2006-01-02")
	f.runShard(ctx, adapter, date, shard)
	return nil
}

func (f *Fetcher) runShard(ctx context.Context, adapter source.Fetcher, date, shard string) {
	src := adapter.Source()
	_ = f.store.SetShardStatus(ctx, src, date, shard, domain.FetchRunning, "")

	items, err := adapter.Fetch(ctx, shard)
	if err != nil {
		f.log.Error("shard fetch failed", "source", src, "shard", shard, "err", err)
		_ = f.store.SetShardStatus(ctx, src, date, shard, domain.FetchFailed, err.Error())
		return
	}

	upserted, err := f.store.UpsertItems(ctx, items)
	if err != nil {
		f.log.Error("shard upsert failed", "source", src, "shard", shard, "err", err)
		_ = f.store.SetShardStatus(ctx, src, date, shard, domain.FetchFailed, err.Error())
		return
	}
	snaps, err := f.store.AppendSnapshots(ctx, items)
	if err != nil {
		f.log.Error("shard snapshot failed", "source", src, "shard", shard, "err", err)
		// snapshots are best-effort; the shard's item data is already persisted
	}

	f.log.Info("shard done", "source", src, "shard", shard, "items", len(items), "upserted", upserted, "snapshots", snaps)
	_ = f.store.SetShardStatus(ctx, src, date, shard, domain.FetchDone, "")
}
