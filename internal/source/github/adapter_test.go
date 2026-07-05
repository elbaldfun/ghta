package github

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/domain"
)

func TestMapRepo(t *testing.T) {
	var n repoNode
	n.Name = "cli"
	n.Owner.Login = "charmbracelet"
	n.Description = "a tool"
	n.StargazerCount = 12345
	n.ForkCount = 67
	n.Issues.TotalCount = 8
	n.PrimaryLanguage = &struct{ Name string }{Name: "Go"}

	item := mapRepo(n)

	if item.Source != domain.SourceGitHub {
		t.Errorf("source = %q, want github", item.Source)
	}
	if item.ExternalID != "charmbracelet/cli" {
		t.Errorf("externalID = %q, want charmbracelet/cli", item.ExternalID)
	}
	if item.PrimaryMetric != "stars" || item.MetricDirection != domain.DirectionDescBetter {
		t.Errorf("primary metric/direction = %q/%q", item.PrimaryMetric, item.MetricDirection)
	}
	if item.Metrics["stars"] != 12345 || item.Metrics["forks"] != 67 || item.Metrics["openIssues"] != 8 {
		t.Errorf("metrics = %+v", item.Metrics)
	}
	if item.Language != "Go" {
		t.Errorf("language = %q, want Go", item.Language)
	}
	if item.SourceData["owner"] != "charmbracelet" {
		t.Errorf("sourceData.owner = %v", item.SourceData["owner"])
	}
}

// TestFetchPaginates drives the client against a mock GitHub GraphQL server that
// returns two pages, verifying cursor paging, mapping, and stop conditions.
func TestFetchPaginates(t *testing.T) {
	cur := "CURSOR1"
	page1 := searchResponse{}
	page1.Data.RateLimit = rateLimit{Remaining: 4000}
	page1.Data.Search.PageInfo = pageInfo{HasNextPage: true, StartCursor: strptr("A"), EndCursor: &cur}
	page1.Data.Search.Edges = []struct {
		Node repoNode `json:"node"`
	}{{Node: repoWith("owner1", "repo1", 2000)}}

	page2 := searchResponse{}
	page2.Data.RateLimit = rateLimit{Remaining: 3999}
	page2.Data.Search.PageInfo = pageInfo{HasNextPage: false, StartCursor: strptr("B")}
	page2.Data.Search.Edges = []struct {
		Node repoNode `json:"node"`
	}{{Node: repoWith("owner2", "repo2", 1500)}}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		hasAfter := strings.Contains(string(body), "CURSOR1")
		w.Header().Set("Content-Type", "application/json")
		if hasAfter {
			_ = json.NewEncoder(w).Encode(page2)
		} else {
			_ = json.NewEncoder(w).Encode(page1)
		}
		calls++
	}))
	defer srv.Close()

	a := NewAdapter("tok", 200, 50, slog.Default())
	a.client.endpoint = srv.URL

	items, err := a.Fetch(context.Background(), "1000..3000")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if calls != 2 {
		t.Errorf("server calls = %d, want 2", calls)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].ExternalID != "owner1/repo1" || items[1].ExternalID != "owner2/repo2" {
		t.Errorf("externalIDs = %q, %q", items[0].ExternalID, items[1].ExternalID)
	}
	if items[0].Metrics["stars"] != 2000 {
		t.Errorf("stars = %v, want 2000", items[0].Metrics["stars"])
	}
}

func repoWith(owner, name string, stars int) repoNode {
	var n repoNode
	n.Owner.Login = owner
	n.Name = name
	n.StargazerCount = stars
	return n
}

func strptr(s string) *string { return &s }
