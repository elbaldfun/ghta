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
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/shot"
)

const (
	maxShotPerRun = 1500
	shotPace      = 150 * time.Millisecond
)

// ShotFetcher finds each app's screenshot (change 15 v2b): the first real UI
// image in its README, else the homepage og:image. Data-gathering only — the UI
// decision waits for the coverage numbers this job logs (review condition).
type ShotFetcher struct {
	store   *repository.Store
	hc      *http.Client
	log     *slog.Logger
	running atomic.Bool
}

func NewShotFetcher(store *repository.Store, log *slog.Logger) *ShotFetcher {
	return &ShotFetcher{
		store: store,
		hc:    &http.Client{Timeout: 10 * time.Second},
		log:   log,
	}
}

// Run processes up to maxShotPerRun unprocessed apps, most-popular-first.
// Resumable (skips shotStatus set), run-guarded, paced.
func (f *ShotFetcher) Run(ctx context.Context) {
	if !f.running.CompareAndSwap(false, true) {
		f.log.Warn("shotfetcher: run already in progress, skipping")
		return
	}
	defer f.running.Store(false)

	filter := repository.AppCorpusFilter()
	filter["shotStatus"] = bson.M{"$exists": false}

	cur, err := f.store.Items().Find(ctx, filter,
		options.Find().
			SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).
			SetLimit(maxShotPerRun).
			SetProjection(bson.M{"externalId": 1, "sourceData.readme": 1, "sourceData.homepageUrl": 1}))
	if err != nil {
		f.log.Error("shotfetcher: query failed", "err", err)
		return
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		f.log.Error("shotfetcher: decode failed", "err", err)
		return
	}
	if len(items) == 0 {
		f.log.Info("shotfetcher: nothing pending")
		return
	}

	done, found, viaOG := 0, 0, 0
	for _, it := range items {
		if ctx.Err() != nil {
			f.log.Warn("shotfetcher: cancelled", "done", done)
			return
		}
		url := f.find(ctx, it)
		set := bson.M{"shotStatus": "done"}
		if url != "" {
			set["screenshotUrl"] = url
			found++
			if it.SourceData != nil {
				if hp, _ := it.SourceData["homepageUrl"].(string); hp != "" && !isRawGithub(url) {
					viaOG++
				}
			}
		}
		if _, err := f.store.Items().UpdateByID(ctx, it.ID, bson.M{"$set": set}); err != nil {
			f.log.Error("shotfetcher: update failed", "item", it.ExternalID, "err", err)
			continue
		}
		done++
		select {
		case <-ctx.Done():
			return
		case <-time.After(shotPace):
		}
	}
	// The coverage number the UI decision waits on.
	f.log.Info("shotfetcher: pass done", "processed", done, "withScreenshot", found,
		"coveragePct", pct(found, done), "ogFallbacks", viaOG)
}

// find tries README candidates in order (dimension-verified), then og:image.
func (f *ShotFetcher) find(ctx context.Context, it domain.TrackedItem) string {
	sd := it.SourceData
	if sd == nil {
		return ""
	}
	readme, _ := sd["readme"].(string)
	for _, cand := range shot.Candidates(readme, it.ExternalID) {
		if shot.Verify(ctx, f.hc, cand) {
			return cand
		}
	}
	if hp, _ := sd["homepageUrl"].(string); hp != "" {
		if og := shot.OGImage(ctx, f.hc, hp); og != "" && shot.Verify(ctx, f.hc, og) {
			return og
		}
	}
	return ""
}

func isRawGithub(u string) bool {
	return len(u) > 34 && u[:34] == "https://raw.githubusercontent.com/"
}

func pct(n, of int) int {
	if of == 0 {
		return 0
	}
	return n * 100 / of
}
