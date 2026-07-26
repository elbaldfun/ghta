package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/source/github"
)

// Minimum stars for a new repo to count as "blowing up" — filters out the noise
// of brand-new repos with a handful of stars.
const newReposStarFloor = 100

// newReposTTL bounds how often we hit GitHub's search API (30 req/min), which a
// per-visit live call would blow through instantly. One query per window serves
// every visitor from cache.
const newReposTTL = time.Hour

// NewReposHandler serves "repos created recently, already popular" — pulled live
// from GitHub search (not our own DB, since these are too new to be tracked),
// then cached so traffic never reaches GitHub directly.
type NewReposHandler struct {
	gh *github.Adapter

	mu    sync.Mutex
	cache map[int]cachedRepos // keyed by lookback days
	sf    singleflight.Group  // collapses concurrent misses for one window into one search
}

type cachedRepos struct {
	items     []domain.TrackedItem
	fetchedAt time.Time
}

func NewNewReposHandler(gh *github.Adapter) *NewReposHandler {
	return &NewReposHandler{gh: gh, cache: map[int]cachedRepos{}}
}

func (h *NewReposHandler) Register(r gin.IRoutes) {
	r.GET("/trending/new", h.List)
}

// List handles GET /trending/new?days=7&limit=N.
func (h *NewReposHandler) List(c *gin.Context) {
	days := 7
	if v := c.Query("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 90 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be 1..90"})
			return
		}
		days = n
	}
	limit := 12
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be 1..50"})
			return
		}
		limit = n
	}

	items, err := h.get(c.Request.Context(), days)
	if err != nil {
		respondErr(c, err)
		return
	}
	if len(items) > limit {
		items = items[:limit]
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// get returns the cached list for a lookback window, refreshing past the TTL.
// A stale cache is served if a refresh fails, so a GitHub hiccup never empties
// the section.
func (h *NewReposHandler) get(ctx context.Context, days int) ([]domain.TrackedItem, error) {
	// Fast path: fresh cache, lock held only for the map read.
	h.mu.Lock()
	if c, ok := h.cache[days]; ok && time.Since(c.fetchedAt) < newReposTTL {
		h.mu.Unlock()
		return c.items, nil
	}
	h.mu.Unlock()

	// Miss: hit GitHub OUTSIDE the lock, collapsing concurrent misses for the
	// same window into one search call. Holding the mutex across the search
	// would serialize every caller behind one upstream round-trip.
	v, err, _ := h.sf.Do(strconv.Itoa(days), func() (any, error) {
		since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
		query := fmt.Sprintf("created:>%s stars:>%d sort:stars-desc", since, newReposStarFloor)
		items, err := h.gh.Search(ctx, query, 50)
		if err != nil {
			h.mu.Lock()
			c, ok := h.cache[days]
			h.mu.Unlock()
			if ok {
				return c.items, nil // serve stale rather than fail
			}
			return nil, err
		}
		h.mu.Lock()
		h.cache[days] = cachedRepos{items: items, fetchedAt: time.Now()}
		h.mu.Unlock()
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]domain.TrackedItem), nil
}
