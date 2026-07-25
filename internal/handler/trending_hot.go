package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/source/github"
)

// ingestTimeout bounds the background fetch+upsert of trending newcomers so a
// slow GitHub call can't leak a goroutine.
const ingestTimeout = 90 * time.Second

// hotReposTTL bounds how often we scrape github.com/trending. One scrape per
// (window, language) serves every visitor from cache, so traffic never hits
// GitHub per-request.
const hotReposTTL = time.Hour

// HotReposHandler serves "this week's breakout repos" — GitHub's own trending
// ranking (stars gained in the window), which has no API, scraped and cached.
// Repos we already track are enriched with our domain classification; brand-new
// low-star repos GitHub surfaces are served as-is (they are not in our corpus,
// which only ingests >=1000-star repos).
type HotReposHandler struct {
	store *repository.Store
	gh    *github.Adapter // full-fetches trending newcomers so they enter the corpus
	hc    *http.Client

	mu    sync.Mutex
	cache map[string]cachedHot // keyed by "since|lang"

	ingesting sync.Map // externalId -> struct{}, dedupes concurrent ingest attempts
}

type cachedHot struct {
	items     []HotRepo
	fetchedAt time.Time
}

// HotRepo is a trending row plus whatever our corpus knows about it.
type HotRepo struct {
	github.TrendingRepo
	CategoryPath domain.PathList `json:"categoryPath,omitempty"`
	Type         string          `json:"type,omitempty"`
	InCorpus     bool            `json:"inCorpus"`
}

func NewHotReposHandler(store *repository.Store, gh *github.Adapter) *HotReposHandler {
	return &HotReposHandler{
		store: store,
		gh:    gh,
		hc:    github.NewTrendingClient(),
		cache: map[string]cachedHot{},
	}
}

func (h *HotReposHandler) Register(r gin.IRoutes) {
	r.GET("/trending/hot", h.List)
}

// List handles GET /trending/hot?since=weekly&lang=&limit=N.
func (h *HotReposHandler) List(c *gin.Context) {
	since := c.DefaultQuery("since", "weekly")
	switch since {
	case "daily", "weekly", "monthly":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "since must be daily|weekly|monthly"})
		return
	}

	lang := c.Query("lang") // GitHub programming-language slug, e.g. "go"; "" = all

	limit := 25
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be 1..50"})
			return
		}
		limit = n
	}

	items, err := h.get(c.Request.Context(), since, lang)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if len(items) > limit {
		items = items[:limit]
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// get returns the cached hot list for (since, lang), refreshing past the TTL. A
// stale cache is served if a refresh fails, so a GitHub hiccup or markup change
// never empties the section.
func (h *HotReposHandler) get(ctx context.Context, since, lang string) ([]HotRepo, error) {
	key := since + "|" + lang

	h.mu.Lock()
	defer h.mu.Unlock()

	if c, ok := h.cache[key]; ok && time.Since(c.fetchedAt) < hotReposTTL {
		return c.items, nil
	}

	scraped, err := github.FetchTrending(ctx, h.hc, since, lang)
	if err != nil {
		if c, ok := h.cache[key]; ok {
			return c.items, nil // serve stale rather than fail
		}
		return nil, err
	}

	items := h.enrich(ctx, scraped)
	h.cache[key] = cachedHot{items: items, fetchedAt: time.Now()}

	// Ingest trending repos we don't track yet so they get classified,
	// snapshotted, and a working detail page. Background + best-effort: the
	// response is served from the scrape regardless of ingest outcome.
	var newcomers []string
	for _, it := range items {
		if !it.InCorpus {
			newcomers = append(newcomers, it.ExternalID)
		}
	}
	go h.ingestNewcomers(newcomers)

	return items, nil
}

// ingestNewcomers full-fetches trending repos not yet in our corpus and upserts
// them. UpsertItems marks freshly-inserted items analysisStatus=pending, so the
// categorizer classifies them on its next pass; AppendSnapshots starts their
// star history. Best-effort and deduped so overlapping hourly refreshes don't
// double-fetch the same repo from GitHub.
func (h *HotReposHandler) ingestNewcomers(externalIDs []string) {
	if h.gh == nil || len(externalIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), ingestTimeout)
	defer cancel()

	var items []domain.TrackedItem
	for _, id := range externalIDs {
		if _, busy := h.ingesting.LoadOrStore(id, struct{}{}); busy {
			continue // another goroutine is already fetching this repo
		}
		found, err := h.gh.Search(ctx, "repo:"+id, 1)
		h.ingesting.Delete(id)
		if err != nil || len(found) == 0 {
			continue
		}
		items = append(items, found[0])
	}
	if len(items) == 0 {
		return
	}
	if _, err := h.store.UpsertItems(ctx, items); err != nil {
		slog.Warn("trending ingest: upsert failed", "count", len(items), "err", err)
		return
	}
	if _, err := h.store.AppendSnapshots(ctx, items); err != nil {
		slog.Warn("trending ingest: snapshot failed", "err", err)
	}
	slog.Info("trending newcomers ingested", "count", len(items))
}

// enrich joins scraped repos to our corpus, attaching domain classification for
// the ones we already track. A DB failure degrades to un-enriched results
// rather than dropping the section.
func (h *HotReposHandler) enrich(ctx context.Context, scraped []github.TrendingRepo) []HotRepo {
	ids := make([]string, len(scraped))
	for i, r := range scraped {
		ids[i] = r.ExternalID
	}

	type known struct {
		ExternalID   string          `bson:"externalId"`
		CategoryPath domain.PathList `bson:"categoryPath"`
		Type         string          `bson:"type"`
	}
	byID := map[string]known{}

	cur, err := h.store.Items().Find(ctx,
		bson.M{"source": domain.SourceGitHub, "externalId": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"externalId": 1, "categoryPath": 1, "type": 1}),
	)
	if err == nil {
		var rows []known
		if err := cur.All(ctx, &rows); err == nil {
			for _, r := range rows {
				byID[r.ExternalID] = r
			}
		}
	}

	out := make([]HotRepo, 0, len(scraped))
	for _, r := range scraped {
		hr := HotRepo{TrendingRepo: r}
		if k, ok := byID[r.ExternalID]; ok {
			hr.InCorpus = true
			hr.CategoryPath = k.CategoryPath
			hr.Type = k.Type
		}
		out = append(out, hr)
	}
	return out
}
