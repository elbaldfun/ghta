package pipeline

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/elbaldfun/ghta/internal/icon"
	"github.com/elbaldfun/ghta/internal/shot"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

const (
	iconMaxPerRun = 800
	iconPace      = 120 * time.Millisecond // gap between homepage fetches
	shotMaxPerRun = 1500
	shotPace      = 150 * time.Millisecond
)

// Icon extracts each app's own icon from its homepage, so the directory shows
// the app's logo instead of the repository owner's avatar. Every app is
// processed once, most popular first; one that yields no icon is still marked
// done.
type Icon struct {
	st      *store.Store
	log     *slog.Logger
	extract func(ctx context.Context, homepage string) string
	pace    time.Duration
	now     func() time.Time
}

func NewIcon(st *store.Store, log *slog.Logger) *Icon {
	if log == nil {
		log = slog.Default()
	}
	hc := &http.Client{Timeout: 8 * time.Second} // bounds a slow or hostile site
	return &Icon{
		st: st, log: log, pace: iconPace, now: time.Now,
		extract: func(ctx context.Context, homepage string) string { return icon.Extract(ctx, hc, homepage) },
	}
}

func (*Icon) Name() string { return "icon" }

func (j *Icon) Run(ctx context.Context) (rep worker.Report, err error) {
	queue, err := j.st.AssetQueue(ctx, "icon", true, iconMaxPerRun)
	if err != nil {
		return rep, err
	}
	found := 0
	defer func() { rep.Note("found", found) }()
	for _, c := range queue {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		url := j.extract(ctx, c.Homepage)
		if ctx.Err() != nil {
			return rep, ctx.Err() // a cut-off fetch is not a verdict
		}
		if err := j.st.SetIcon(ctx, c.ID, url, j.now()); err != nil {
			rep.Failed++
			j.log.Warn("icon: not recorded", "item", c.ExternalID, "err", err)
			continue
		}
		rep.Processed++
		if url != "" {
			found++
		}
		sleepCtx(ctx, j.pace)
	}
	return rep, nil
}

// Shot finds each app's screenshot: the first real UI image in its README,
// else the homepage's og:image. The source is recorded because only README
// images are good enough for the screenshot wall; og images, often marketing
// cards, are a detail-page fallback.
type Shot struct {
	st   *store.Store
	log  *slog.Logger
	find func(ctx context.Context, externalID, readme, homepage string) (url, source string)
	pace time.Duration
	now  func() time.Time
}

func NewShot(st *store.Store, log *slog.Logger) *Shot {
	if log == nil {
		log = slog.Default()
	}
	hc := &http.Client{Timeout: 10 * time.Second}
	return &Shot{
		st: st, log: log, pace: shotPace, now: time.Now,
		find: func(ctx context.Context, externalID, readme, homepage string) (string, string) {
			for _, cand := range shot.Candidates(readme, externalID) {
				if shot.Verify(ctx, hc, cand) {
					return cand, "readme"
				}
			}
			if homepage != "" {
				if og := shot.OGImage(ctx, hc, homepage); og != "" && shot.Verify(ctx, hc, og) {
					return og, "og"
				}
			}
			return "", ""
		},
	}
}

func (*Shot) Name() string { return "shot" }

func (j *Shot) Run(ctx context.Context) (rep worker.Report, err error) {
	queue, err := j.st.AssetQueue(ctx, "shot", false, shotMaxPerRun)
	if err != nil {
		return rep, err
	}
	found, viaOG := 0, 0
	defer func() { rep.Note("found", found); rep.Note("viaOg", viaOG) }()
	for _, c := range queue {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		readme, err := j.st.Readme(ctx, c.Source, c.ExternalID)
		if err != nil {
			rep.Failed++
			j.log.Warn("shot: readme not read", "item", c.ExternalID, "err", err)
			continue
		}
		url, source := j.find(ctx, c.ExternalID, readme, c.Homepage)
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		if err := j.st.SetShot(ctx, c.ID, url, source, j.now()); err != nil {
			rep.Failed++
			j.log.Warn("shot: not recorded", "item", c.ExternalID, "err", err)
			continue
		}
		rep.Processed++
		if url != "" {
			found++
			if source == "og" {
				viaOG++
			}
		}
		sleepCtx(ctx, j.pace)
	}
	return rep, nil
}
