package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/pkg/query"
)

func (a *API) registerRepos(r gin.IRouter) {
	r.GET("/repos", a.listRepos)
	r.GET("/search/suggest", a.suggest)
}

// RepoCard is one row of a repository list.
type RepoCard struct {
	FullName    string    `json:"fullName"`
	Owner       string    `json:"owner"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	TaglineZh   string    `json:"taglineZh,omitempty"`
	TaglineEn   string    `json:"taglineEn,omitempty"`
	Language    string    `json:"language,omitempty"`
	License     string    `json:"license,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	IconURL     string    `json:"iconUrl,omitempty"`
	Topics      []string  `json:"topics"`
	Type        string    `json:"type,omitempty"`
	Domain      []string  `json:"domain"`
	Stars       int       `json:"stars"`
	Forks       int       `json:"forks"`
	OpenIssues  int       `json:"openIssues"`
	Growth      Growth    `json:"growth"`
	Rank        int       `json:"rank,omitempty"`
	PushedAt    time.Time `json:"pushedAt,omitzero"`
}

// Growth is the star gain over 1, 7 and 30 days; null means the repo is
// younger than the window, not that it did not grow.
type Growth struct {
	D1  *float64 `json:"d1"`
	D7  *float64 `json:"d7"`
	D30 *float64 `json:"d30"`
}

// cardDescription is how much of a description a card carries. GitHub caps
// descriptions near this, but a few repos carry tens of KB of padding.
const cardDescription = 300

// clip trims s and cuts it to n runes, marking the cut.
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return s
}

func card(it *item.Item) RepoCard {
	owner, name, _ := strings.Cut(it.ExternalID, "/")
	c := RepoCard{
		FullName: it.ExternalID, Owner: owner, Name: name, Description: clip(it.Description, cardDescription),
		TaglineZh: it.Labels.TaglineZh, TaglineEn: it.Labels.TaglineEn,
		Language: it.Language, License: it.License, Homepage: it.Homepage, IconURL: it.Assets.IconURL,
		Topics: it.Topics, Type: it.Labels.Type, Domain: it.Labels.Domain,
		Stars: int(it.Metrics["stars"]), Forks: int(it.Metrics["forks"]), OpenIssues: int(it.Metrics["openIssues"]),
		Growth: Growth{it.Growth.D1, it.Growth.D7, it.Growth.D30},
	}
	// A repo without topics of its own shows the judge's tags, so it still
	// has something to search and relate by.
	if len(c.Topics) == 0 {
		c.Topics = it.Labels.Tags
	}
	if c.Topics == nil {
		c.Topics = []string{}
	}
	if c.Domain == nil {
		c.Domain = []string{}
	}
	if it.Ranks != nil {
		c.Rank = it.Ranks.Overall
	}
	if it.Gh != nil {
		c.PushedAt = it.Gh.PushedAt
	}
	return c
}

// sorts maps the public sort names to stored paths. "relevance" is the text
// search's own order.
var sorts = map[string]string{
	"stars":   "metrics.stars",
	"forks":   "metrics.forks",
	"issues":  "metrics.openIssues",
	"daily":   "growth.d1",
	"weekly":  "growth.d7",
	"monthly": "growth.d30",
	"updated": "gh.pushedAt",
}

const maxListLimit = 50

// GET /v1/repos — a page of GitHub repositories.
//
//	q         text search (relevance order unless sort is given)
//	owner     a GitHub account, exactly as stored
//	language  exact language
//	domain    a leaf path or a top-level domain
//	type      a form (cli, app, library, …)
//	license   exact license name
//	topics    comma-separated; every one must be present
//	stars     range on stars: "a..b", ">n", "<n" or "n"
//	issues    range on open issues, same forms
//	sort      relevance|stars|forks|issues|daily|weekly|monthly|updated, with
//	          ":asc" for ascending; default stars, or relevance with q
//	limit     1..50, default 24; page from 1
func (a *API) listRepos(c *gin.Context) {
	q := store.ListQuery{
		Source:   item.SourceGitHub,
		Q:        strings.TrimSpace(c.Query("q")),
		Owner:    c.Query("owner"),
		Language: c.Query("language"),
		Domain:   c.Query("domain"),
		Type:     c.Query("type"),
		License:  c.Query("license"),
	}
	for _, t := range strings.Split(c.Query("topics"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			q.Topics = append(q.Topics, t)
		}
	}
	var err error
	if q.Stars, err = query.ParseRange(c.Query("stars")); err != nil {
		a.fail(c, errBadInput("stars: "+err.Error()))
		return
	}
	if q.Issues, err = query.ParseRange(c.Query("issues")); err != nil {
		a.fail(c, errBadInput("issues: "+err.Error()))
		return
	}
	name, dir, _ := strings.Cut(c.Query("sort"), ":")
	switch {
	case name == "" || name == "relevance":
	case sorts[name] != "":
		q.Sort = sorts[name]
	default:
		a.fail(c, errBadInput("unsupported sort "+name))
		return
	}
	if dir != "" && dir != "asc" && dir != "desc" {
		a.fail(c, errBadInput("sort order must be asc or desc"))
		return
	}
	q.Asc = dir == "asc"
	if q.Sort == "" && (q.Q == "" || store.ContainsCJK(q.Q)) {
		q.Sort = sorts["stars"]
	}
	limit, err := intParam(c, "limit", 24, 1, maxListLimit)
	if err != nil {
		a.fail(c, err)
		return
	}
	pg, err := intParam(c, "page", 1, 1, store.MaxListTotal/maxListLimit)
	if err != nil {
		a.fail(c, err)
		return
	}
	q.Skip, q.Limit = (pg-1)*limit, limit

	items, total, err := a.st.ListItems(c.Request.Context(), q)
	if err != nil {
		a.fail(c, err)
		return
	}
	rows := make([]RepoCard, len(items))
	for i := range items {
		rows[i] = card(&items[i])
	}
	// total stops at store.MaxListTotal: "10,000+" is all a pager needs.
	a.cached(c, maxAgeList, gin.H{"data": rows, "total": total, "page": pg})
}

// Suggestion is one autocomplete entry.
type Suggestion struct {
	FullName string `json:"fullName"`
	Name     string `json:"name"`
	Stars    int    `json:"stars"`
	Language string `json:"language,omitempty"`
	IconURL  string `json:"iconUrl,omitempty"`
}

// GET /v1/search/suggest?q= — repos whose name starts with q, most starred
// first. Under two characters there is nothing to suggest.
func (a *API) suggest(c *gin.Context) {
	prefix := strings.TrimSpace(c.Query("q"))
	out := []Suggestion{}
	if len([]rune(prefix)) >= 2 {
		items, err := a.st.Suggest(c.Request.Context(), prefix, 8)
		if err != nil {
			a.fail(c, err)
			return
		}
		for _, it := range items {
			out = append(out, Suggestion{FullName: it.ExternalID, Name: it.Name, Stars: int(it.Metrics["stars"]),
				Language: it.Language, IconURL: it.Assets.IconURL})
		}
	}
	a.cached(c, maxAgeList, gin.H{"data": out})
}
