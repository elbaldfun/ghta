package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

// A repository's page is three reads: the small detail, the README and the
// star curve, so a page that shows one need not carry the others.
func (a *API) registerDetail(r gin.IRouter) {
	r.GET("/repos/:owner/:name", a.repo)
	r.GET("/repos/:owner/:name/readme", a.readme)
	r.GET("/repos/:owner/:name/history", a.history)
	r.GET("/trend", a.trend)
}

// maxAgeDetail: a repo changes once a night, and its page is the most
// crawled thing on the site.
const maxAgeDetail = 600

// Backfill returns a repo's long star curve from outside the snapshots (GH
// Archive), fetching and keeping it on first use, or nil when no source has
// it.
type Backfill func(ctx context.Context, externalID string) []Point

// WithBackfill sets the long-curve source the history read merges in.
func (a *API) WithBackfill(b Backfill) *API {
	a.backfill = b
	return a
}

// Point is one value of a curve, at a Unix time in milliseconds.
type Point struct {
	T int64 `json:"t"`
	V int   `json:"v"`
}

// Rank is a position in one ranking: overall, within the language, or
// within a domain leaf.
type Rank struct {
	Scope string `json:"scope"`
	Key   string `json:"key,omitempty"`
	Rank  int    `json:"rank"`
	// Change is the places gained since the last ranking on an earlier day,
	// negative for places lost; absent when there is nothing to compare.
	Change *int `json:"change,omitempty"`
}

type RepoDetail struct {
	RepoCard
	Ranks          []Rank              `json:"ranks"`
	ScreenshotURL  string              `json:"screenshotUrl,omitempty"`
	ShotSource     string              `json:"shotSource,omitempty"`
	Platforms      []string            `json:"platforms"`
	PlatformSource string              `json:"platformSource,omitempty"`
	ReleaseAssets  []item.ReleaseAsset `json:"releaseAssets"`
	LatestRelease  *LatestRelease      `json:"latestRelease"`
	Shelf          string              `json:"shelf,omitempty"`
	HasGui         *bool               `json:"hasGui,omitempty"`
	Alternatives   []item.Alternative  `json:"alternatives"`
	Tags           []string            `json:"tags"`
	FetchedAt      time.Time           `json:"fetchedAt"`
}

type LatestRelease struct {
	Tag         string    `json:"tag"`
	PublishedAt time.Time `json:"publishedAt,omitzero"`
}

// repoID is the external id a repo route names.
func repoID(c *gin.Context) string { return c.Param("owner") + "/" + c.Param("name") }

// movedTo is the name a stale repository goes by now, when reconcile found it
// renamed onto one already tracked; "" for any other repository.
func movedTo(it *item.Item) string {
	if !it.Stale {
		return ""
	}
	if to, ok := strings.CutPrefix(it.StaleReason, "renamed:"); ok && strings.Contains(to, "/") {
		return to
	}
	return ""
}

// GET /v1/repos/{owner}/{name} — a renamed repository answers with a
// permanent redirect to its new name; one gone from GitHub with 404.
func (a *API) repo(c *gin.Context) {
	it, err := a.st.Item(c.Request.Context(), item.SourceGitHub, repoID(c))
	if err != nil {
		a.fail(c, notFound(err))
		return
	}
	if to := movedTo(&it); to != "" {
		c.Redirect(http.StatusMovedPermanently, "/v1/repos/"+to)
		return
	}
	if it.Stale {
		a.fail(c, errNotFound)
		return
	}
	d := RepoDetail{
		RepoCard: card(&it), Ranks: []Rank{}, Platforms: []string{}, ReleaseAssets: []item.ReleaseAsset{},
		ScreenshotURL: it.Assets.ScreenshotURL, ShotSource: it.Assets.ShotSource,
		HasGui: it.Labels.HasGui, Alternatives: it.Labels.Alternatives, Tags: it.Labels.Tags, FetchedAt: it.FetchedAt,
	}
	d.Description = strings.TrimSpace(it.Description)
	if it.Labels.Shelf != item.ShelfExcluded {
		d.Shelf = it.Labels.Shelf
	}
	if r := it.Ranks; r != nil {
		prev := r.Prev
		if prev == nil {
			prev = &item.RankPositions{}
		}
		d.Ranks = append(d.Ranks, Rank{Scope: "overall", Rank: r.Overall, Change: rankChange(prev.Overall, r.Overall)})
		if r.Language > 0 {
			d.Ranks = append(d.Ranks, Rank{Scope: "language", Key: it.Language, Rank: r.Language, Change: rankChange(prev.Language, r.Language)})
		}
		// In the item's own order of domains: the first is its main one.
		for _, p := range it.Labels.Domain {
			if n := r.Domain[p]; n > 0 {
				d.Ranks = append(d.Ranks, Rank{Scope: "domain", Key: p, Rank: n, Change: rankChange(prev.Domain[p], n)})
			}
		}
	}
	if g := it.Gh; g != nil {
		if g.Platforms != nil {
			d.Platforms = g.Platforms
		}
		d.PlatformSource = g.PlatformSource
		if g.ReleaseAssets != nil {
			d.ReleaseAssets = g.ReleaseAssets
		}
		if g.LatestRelease != nil {
			d.LatestRelease = &LatestRelease{Tag: g.LatestRelease.Tag, PublishedAt: g.LatestRelease.PublishedAt}
		}
	}
	if d.Alternatives == nil {
		d.Alternatives = []item.Alternative{}
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	a.cached(c, maxAgeDetail, gin.H{"data": d})
}

// rankChange is the places gained from was to now; nil when there was no
// earlier position to compare with.
func rankChange(was, now int) *int {
	if was == 0 {
		return nil
	}
	d := was - now
	return &d
}

func notFound(err error) error {
	if errors.Is(err, store.ErrNoItem) {
		return errNotFound
	}
	return err
}

// GET /v1/repos/{owner}/{name}/readme — the README as stored, in Markdown.
// Rendering and sanitising stay with the page, which owns the markup.
func (a *API) readme(c *gin.Context) {
	id := repoID(c)
	ctx := c.Request.Context()
	if _, err := a.st.Item(ctx, item.SourceGitHub, id); err != nil {
		a.fail(c, notFound(err))
		return
	}
	md, err := a.st.Readme(ctx, item.SourceGitHub, id)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeDetail, gin.H{"data": gin.H{"markdown": md}})
}

// GET /v1/repos/{owner}/{name}/history — the star curve: every daily
// snapshot, preceded by the GH Archive backfill where there is one.
func (a *API) history(c *gin.Context) {
	id := repoID(c)
	ctx := c.Request.Context()
	it, err := a.st.Item(ctx, item.SourceGitHub, id)
	if err != nil {
		a.fail(c, notFound(err))
		return
	}
	readings, err := a.st.MetricSeries(ctx, item.SourceGitHub, id, "stars", time.Time{})
	if err != nil {
		a.fail(c, err)
		return
	}
	var back []Point
	if a.backfill != nil {
		back = a.backfill(ctx, id)
	}
	a.cached(c, maxAgeDetail, gin.H{"data": mergeCurve(back, readings, it.Metrics["stars"])})
}

// mergeCurve puts the backfill in front of the snapshots. GH Archive counts
// stars as events and drifts from GitHub's own count, so the backfill is
// scaled to meet the first snapshot (or the current count, with none) and
// only its points before that are kept.
func mergeCurve(back []Point, snaps []store.Reading, current float64) []Point {
	out := []Point{}
	anchor, cutoff := current, time.Now().UnixMilli()
	if len(snaps) > 0 {
		anchor, cutoff = snaps[0].V, snaps[0].At.UnixMilli()
	}
	var kept []Point
	for _, p := range back {
		if p.T < cutoff {
			kept = append(kept, p)
		}
	}
	if n := len(kept); n > 0 {
		factor := 1.0
		if last := kept[n-1].V; last > 0 && anchor > 0 {
			factor = anchor / float64(last)
		}
		for _, p := range kept {
			out = append(out, Point{T: p.T, V: int(math.Round(float64(p.V) * factor))})
		}
	}
	for _, s := range snaps {
		out = append(out, Point{T: s.At.UnixMilli(), V: int(s.V)})
	}
	return out
}

// Trend limits: a board page's sparklines at once, over up to the history
// ring's length.
const (
	maxTrendIDs  = 60
	maxTrendDays = 35
)

// GET /v1/trend?ids=a/b,c/d&days=30 — the daily star curve of each repo,
// from its history ring. A repo with no points in the window is absent.
func (a *API) trend(c *gin.Context) {
	var ids []string
	for _, id := range strings.Split(c.Query("ids"), ",") {
		if id = strings.TrimSpace(id); id != "" && len(ids) < maxTrendIDs {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		a.fail(c, errBadInput("ids is required"))
		return
	}
	days, err := intParam(c, "days", 30, 1, maxTrendDays)
	if err != nil {
		a.fail(c, err)
		return
	}
	rings, err := a.st.Histories(c.Request.Context(), item.SourceGitHub, ids)
	if err != nil {
		a.fail(c, err)
		return
	}
	from := item.DayOf(time.Now().AddDate(0, 0, -days))
	out := map[string][]Point{}
	for id, ring := range rings {
		i := sort.Search(len(ring), func(i int) bool { return ring[i].D >= from })
		for _, p := range ring[i:] {
			t, err := time.Parse("2006-01-02", p.D)
			if err != nil {
				continue
			}
			out[id] = append(out[id], Point{T: t.UnixMilli(), V: int(p.V)})
		}
	}
	a.cached(c, maxAgeBoard, gin.H{"data": out})
}
