package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
)

// The aggregated rankings, read whole from the boards the nightly job writes.
// domain is "all" or a top-level domain; an unknown one is a 404, as there
// is no such board.
func (a *API) registerBoards(r gin.IRouter) {
	r.GET("/developers", a.developers)
	r.GET("/topics", a.topics)
	r.GET("/ecosystem", a.ecosystem)
	r.GET("/heat", a.heat)
	r.GET("/categories", a.categories)
	r.GET("/facets/type", a.typeFacets)
	r.GET("/stats/languages", a.languages)
	r.GET("/stats/staleness", a.staleness)
	r.GET("/models", a.models)
	r.GET("/hot", a.hot)
}

// GET /v1/developers?board=rising|merit&domain=all&limit=24&page=1
func (a *API) developers(c *gin.Context) {
	board, err := oneOf(c, "board", boards.DevRising, boards.DevMerit)
	if err != nil {
		a.fail(c, err)
		return
	}
	limit, err := intParam(c, "limit", 24, 1, 100)
	if err != nil {
		a.fail(c, err)
		return
	}
	pg, err := intParam(c, "page", 1, 1, boards.DevTopN)
	if err != nil {
		a.fail(c, err)
		return
	}
	domain := c.DefaultQuery("domain", "all")
	var rows []boards.Developer
	at, err := a.st.Board(c.Request.Context(), boards.DevelopersKey(board, domain), &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": page(rows, limit, pg), "total": len(rows), "page": pg,
		"board": board, "domain": domain, "computedAt": at})
}

// GET /v1/topics?sort=hot|popular&domain=all&limit=40
func (a *API) topics(c *gin.Context) {
	sort, err := oneOf(c, "sort", boards.TopicHot, boards.TopicPopular)
	if err != nil {
		a.fail(c, err)
		return
	}
	limit, err := intParam(c, "limit", 40, 1, boards.TopicTopN)
	if err != nil {
		a.fail(c, err)
		return
	}
	domain := c.DefaultQuery("domain", "all")
	var rows []boards.Topic
	at, err := a.st.Board(c.Request.Context(), boards.TopicsKey(sort, domain), &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": page(rows, limit, 1), "sort": sort, "domain": domain, "computedAt": at})
}

// GET /v1/ecosystem?pillar=all|skill|mcp|agent&sort=hot|popular&limit=30&page=1
func (a *API) ecosystem(c *gin.Context) {
	pillar, err := oneOf(c, "pillar", boards.Pillars...)
	if err != nil {
		a.fail(c, err)
		return
	}
	sort, err := oneOf(c, "sort", boards.EcoHot, boards.EcoPopular)
	if err != nil {
		a.fail(c, err)
		return
	}
	limit, err := intParam(c, "limit", 30, 1, 100)
	if err != nil {
		a.fail(c, err)
		return
	}
	pg, err := intParam(c, "page", 1, 1, boards.EcoTopN)
	if err != nil {
		a.fail(c, err)
		return
	}
	var rows []boards.EcoItem
	at, err := a.st.Board(c.Request.Context(), boards.EcosystemKey(pillar, sort), &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	if rows, err = a.currentEco(c.Request.Context(), rows, sort); err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": page(rows, limit, pg), "total": len(rows), "page": pg,
		"pillar": pillar, "sort": sort, "computedAt": at})
}

// currentEco puts the items' current stars, forks and day's gain on an
// ecosystem board and orders it by them again, so a repository shows the
// same growth.d1 here as in /v1/repos. The board is written once a night,
// after the fetch; the items change while the fetch runs (and on any rerun
// after the board), and between the two the board showed the day before's
// gain. Who is on the board stays as computed; one gone since is dropped.
func (a *API) currentEco(ctx context.Context, rows []boards.EcoItem, sort string) ([]boards.EcoItem, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ExternalID
	}
	now, err := a.st.CurrentCounts(ctx, item.SourceGitHub, ids)
	if err != nil {
		return nil, err
	}
	out := rows[:0]
	for _, r := range rows {
		n, ok := now[r.ExternalID]
		if !ok || n.Stale {
			continue
		}
		r.Stars, r.Forks, r.Growth = int(n.Metrics["stars"]), int(n.Metrics["forks"]), 0
		if n.Growth.D1 != nil {
			r.Growth = int(*n.Growth.D1)
		}
		out = append(out, r)
	}
	boards.SortEco(out, sort)
	return out, nil
}

// whole serves a board that is returned as it is stored.
func whole[T any](a *API, c *gin.Context, key string) {
	var data T
	at, err := a.st.Board(c.Request.Context(), key, &data)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, struct {
		Data       T         `json:"data"`
		ComputedAt time.Time `json:"computedAt"`
	}{data, at})
}

// GET /v1/heat — the domain heat map, hottest domain first.
func (a *API) heat(c *gin.Context) { whole[[]boards.HeatCell](a, c, boards.HeatKey) }

// GET /v1/categories — the domain tree with item counts.
func (a *API) categories(c *gin.Context) { whole[[]boards.CategoryNode](a, c, boards.CategoriesKey) }

// GET /v1/facets/type — the forms with item counts, in filter-chip order.
func (a *API) typeFacets(c *gin.Context) { whole[[]boards.Facet](a, c, boards.TypeFacetsKey) }

// GET /v1/stats/languages?limit=20
func (a *API) languages(c *gin.Context) {
	limit, err := intParam(c, "limit", 20, 1, boards.LanguageTopN)
	if err != nil {
		a.fail(c, err)
		return
	}
	var rows []boards.LanguageStat
	at, err := a.st.Board(c.Request.Context(), boards.LanguagesKey, &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": page(rows, limit, 1), "computedAt": at})
}

// GET /v1/stats/staleness?examples=10
func (a *API) staleness(c *gin.Context) {
	n, err := intParam(c, "examples", 0, 0, boards.StaleExamples)
	if err != nil {
		a.fail(c, err)
		return
	}
	var s boards.Staleness
	at, err := a.st.Board(c.Request.Context(), boards.StalenessKey, &s)
	if err != nil {
		a.fail(c, err)
		return
	}
	s.Examples = s.Examples[:min(n, len(s.Examples))]
	a.cached(c, maxAgeBoard, gin.H{"data": s, "computedAt": at})
}

// GET /v1/models?task=&sort=hot|likes|downloads|new&limit=30&page=1 — the
// HuggingFace model boards; task is a task group, empty for all.
func (a *API) models(c *gin.Context) {
	sort, err := oneOf(c, "sort", boards.ModelSorts...)
	if err != nil {
		a.fail(c, err)
		return
	}
	limit, err := intParam(c, "limit", 30, 1, 100)
	if err != nil {
		a.fail(c, err)
		return
	}
	pg, err := intParam(c, "page", 1, 1, boards.ModelTopN)
	if err != nil {
		a.fail(c, err)
		return
	}
	task := c.Query("task")
	var rows []boards.Model
	at, err := a.st.Board(c.Request.Context(), boards.ModelsKey(task, sort), &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": page(rows, limit, pg), "total": len(rows), "page": pg,
		"task": task, "sort": sort, "computedAt": at})
}

// GET /v1/hot?since=daily|weekly|monthly&limit=25 — GitHub's trending list
// for the window, as scraped last night, in GitHub's order.
func (a *API) hot(c *gin.Context) {
	since, err := oneOf(c, "since", boards.HotWindows...)
	if err != nil {
		a.fail(c, err)
		return
	}
	limit, err := intParam(c, "limit", 25, 1, 50)
	if err != nil {
		a.fail(c, err)
		return
	}
	var rows []boards.HotRepo
	at, err := a.st.Board(c.Request.Context(), boards.HotKey(since), &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": page(rows, limit, 1), "since": since, "computedAt": at})
}
