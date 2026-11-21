package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
)

func TestParseRepo(t *testing.T) {
	for in, want := range map[string]string{
		"golang/go":                             "golang/go",
		" golang/go/ ":                          "golang/go",
		"https://github.com/golang/go":          "golang/go",
		"github.com/golang/go.git":              "golang/go",
		"https://starrank.dev/repo/golang/go":   "golang/go",
		"https://starrank.dev/zh/repo/a.b/c_d":  "a.b/c_d",
		"https://www.github.com/vercel/next.js": "vercel/next.js",
	} {
		if got, ok := parseRepo(in); !ok || got != want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "golang", "a/b/c", "https://example.com/a/b", "a b/c", "$where/x"} {
		if got, ok := parseRepo(in); ok {
			t.Errorf("parseRepo(%q) = %q; want rejected", in, got)
		}
	}
}

func TestMCPLimit(t *testing.T) {
	if n, err := mcpLimit(0); n != mcpDefaultLimit || err != nil {
		t.Fatalf("default = %d %v", n, err)
	}
	for _, bad := range []int{-1, mcpMaxLimit + 1, 1000} {
		if _, err := mcpLimit(bad); err == nil {
			t.Fatalf("limit %d accepted", bad)
		}
	}
	if err := checkFilters("ai/agents", "C++"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"AI", "ai/", "ai/agents/x", ".*", "a$b"} {
		if checkFilters(bad, "") == nil {
			t.Fatalf("domain %q accepted", bad)
		}
	}
}

func TestClientIP(t *testing.T) {
	req := func(remote, xff string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	// Behind the edge proxy (a private peer) the last forwarded entry counts.
	if ip := clientIP(req("172.18.0.5:5000", "1.2.3.4, 5.6.7.8")); ip != "5.6.7.8" {
		t.Fatalf("proxied = %s", ip)
	}
	// A public peer cannot claim another address.
	if ip := clientIP(req("9.9.9.9:5000", "1.2.3.4")); ip != "9.9.9.9" {
		t.Fatalf("direct = %s", ip)
	}
	if ip := clientIP(req("172.18.0.5:5000", "garbage")); ip != "172.18.0.5" {
		t.Fatalf("bad xff = %s", ip)
	}
}

func TestMCPLimiter(t *testing.T) {
	l := NewMCPLimiter()
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	for i := range mcpPerClientBurst {
		if !l.allow("a") {
			t.Fatalf("call %d refused within the burst", i)
		}
	}
	if l.allow("a") {
		t.Fatal("call past the burst allowed")
	}
	if !l.allow("b") {
		t.Fatal("another client is limited by the first")
	}
	now = now.Add(2 * time.Second)
	if !l.allow("a") {
		t.Fatal("no refill after two seconds")
	}

	// Calls beyond the in-flight cap are refused, not queued.
	block := make(chan struct{})
	h := l.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	done := make(chan int, mcpMaxInFlight+1)
	for i := range mcpMaxInFlight + 1 {
		go func() {
			r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
			r.RemoteAddr = "10.0.0.1:1"
			r.Header.Set("X-Forwarded-For", "8.8.8."+string(rune('0'+i)))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			done <- w.Code
		}()
	}
	if code := <-done; code != http.StatusTooManyRequests {
		t.Fatalf("over the cap = %d", code)
	}
	close(block)
	for range mcpMaxInFlight {
		if code := <-done; code != http.StatusOK {
			t.Fatalf("within the cap = %d", code)
		}
	}
}

// TestMCPTools calls every tool over Streamable HTTP, as a client would.
func TestMCPTools(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	agent := ghRepo("acme/agentkit", "Python", 5000, 300, "ai/agents")
	agent.Description = "An agent framework"
	agent.Labels.TaglineEn = "Build LLM agents"
	agent.Growth.D1 = f64(40.4)
	agent.Ranks = &item.Ranks{Overall: 12, Language: 3, Domain: map[string]int{"ai/agents": 1},
		Prev: &item.RankPositions{Overall: 15, Language: 3, Domain: map[string]int{"ai/agents": 2}}}
	vision := ghRepo("acme/seer", "Rust", 9000, 10, "ai/vision")
	vision.Description = "Fast image agent toolkit"
	vision.Growth.D1 = f64(2)
	web := ghRepo("other/webkit", "TypeScript", 100, 90, "web/frontend")
	web.Growth.D1 = f64(80)
	gone := ghRepo("old/name", "Go", 1, 0)
	gone.Stale, gone.StaleReason = true, "renamed:acme/seer"
	seed(t, st, agent, vision, web, gone)
	must(t, st.PutBoard(ctx, boards.CategoriesKey, []boards.CategoryNode{
		{Path: "ai", Name: "人工智能", NameEn: "AI", Count: 2, Children: []boards.CategoryNode{
			{Path: "ai/agents", Name: "智能体", NameEn: "Agents", Count: 1}}},
		{Path: "web", Name: "Web", Count: 1},
	}, time.Now()))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(st, nil).RegisterMCP(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var toolNames []string
	for _, tl := range tools.Tools {
		toolNames = append(toolNames, tl.Name)
		if tl.Annotations == nil || !tl.Annotations.ReadOnlyHint {
			t.Errorf("%s is not marked read-only", tl.Name)
		}
	}
	if got := strings.Join(toolNames, ","); got != "get_repo,list_domains,search_repos,top_repos,trending_today" {
		t.Fatalf("tools = %s", got)
	}

	call := func(name string, args map[string]any, out any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if out != nil && !res.IsError {
			b, _ := json.Marshal(res.StructuredContent)
			if err := json.Unmarshal(b, out); err != nil {
				t.Fatal(err)
			}
		}
		return res
	}
	repos := func(l MCPList) string {
		var ids []string
		for _, r := range l.Repos {
			ids = append(ids, r.Repo)
		}
		return strings.Join(ids, ",")
	}

	var l MCPList
	call("search_repos", map[string]any{"query": "agent"}, &l)
	if !strings.Contains(repos(l), "acme/agentkit") || l.URL != "https://starrank.dev/github?q=agent" {
		t.Fatalf("search = %s %s", repos(l), l.URL)
	}
	if a := l.Repos[0]; a.Repo == "acme/agentkit" && (a.Description != "Build LLM agents" || *a.Growth.D1 != 40 || a.URL != "https://starrank.dev/repo/acme/agentkit") {
		t.Fatalf("row = %+v", a)
	}

	call("top_repos", map[string]any{"domain": "ai", "sort": "weekly"}, &l)
	if repos(l) != "acme/agentkit,acme/seer" || l.Total != 2 || l.URL != "https://starrank.dev/github?category=ai&sort=weekly" {
		t.Fatalf("top = %s %d %s", repos(l), l.Total, l.URL)
	}
	call("top_repos", map[string]any{"language": "Rust", "limit": 1}, &l)
	if repos(l) != "acme/seer" {
		t.Fatalf("top rust = %s", repos(l))
	}

	call("trending_today", nil, &l)
	if repos(l) != "other/webkit,acme/agentkit,acme/seer" {
		t.Fatalf("trending = %s", repos(l))
	}

	var d MCPRepoDetail
	call("get_repo", map[string]any{"repo": "https://github.com/acme/agentkit"}, &d)
	up3, same, up1 := 3, 0, 1
	if len(d.Ranks) != 3 || *d.Ranks[0].Change != up3 || *d.Ranks[1].Change != same || *d.Ranks[2].Change != up1 ||
		d.Ranks[2].URL != "https://starrank.dev/github?category=ai%2Fagents" || d.GitHub != "https://github.com/acme/agentkit" {
		t.Fatalf("detail = %+v", d)
	}
	// A rename is followed; case is forgiven.
	call("get_repo", map[string]any{"repo": "old/name"}, &d)
	if d.Repo != "acme/seer" {
		t.Fatalf("renamed = %s", d.Repo)
	}
	call("get_repo", map[string]any{"repo": "ACME/AgentKit"}, &d)
	if d.Repo != "acme/agentkit" {
		t.Fatalf("case = %s", d.Repo)
	}

	var dom domainsOut
	call("list_domains", nil, &dom)
	if len(dom.Domains) != 2 || dom.Domains[0].Name != "AI" || dom.Domains[1].Name != "Web" {
		t.Fatalf("domains = %+v", dom)
	}
	call("list_domains", map[string]any{"parent": "ai"}, &dom)
	if len(dom.Domains) != 1 || dom.Domains[0].Path != "ai/agents" {
		t.Fatalf("leaves = %+v", dom)
	}

	// Bad input is a tool error the agent can read, not a protocol failure.
	for name, args := range map[string]map[string]any{
		"get_repo":     {"repo": "nobody/nothing"},
		"search_repos": {"query": "a"},
		"top_repos":    {"limit": 500},
		"list_domains": {"parent": "nope"},
	} {
		if res := call(name, args, nil); !res.IsError {
			t.Errorf("%s(%v) succeeded", name, args)
		}
	}
	if res := call("top_repos", map[string]any{"sort": "forks"}, nil); !res.IsError {
		t.Error("unsupported sort accepted")
	}
}
