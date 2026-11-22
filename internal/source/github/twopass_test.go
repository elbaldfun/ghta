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
	"time"

	"github.com/elbaldfun/ghta/internal/item"
)

func TestNeedsDetail(t *testing.T) {
	pushed := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	n := repoWith("o", "r", 1200)
	n.PushedAt = &pushed
	n.HomepageURL = "https://r.dev"
	n.PrimaryLanguage = &struct{ Name string }{"Go"}
	st := item.DetailState{PushedAt: pushed, DetailedOn: "2026-11-20", Homepage: "https://r.dev", Language: "Go"}
	today := "2026-11-21"

	if needsDetail(n, st, true, "o/r", today) {
		t.Error("unchanged since yesterday's detailed read: light")
	}
	if !needsDetail(n, st, false, "o/r", today) {
		t.Error("new repository: detail")
	}
	if !needsDetail(n, item.DetailState{PushedAt: pushed}, true, "o/r", today) {
		t.Error("never detailed: detail")
	}
	later := pushed.Add(time.Hour)
	p := n
	p.PushedAt = &later
	if !needsDetail(p, st, true, "o/r", today) {
		t.Error("pushed since: detail")
	}
	p = n
	p.PushedAt = nil
	if !needsDetail(p, st, true, "o/r", today) {
		t.Error("pushedAt gone: detail")
	}
	p = n
	p.HomepageURL = "https://new.dev"
	if !needsDetail(p, st, true, "o/r", today) {
		t.Error("homepage changed (platform detection reads it): detail")
	}
	p = n
	p.RepositoryTopics.Edges = append(p.RepositoryTopics.Edges, struct {
		Node struct {
			Topic struct{ Name string } `json:"topic"`
			URL   string                `json:"url"`
		} `json:"node"`
	}{})
	p.RepositoryTopics.Edges[0].Node.Topic.Name = "macos"
	if !needsDetail(p, st, true, "o/r", today) {
		t.Error("topics changed: detail")
	}

	// The weekly refresh: due 7 + jitter days after the last detailed read.
	due := detailRefresh + int(jitter("o/r"))
	old := st
	old.DetailedOn = time.Date(2026, 11, 21-due+1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	if needsDetail(n, old, true, "o/r", today) {
		t.Errorf("%d days after its read, a day before its refresh: light", due-1)
	}
	old.DetailedOn = time.Date(2026, 11, 21-due, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	if !needsDetail(n, old, true, "o/r", today) {
		t.Errorf("%d days after its read: refresh", due)
	}

	// The jitter spreads repositories over the week.
	seen := map[uint32]bool{}
	for _, id := range []string{"a/a", "b/b", "c/c", "d/d", "e/e", "f/f", "g/g", "h/h", "i/i", "j/j", "k/k", "l/l", "m/m", "n/n"} {
		j := jitter(id)
		if j >= detailJitter {
			t.Fatalf("jitter %d out of range", j)
		}
		seen[j] = true
	}
	if len(seen) < 4 {
		t.Errorf("jitter barely spreads: %v", seen)
	}
}

// TestFetchItemsTwoPass drives the light search and the by-id detail read
// against a mock GraphQL server.
func TestFetchItemsTwoPass(t *testing.T) {
	pushed := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	later := pushed.Add(time.Hour)
	mk := func(name, id string, stars int, at *time.Time) repoNode {
		n := repoWith("o", name, stars)
		n.ID, n.PushedAt = id, at
		return n
	}
	light := []repoNode{
		mk("same", "N1", 1200, &pushed),    // unchanged: light
		mk("pushed", "N2", 1300, &later),   // pushed since: detail
		mk("new", "N3", 1400, &pushed),     // not stored: detail
		mk("vanished", "N4", 1500, &later), // pushed, but gone by the detail read
	}
	detail := func(n repoNode) *repoNode {
		n.Readme = &struct{ Text string }{"# " + n.Name}
		n.Releases.Edges = append(n.Releases.Edges, struct {
			Node struct {
				Name          string     `json:"name"`
				TagName       string     `json:"tagName"`
				IsPrerelease  bool       `json:"isPrerelease"`
				IsLatest      bool       `json:"isLatest"`
				IsDraft       bool       `json:"isDraft"`
				PublishedAt   *time.Time `json:"publishedAt"`
				ReleaseAssets struct {
					Nodes []struct {
						Name        string `json:"name"`
						DownloadURL string `json:"downloadUrl"`
						Size        int64  `json:"size"`
					} `json:"nodes"`
				} `json:"releaseAssets"`
			} `json:"node"`
		}{})
		n.Releases.Edges[0].Node.TagName = "v1"
		n.Releases.Edges[0].Node.IsLatest = true
		return &n
	}

	var searches int
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "nodes(ids:") {
			var out struct {
				Data struct {
					RateLimit rateLimit   `json:"rateLimit"`
					Nodes     []*repoNode `json:"nodes"`
				} `json:"data"`
				Errors []graphqlError `json:"errors"`
			}
			out.Data.RateLimit = rateLimit{Remaining: 4000}
			for _, id := range req.Variables["ids"].([]any) {
				asked = append(asked, id.(string))
				var hit *repoNode
				for _, n := range light {
					if n.ID == id && n.ID != "N4" {
						hit = detail(n)
					}
				}
				if hit == nil {
					out.Errors = append(out.Errors, graphqlError{Type: "NOT_FOUND", Message: "gone"})
				}
				out.Data.Nodes = append(out.Data.Nodes, hit)
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		if strings.Contains(req.Query, "README.md") {
			t.Error("the light search must not read the README")
		}
		searches++
		page := searchResponse{}
		page.Data.RateLimit = rateLimit{Remaining: 4000}
		page.Data.Search.PageInfo = pageInfo{StartCursor: strptr("A")}
		for _, n := range light {
			page.Data.Search.Edges = append(page.Data.Search.Edges, struct {
				Node repoNode `json:"node"`
			}{Node: n})
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()

	a := NewAdapter("tok", 50, 50, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.client.endpoint = srv.URL
	a.now = func() time.Time { return time.Date(2026, 11, 21, 3, 30, 0, 0, time.UTC) }
	stored := item.DetailState{PushedAt: pushed, DetailedOn: "2026-11-20", Topics: []string{}}
	a.SetDetailLookup(func(_ context.Context, ids []string) (map[string]item.DetailState, error) {
		return map[string]item.DetailState{"o/same": stored, "o/pushed": stored, "o/vanished": stored}, nil
	})

	got, err := a.FetchItems(context.Background(), "1000..1010")
	if err != nil {
		t.Fatal(err)
	}
	if searches != 1 || strings.Join(asked, ",") != "N2,N3,N4" {
		t.Fatalf("searches=%d detail ids=%v; want one light page, then N2,N3,N4 by id", searches, asked)
	}
	by := map[string]item.Fetched{}
	for _, f := range got {
		by[f.Item.ExternalID] = f
	}
	if len(by) != 4 {
		t.Fatalf("got %d items", len(by))
	}
	for _, id := range []string{"o/same", "o/vanished"} {
		f := by[id]
		if !f.Light || f.Blob != nil || f.Item.Gh.DetailedOn != "" {
			t.Errorf("%s: want a light reading, got light=%v blob=%v detailedOn=%q", id, f.Light, f.Blob != nil, f.Item.Gh.DetailedOn)
		}
	}
	for _, id := range []string{"o/pushed", "o/new"} {
		f := by[id]
		if f.Light || f.Blob == nil || f.Blob.Readme == "" || f.Item.Gh.LatestRelease == nil || f.Item.Gh.DetailedOn != "2026-11-21" {
			t.Errorf("%s: want a full reading, got %+v", id, f)
		}
	}
	// Metrics come from the light search for every item, read or not in full.
	if by["o/same"].Item.Metrics["stars"] != 1200 || by["o/new"].Item.Metrics["stars"] != 1400 ||
		!by["o/same"].Item.Gh.PushedAt.Equal(pushed) {
		t.Errorf("metrics: same=%v new=%v", by["o/same"].Item.Metrics, by["o/new"].Item.Metrics)
	}
}
