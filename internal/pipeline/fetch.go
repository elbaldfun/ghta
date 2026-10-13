package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

const fetchWriteBatch = 500

// Fetch pulls every source shard by shard and stores the result with
// store.WriteFetched, which also advances each item's history ring and
// growth — there is no separate metrics pass.
//
// Progress is kept per shard and UTC day, so a run that is interrupted or
// repeated the same day picks up the shards that are not done yet.
type Fetch struct {
	st      *store.Store
	sources []source.ItemFetcher
	log     *slog.Logger
	now     func() time.Time
}

func NewFetch(st *store.Store, log *slog.Logger, sources ...source.ItemFetcher) *Fetch {
	if log == nil {
		log = slog.Default()
	}
	return &Fetch{st: st, sources: sources, log: log, now: time.Now}
}

func (*Fetch) Name() string { return "fetch" }

// sourceTally is the per-source part of the run report.
type sourceTally struct {
	Shards           int `bson:"shards" json:"shards"`
	Skipped          int `bson:"skipped" json:"skipped"` // already done today
	Failed           int `bson:"failed" json:"failed"`
	store.FetchStats `bson:",inline"`
}

func (j *Fetch) Run(ctx context.Context) (worker.Report, error) {
	var rep worker.Report
	date := item.DayOf(j.now())
	attempted, lastErr := 0, error(nil)

	for _, src := range j.sources {
		name := string(src.Source())
		shards := src.Shards()
		tally := sourceTally{Shards: len(shards)}
		j.log.Info("fetch: source starting", "source", name, "shards", len(shards))

		for _, shard := range shards {
			if err := ctx.Err(); err != nil {
				rep.Note(name, tally)
				return rep, err
			}
			status, err := j.st.ShardStatus(ctx, name, date, shard)
			if err != nil {
				rep.Note(name, tally)
				return rep, err
			}
			if status == store.ShardDone {
				tally.Skipped++
				continue
			}
			attempted++
			stats, err := j.runShard(ctx, src, date, shard)
			tally.Add(stats)
			if err != nil {
				if ctx.Err() != nil {
					rep.Note(name, tally)
					return rep, ctx.Err()
				}
				tally.Failed++
				rep.Failed++
				lastErr = err
				j.log.Error("fetch: shard failed", "source", name, "shard", shard, "err", err)
				continue
			}
			rep.Processed += stats.Items
		}
		rep.Note(name, tally)
		j.log.Info("fetch: source done", "source", name, "items", tally.Items, "new", tally.New,
			"snapshots", tally.Snapshots, "blobs", tally.Blobs, "skipped", tally.Skipped, "failed", tally.Failed)
	}

	// Scattered shard failures are retried by the next run. Every shard
	// failing is a broken token or an unreachable source: fail the run.
	if attempted > 0 && rep.Failed == attempted {
		return rep, fmt.Errorf("all %d shards failed; last: %w", attempted, lastErr)
	}
	return rep, nil
}

func (j *Fetch) runShard(ctx context.Context, src source.ItemFetcher, date, shard string) (store.FetchStats, error) {
	name := string(src.Source())
	// Bookkeeping survives a shutdown, so a cut-off shard reads as failed, not
	// as running forever.
	book := context.WithoutCancel(ctx)
	mark := func(status string, err error) {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if e := j.st.SetShardStatus(book, name, date, shard, status, msg, j.now()); e != nil {
			j.log.Warn("fetch: shard status not recorded", "source", name, "shard", shard, "err", e)
		}
	}

	mark(store.ShardRunning, nil)
	fetched, err := src.FetchItems(ctx, shard)
	if err != nil {
		mark(store.ShardFailed, err)
		return store.FetchStats{}, err
	}
	// A HuggingFace board is 20,000 models in one shard: write in chunks.
	var stats store.FetchStats
	for start := 0; start < len(fetched); start += fetchWriteBatch {
		part, err := j.st.WriteFetched(ctx, fetched[start:min(start+fetchWriteBatch, len(fetched))], j.now())
		stats.Add(part)
		if err != nil {
			mark(store.ShardFailed, err)
			return stats, err
		}
	}
	mark(store.ShardDone, nil)
	j.log.Debug("fetch: shard done", "source", name, "shard", shard, "items", stats.Items,
		"new", stats.New, "snapshots", stats.Snapshots, "blobs", stats.Blobs)
	return stats, nil
}
