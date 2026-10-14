package api

import (
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/store"
)

const replacingTop = 300

var (
	slugPath = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)?$`)
	slugWord = regexp.MustCompile(`^[a-z0-9-]+$`)
)

func (a *API) registerApps(r gin.IRouter) {
	r.GET("/apps", a.apps)
	r.GET("/apps/best/:group/:shelf", a.bestOf)
	r.GET("/alternatives", a.altTargets)
	r.GET("/alternatives/:slug", a.replacing)
}

// GET /v1/apps?os=&kind=app|cli&domain=&shelf=&sort=hot|popular|new&limit=30&page=1
// — the app store. shelf is a shelf slug ("system/screenshot") or a group:
// one segment ("system"), or "dev" for the developer apps.
func (a *API) apps(c *gin.Context) {
	os, err := oneOf(c, "os", append([]string{""}, boards.AppOSes...)...)
	if err != nil {
		a.fail(c, err)
		return
	}
	kind, err := oneOf(c, "kind", "", boards.AppKindApp, boards.AppKindCLI)
	if err != nil {
		a.fail(c, err)
		return
	}
	sort, err := oneOf(c, "sort", boards.AppSorts...)
	if err != nil {
		a.fail(c, err)
		return
	}
	domain, shelf := c.Query("domain"), c.Query("shelf")
	if domain != "" && !slugWord.MatchString(domain) {
		a.fail(c, errBadInput("domain must be a top-level domain"))
		return
	}
	if shelf != "" && !slugPath.MatchString(shelf) {
		a.fail(c, errBadInput("shelf must be a shelf or a shelf group"))
		return
	}
	limit, err := intParam(c, "limit", 30, 1, 100)
	if err != nil {
		a.fail(c, err)
		return
	}
	pg, err := intParam(c, "page", 1, 1, store.AppsDepth)
	if err != nil {
		a.fail(c, err)
		return
	}
	rows := []boards.App{}
	total, err := a.st.ListApps(c.Request.Context(), store.AppQuery{
		OS: os, Kind: kind, Domain: domain, Shelf: shelf, Sort: sort, Skip: (pg - 1) * limit, Limit: limit,
	}, &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": rows, "total": total, "page": pg, "limit": limit})
}

// GET /v1/apps/best/:group/:shelf — a shelf's best-of collection.
func (a *API) bestOf(c *gin.Context) {
	shelf := c.Param("group") + "/" + c.Param("shelf")
	if !slugPath.MatchString(shelf) {
		a.fail(c, errNotFound)
		return
	}
	var rows []boards.App
	at, err := a.st.Board(c.Request.Context(), boards.BestOfKey(shelf), &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": rows, "shelf": shelf, "computedAt": at})
}

// GET /v1/alternatives — the products with the most open-source alternatives.
func (a *API) altTargets(c *gin.Context) {
	var rows []boards.AltTarget
	at, err := a.st.Board(c.Request.Context(), boards.AltTargetsKey, &rows)
	if err != nil {
		a.fail(c, err)
		return
	}
	a.cached(c, maxAgeBoard, gin.H{"data": rows, "computedAt": at})
}

// GET /v1/alternatives/:slug — the open-source apps that replace one product,
// most-starred first, with the product's name.
func (a *API) replacing(c *gin.Context) {
	slug := c.Param("slug")
	if !slugWord.MatchString(slug) {
		a.fail(c, errNotFound)
		return
	}
	rows := []boards.App{}
	if err := a.st.AppsReplacing(c.Request.Context(), slug, replacingTop, &rows); err != nil {
		a.fail(c, err)
		return
	}
	if len(rows) == 0 {
		a.fail(c, errNotFound)
		return
	}
	name := slug
	for _, alt := range rows[0].Alternatives {
		if alt.Slug == slug && alt.Name != "" {
			name = alt.Name
		}
	}
	a.cached(c, maxAgeBoard, gin.H{"data": rows, "name": name, "slug": slug})
}
