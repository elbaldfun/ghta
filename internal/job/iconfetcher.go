package job

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/icon"
	"github.com/elbaldfun/ghta/internal/repository"
)

const (
	maxIconPerRun = 800
	iconPace      = 120 * time.Millisecond // gap between homepage fetches
)

// IconFetcher extracts each app's brand icon from its homepage (change 13), so
// the directory shows the app's own logo instead of the repo owner's avatar. It
// only touches the app corpus that has a homepage, most-popular-first.
type IconFetcher struct {
	store   *repository.Store
	hc      *http.Client
	log     *slog.Logger
	running atomic.Bool
}

func NewIconFetcher(store *repository.Store, log *slog.Logger) *IconFetcher {
	return &IconFetcher{
		store: store,
		// No redirect cap issues: default client follows up to 10, fine for a
		// homepage. Timeout bounds a slow/hostile site.
		hc:  &http.Client{Timeout: 8 * time.Second},
		log: log,
	}
}

// Run extracts icons for up to maxIconPerRun unprocessed apps that have a
// homepage. Re-entrant runs are rejected; the job is resumable (skips
// iconStatus set) and paced so it never hammers any one host.
func (f *IconFetcher) Run(ctx context.Context) {
	if !f.running.CompareAndSwap(false, true) {
		f.log.Warn("iconfetcher: run already in progress, skipping")
		return
	}
	defer f.running.Store(false)

	filter := repository.AppCorpusFilter()
	filter["iconStatus"] = bson.M{"$exists": false}
	filter["sourceData.homepageUrl"] = bson.M{"$nin": bson.A{"", nil}}

	cur, err := f.store.Items().Find(ctx, filter,
		options.Find().
			SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).
			SetLimit(maxIconPerRun).
			SetProjection(bson.M{"externalId": 1, "sourceData.homepageUrl": 1}))
	if err != nil {
		f.log.Error("iconfetcher: query failed", "err", err)
		return
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		f.log.Error("iconfetcher: decode failed", "err", err)
		return
	}
	if len(items) == 0 {
		f.log.Info("iconfetcher: no unprocessed apps with a homepage")
		return
	}

	done, found := 0, 0
	for _, it := range items {
		if ctx.Err() != nil {
			f.log.Warn("iconfetcher: cancelled", "done", done)
			return
		}
		homepage, _ := it.SourceData["homepageUrl"].(string)
		iconURL := icon.Extract(ctx, f.hc, homepage)

		set := bson.M{"iconStatus": "done"}
		if iconURL != "" {
			set["iconUrl"] = iconURL
			found++
		}
		if _, err := f.store.Items().UpdateByID(ctx, it.ID, bson.M{"$set": set}); err != nil {
			f.log.Error("iconfetcher: update failed", "item", it.ExternalID, "err", err)
			continue
		}
		done++
		select {
		case <-ctx.Done():
			return
		case <-time.After(iconPace):
		}
	}
	f.log.Info("iconfetcher done", "processed", done, "withIcon", found)
}
