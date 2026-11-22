package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

// The MCP server: the same read paths as /v1, offered as tools to coding
// agents (Claude Code, Cursor, …) over Streamable HTTP at /mcp.
//
// It is stateless and answers in plain JSON, so it holds no sessions and no
// open streams: each POST is one query against the store, like a /v1 call.
// Every tool has a small result cap and a short deadline; the HTTP side
// (mcp_limit.go) rate-limits by client and caps concurrent calls.

// SiteURL is where the tools' links point.
const SiteURL = "https://starrank.dev"

// Tool limits. An agent reads every row it gets, so the defaults are small.
const (
	mcpDefaultLimit = 10
	mcpMaxLimit     = 25
	mcpQueryTimeout = 8 * time.Second
	mcpMaxQuery     = 100
	mcpDescription  = 160
)

// MCPVersion is reported to clients as the server version.
var MCPVersion = "1.0.0"

const mcpInstructions = `StarRank (starrank.dev) ranks open-source GitHub repositories by stars and star growth, grouped by domain (a taxonomy such as "ai/agents") and language. Data refreshes once a day.
Use search_repos to find projects by keyword, top_repos for a ranking (filter by domain/language, sort by total stars or growth), trending_today for what is gaining stars right now, get_repo for one repository's ranks and growth, and list_domains to learn the domain paths top_repos accepts.
Growth numbers are stars gained over the last 1/7/30 days; null means the repository is younger than the window. Every result carries a starrank.dev link; cite it when you show results.`

// MCPRepo is one repository as the tools return it: the card, trimmed to
// what an agent needs.
type MCPRepo struct {
	Repo        string    `json:"repo" jsonschema:"owner/name on GitHub"`
	Description string    `json:"description,omitempty"`
	Language    string    `json:"language,omitempty"`
	Domains     []string  `json:"domains,omitempty" jsonschema:"StarRank domain paths, main one first"`
	Stars       int       `json:"stars"`
	Growth      MCPGrowth `json:"growth"`
	Rank        int       `json:"rank,omitempty" jsonschema:"overall StarRank position"`
	URL         string    `json:"url" jsonschema:"the repository's StarRank page"`
}

// MCPGrowth is the stars gained over 1, 7 and 30 days; null where the
// repository is younger than the window.
type MCPGrowth struct {
	D1  *int `json:"d1"`
	D7  *int `json:"d7"`
	D30 *int `json:"d30"`
}

// MCPList is a list result.
type MCPList struct {
	Repos []MCPRepo `json:"repos"`
	// Total is how many repositories match, counted up to 10,000.
	Total int    `json:"total,omitempty"`
	URL   string `json:"url" jsonschema:"the same list on starrank.dev"`
}

// MCPRepoDetail is get_repo's result.
type MCPRepoDetail struct {
	MCPRepo
	Forks      int       `json:"forks"`
	OpenIssues int       `json:"openIssues"`
	License    string    `json:"license,omitempty"`
	Homepage   string    `json:"homepage,omitempty"`
	Topics     []string  `json:"topics,omitempty"`
	Type       string    `json:"type,omitempty" jsonschema:"form: cli, app, library, …"`
	Ranks      []MCPRank `json:"ranks"`
	PushedAt   string    `json:"pushedAt,omitempty" jsonschema:"last push, YYYY-MM-DD"`
	GitHub     string    `json:"github"`
}

// MCPRank is a position in one ranking.
type MCPRank struct {
	Scope string `json:"scope" jsonschema:"overall, language or domain"`
	Key   string `json:"key,omitempty" jsonschema:"the language or domain path"`
	Rank  int    `json:"rank"`
	// Change is the places gained since the previous ranking, negative for
	// places lost; absent when there is nothing to compare with.
	Change *int `json:"change,omitempty" jsonschema:"places gained since the previous day's ranking; negative means it dropped"`
	// New marks a position the repo did not hold at the previous ranking.
	New bool   `json:"new,omitempty" jsonschema:"true when the repo was not in this ranking the previous day"`
	URL string `json:"url,omitempty"`
}

// MCPDomain is one node of the domain tree.
type MCPDomain struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	NameZh string `json:"nameZh,omitempty"`
	Count  int    `json:"count" jsonschema:"repositories in the domain"`
}

type searchIn struct {
	Query    string `json:"query" jsonschema:"keywords, e.g. 'vector database' or 'rust web framework'"`
	Language string `json:"language,omitempty" jsonschema:"exact GitHub language name, e.g. Rust, TypeScript, Python"`
	Limit    int    `json:"limit,omitempty" jsonschema:"rows to return, 1-25, default 10"`
}

type topIn struct {
	Domain   string `json:"domain,omitempty" jsonschema:"a domain path from list_domains, top-level ('ai') or leaf ('ai/agents')"`
	Language string `json:"language,omitempty" jsonschema:"exact GitHub language name, e.g. Rust, Go, C++"`
	Sort     string `json:"sort,omitempty" jsonschema:"stars (total, default), daily, weekly or monthly (stars gained in the window)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"rows to return, 1-25, default 10"`
}

type trendingIn struct {
	Domain   string `json:"domain,omitempty" jsonschema:"optional domain path from list_domains"`
	Language string `json:"language,omitempty" jsonschema:"optional exact GitHub language name"`
	Limit    int    `json:"limit,omitempty" jsonschema:"rows to return, 1-25, default 10"`
}

type repoIn struct {
	Repo string `json:"repo" jsonschema:"owner/name, or a github.com or starrank.dev URL of the repository"`
}

type domainsIn struct {
	Parent string `json:"parent,omitempty" jsonschema:"a top-level domain to list its leaves; empty lists the top level only"`
}

type domainsOut struct {
	Domains []MCPDomain `json:"domains"`
}

// mcpSorts are the sorts top_repos offers, by public name.
var mcpSorts = map[string]string{"stars": "metrics.stars", "daily": "growth.d1", "weekly": "growth.d7", "monthly": "growth.d30"}

var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)}

func ptr[T any](v T) *T { return &v }

// MCPServer builds the MCP server with StarRank's tools.
func (a *API) MCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "starrank", Title: "StarRank", Version: MCPVersion, WebsiteURL: SiteURL},
		&mcp.ServerOptions{Instructions: mcpInstructions})
	mcp.AddTool(s, &mcp.Tool{Name: "search_repos", Title: "Search repositories", Annotations: readOnly,
		Description: "Search open-source GitHub repositories tracked by StarRank by keyword, most relevant first. Returns stars, star growth, rank and a starrank.dev link per repository."},
		a.mcpSearch)
	mcp.AddTool(s, &mcp.Tool{Name: "top_repos", Title: "Top repositories", Annotations: readOnly,
		Description: "A StarRank ranking of GitHub repositories, optionally within a domain (see list_domains) and/or a language, ordered by total stars or by stars gained over a day, week or month."},
		a.mcpTop)
	mcp.AddTool(s, &mcp.Tool{Name: "trending_today", Title: "Trending today", Annotations: readOnly,
		Description: "Repositories that gained the most stars in the last day, optionally within a domain or language."},
		a.mcpTrending)
	mcp.AddTool(s, &mcp.Tool{Name: "get_repo", Title: "Repository details", Annotations: readOnly,
		Description: "One repository's StarRank details: stars, forks, star growth, its overall, language and domain ranks with the change since the previous day, and links."},
		a.mcpRepo)
	mcp.AddTool(s, &mcp.Tool{Name: "list_domains", Title: "List domains", Annotations: readOnly,
		Description: "StarRank's domain taxonomy with repository counts: the top-level domains, or the leaves of one. Use the paths as top_repos' domain."},
		a.mcpDomains)
	return s
}

// RegisterMCP mounts the MCP endpoint at /mcp, behind the limits.
func (a *API) RegisterMCP(r gin.IRouter) {
	h := gin.WrapH(NewMCPLimiter().Wrap(a.MCPHandler()))
	r.POST("/mcp", h)
	r.GET("/mcp", h)
	r.DELETE("/mcp", h)
}

// MCPHandler serves the MCP server over Streamable HTTP, statelessly and
// with JSON responses.
func (a *API) MCPHandler() http.Handler {
	s := a.MCPServer()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

func mcpLimit(n int) (int, error) {
	switch {
	case n == 0:
		return mcpDefaultLimit, nil
	case n < 1 || n > mcpMaxLimit:
		return 0, fmt.Errorf("limit must be 1..%d", mcpMaxLimit)
	}
	return n, nil
}

// domainPath accepts a domain path as list_domains gives it.
var domainPath = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(/[a-z0-9][a-z0-9-]*)?$`)

func checkFilters(domain, language string) error {
	if domain != "" && !domainPath.MatchString(domain) {
		return fmt.Errorf("domain %q is not a domain path; call list_domains", domain)
	}
	if len(language) > 40 {
		return fmt.Errorf("language is too long")
	}
	return nil
}

func (a *API) mcpList(ctx context.Context, q store.ListQuery, site url.Values) (*mcp.CallToolResult, MCPList, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpQueryTimeout)
	defer cancel()
	q.Source = item.SourceGitHub
	items, total, err := a.st.ListItems(ctx, q)
	if err != nil {
		a.log.Error("mcp: list failed", "err", err)
		return nil, MCPList{}, fmt.Errorf("the query failed; try again later")
	}
	out := MCPList{Repos: make([]MCPRepo, len(items)), Total: int(total), URL: boardURL(site)}
	for i := range items {
		out.Repos[i] = mcpRepo(&items[i])
	}
	return nil, out, nil
}

func (a *API) mcpSearch(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, MCPList, error) {
	q := strings.TrimSpace(in.Query)
	if len([]rune(q)) < 2 || len([]rune(q)) > mcpMaxQuery {
		return nil, MCPList{}, fmt.Errorf("query must be 2..%d characters", mcpMaxQuery)
	}
	limit, err := mcpLimit(in.Limit)
	if err != nil {
		return nil, MCPList{}, err
	}
	if err := checkFilters("", in.Language); err != nil {
		return nil, MCPList{}, err
	}
	lq := store.ListQuery{Q: q, Language: in.Language, Limit: limit}
	// CJK text has no relevance order (store.ListQuery.Q): stars stand in.
	if store.ContainsCJK(q) {
		lq.Sort = "metrics.stars"
	}
	site := url.Values{"q": {q}}
	if in.Language != "" {
		site.Set("lang", in.Language)
	}
	return a.mcpList(ctx, lq, site)
}

func (a *API) mcpTop(ctx context.Context, _ *mcp.CallToolRequest, in topIn) (*mcp.CallToolResult, MCPList, error) {
	limit, err := mcpLimit(in.Limit)
	if err != nil {
		return nil, MCPList{}, err
	}
	if err := checkFilters(in.Domain, in.Language); err != nil {
		return nil, MCPList{}, err
	}
	sortName := in.Sort
	if sortName == "" {
		sortName = "stars"
	}
	path, ok := mcpSorts[sortName]
	if !ok {
		return nil, MCPList{}, fmt.Errorf("sort must be stars, daily, weekly or monthly")
	}
	site := url.Values{}
	if in.Domain != "" {
		site.Set("category", in.Domain)
	}
	if in.Language != "" {
		site.Set("lang", in.Language)
	}
	if sortName != "stars" {
		site.Set("sort", sortName)
	}
	return a.mcpList(ctx, store.ListQuery{Domain: in.Domain, Language: in.Language, Sort: path, Limit: limit}, site)
}

func (a *API) mcpTrending(ctx context.Context, req *mcp.CallToolRequest, in trendingIn) (*mcp.CallToolResult, MCPList, error) {
	return a.mcpTop(ctx, req, topIn{Domain: in.Domain, Language: in.Language, Sort: "daily", Limit: in.Limit})
}

// repoRef reads owner/name out of what an agent may pass for a repository.
var repoRef = regexp.MustCompile(`^(?:https?://)?(?:www\.)?(?:github\.com/|starrank\.dev/(?:[a-zA-Z-]+/)?repo/)?([A-Za-z0-9_.-]{1,100})/([A-Za-z0-9_.-]{1,100})/?$`)

func parseRepo(s string) (string, bool) {
	m := repoRef.FindStringSubmatch(strings.TrimSuffix(strings.TrimSpace(s), ".git"))
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2], true
}

func (a *API) mcpRepo(ctx context.Context, _ *mcp.CallToolRequest, in repoIn) (*mcp.CallToolResult, MCPRepoDetail, error) {
	id, ok := parseRepo(in.Repo)
	if !ok {
		return nil, MCPRepoDetail{}, fmt.Errorf("repo must be owner/name, e.g. golang/go")
	}
	ctx, cancel := context.WithTimeout(ctx, mcpQueryTimeout)
	defer cancel()
	it, err := a.st.Item(ctx, item.SourceGitHub, id)
	// The id is matched as stored; GitHub names are case-insensitive, so a
	// miss retries through the case-insensitive name index.
	if errors.Is(err, store.ErrNoItem) {
		if found, ferr := a.findRepo(ctx, id); ferr == nil {
			it, err = found, nil
		}
	}
	if err != nil {
		if errors.Is(err, store.ErrNoItem) {
			return nil, MCPRepoDetail{}, fmt.Errorf("%s is not tracked by StarRank (it may be too small or not a GitHub repository)", id)
		}
		a.log.Error("mcp: repo failed", "id", id, "err", err)
		return nil, MCPRepoDetail{}, fmt.Errorf("the query failed; try again later")
	}
	for range 2 { // follow a rename, once
		to := movedTo(&it)
		if to == "" {
			break
		}
		if it, err = a.st.Item(ctx, item.SourceGitHub, to); err != nil {
			return nil, MCPRepoDetail{}, fmt.Errorf("%s was renamed to %s, which is not tracked", id, to)
		}
	}
	if it.Stale {
		return nil, MCPRepoDetail{}, fmt.Errorf("%s is no longer on GitHub", id)
	}
	return nil, mcpDetail(&it), nil
}

// findRepo looks a repository up by name, ignoring case.
func (a *API) findRepo(ctx context.Context, id string) (item.Item, error) {
	_, name, _ := strings.Cut(id, "/")
	items, err := a.st.Suggest(ctx, name, 8)
	if err != nil {
		return item.Item{}, err
	}
	for _, it := range items {
		if strings.EqualFold(it.ExternalID, id) {
			return a.st.Item(ctx, item.SourceGitHub, it.ExternalID)
		}
	}
	return item.Item{}, store.ErrNoItem
}

func (a *API) mcpDomains(ctx context.Context, _ *mcp.CallToolRequest, in domainsIn) (*mcp.CallToolResult, domainsOut, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpQueryTimeout)
	defer cancel()
	var tree []boards.CategoryNode
	if _, err := a.st.Board(ctx, boards.CategoriesKey, &tree); err != nil {
		a.log.Error("mcp: categories failed", "err", err)
		return nil, domainsOut{}, fmt.Errorf("the domain list is unavailable; try again later")
	}
	out := domainsOut{Domains: []MCPDomain{}}
	nodes := tree
	if p := strings.TrimSpace(in.Parent); p != "" {
		nodes = nil
		for _, n := range tree {
			if n.Path == p {
				nodes = n.Children
			}
		}
		if nodes == nil {
			return nil, domainsOut{}, fmt.Errorf("%q is not a top-level domain; call list_domains without parent", p)
		}
	}
	for _, n := range nodes {
		d := MCPDomain{Path: n.Path, Name: n.NameEn, NameZh: n.Name, Count: n.Count}
		if d.Name == "" {
			d.Name, d.NameZh = n.Name, ""
		}
		out.Domains = append(out.Domains, d)
	}
	return nil, out, nil
}

func growthInt(p *float64) *int {
	if p == nil {
		return nil
	}
	n := int(math.Round(*p))
	return &n
}

func repoURL(id string) string { return SiteURL + "/repo/" + id }

// boardURL is the GitHub board with these filters; the site redirects a
// category or language to its canonical path.
func boardURL(q url.Values) string {
	if len(q) == 0 {
		return SiteURL + "/github"
	}
	return SiteURL + "/github?" + q.Encode()
}

func mcpRepo(it *item.Item) MCPRepo {
	desc := it.Labels.TaglineEn
	if desc == "" {
		desc = it.Description
	}
	r := MCPRepo{
		Repo: it.ExternalID, Description: clip(desc, mcpDescription), Language: it.Language, Domains: it.Labels.Domain,
		Stars: int(it.Metrics["stars"]), URL: repoURL(it.ExternalID),
		Growth: MCPGrowth{growthInt(it.Growth.D1), growthInt(it.Growth.D7), growthInt(it.Growth.D30)},
	}
	if it.Ranks != nil {
		r.Rank = it.Ranks.Overall
	}
	return r
}

func mcpDetail(it *item.Item) MCPRepoDetail {
	d := MCPRepoDetail{
		MCPRepo: mcpRepo(it), Forks: int(it.Metrics["forks"]), OpenIssues: int(it.Metrics["openIssues"]),
		License: it.License, Homepage: it.Homepage, Topics: it.Topics, Type: it.Labels.Type,
		Ranks: []MCPRank{}, GitHub: "https://github.com/" + it.ExternalID,
	}
	if len(d.Topics) > 10 {
		d.Topics = d.Topics[:10]
	}
	if it.Gh != nil && !it.Gh.PushedAt.IsZero() {
		d.PushedAt = it.Gh.PushedAt.UTC().Format("2006-01-02")
	}
	if r := it.Ranks; r != nil {
		// The same move as /v1/repos shows: scopeRank in detail.go.
		add := func(scope, key string, now int, url string) {
			change, isNew := rankMove(r, scope, key, now)
			d.Ranks = append(d.Ranks, MCPRank{Scope: scope, Key: key, Rank: now, Change: change, New: isNew, URL: url})
		}
		if r.Overall > 0 {
			add("overall", "", r.Overall, SiteURL+"/github")
		}
		if r.Language > 0 {
			add("language", it.Language, r.Language, boardURL(url.Values{"lang": {it.Language}}))
		}
		for _, p := range it.Labels.Domain {
			if n := r.Domain[p]; n > 0 {
				add("domain", p, n, boardURL(url.Values{"category": {p}}))
			}
		}
	}
	return d
}
