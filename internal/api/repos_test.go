package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func f64(v float64) *float64 { return &v }

func ghRepo(id, lang string, stars, d7 float64, domain ...string) item.Item {
	owner, name, _ := strings.Cut(id, "/")
	return item.Item{
		Source: item.SourceGitHub, ExternalID: id, Name: name, Language: lang, Topics: []string{},
		PrimaryMetric: "stars", Metrics: map[string]float64{"stars": stars, "forks": stars / 10},
		Growth: item.Growth{D7: f64(d7)}, History: []item.Point{},
		Labels: item.Labels{Domain: domain}, Gh: &item.GitHub{Owner: owner},
	}
}

func seed(t *testing.T, st *store.Store, items ...item.Item) {
	t.Helper()
	for _, it := range items {
		if _, err := st.Items().InsertOne(context.Background(), it); err != nil {
			t.Fatal(err)
		}
	}
}

type listPage struct {
	Data  []RepoCard `json:"data"`
	Total int        `json:"total"`
}

func names(p listPage) string {
	var out []string
	for _, c := range p.Data {
		out = append(out, c.FullName)
	}
	return strings.Join(out, ",")
}

func TestListRepos(t *testing.T) {
	st := testStore(t)
	gone := ghRepo("o/gone", "Go", 9e6, 9e6)
	gone.Stale = true
	model := ghRepo("m/model", "", 8e6, 8e6)
	model.Source = item.SourceHuggingFace
	tagged := ghRepo("o/tagged", "Rust", 300, 1, "ai/vision")
	tagged.Labels.Tags = []string{"image"}
	tagged.Labels.Type = "cli"
	tagged.Labels.TaglineZh = "图像笔记工具"
	tagged.Description = strings.Repeat("长", 500)
	topical := ghRepo("o/react-kit", "TypeScript", 200, 50, "web/frontend")
	topical.Topics = []string{"react", "ui"}
	topical.Description = "React component kit"
	seed(t, st, gone, model, tagged, topical,
		ghRepo("o/react", "TypeScript", 100, 70, "web/frontend"),
		ghRepo("o/tie-b", "Go", 50, 5), ghRepo("o/tie-a", "Go", 50, 5),
	)
	r := router(st)

	var p listPage
	get(t, r, "/v1/repos", nil, &p)
	// Stars by default; tombstoned repos and other sources never list; equal
	// stars fall back to the id.
	if names(p) != "o/tagged,o/react-kit,o/react,o/tie-a,o/tie-b" || p.Total != 5 {
		t.Fatalf("default = %s (%d)", names(p), p.Total)
	}
	c := p.Data[0]
	if strings.Join(c.Topics, ",") != "image" || c.Type != "cli" || c.Domain[0] != "ai/vision" || c.Owner != "o" || c.Name != "tagged" {
		t.Fatalf("card = %+v; topics fall back to the judge's tags", c)
	}
	if n := len([]rune(c.Description)); n != cardDescription+1 {
		t.Fatalf("description = %d runes; want the clip and its mark", n)
	}

	get(t, r, "/v1/repos?sort=stars:asc&limit=2&page=1", nil, &p)
	if names(p) != "o/tie-b,o/tie-a" {
		t.Fatalf("ascending = %s; ties reverse with the order", names(p))
	}
	for url, want := range map[string]string{
		"/v1/repos?sort=weekly&limit=2":       "o/react,o/react-kit",
		"/v1/repos?language=Go":               "o/tie-a,o/tie-b",
		"/v1/repos?domain=web":                "o/react-kit,o/react",
		"/v1/repos?domain=ai/vision&type=cli": "o/tagged",
		"/v1/repos?topics=react,ui":           "o/react-kit",
		"/v1/repos?stars=100..250":            "o/react-kit,o/react",
		"/v1/repos?q=react":                   "o/react,o/react-kit", // the exact name first
		"/v1/repos?q=react&sort=stars":        "o/react-kit,o/react",
		"/v1/repos?q=%E7%AC%94%E8%AE%B0":      "o/tagged", // 笔记, in the Chinese tagline
		"/v1/repos?limit=2&page=3":            "o/tie-b",
	} {
		get(t, r, url, nil, &p)
		if names(p) != want {
			t.Errorf("%s = %s; want %s", url, names(p), want)
		}
	}
	for _, url := range []string{"/v1/repos?sort=size", "/v1/repos?sort=stars:up", "/v1/repos?stars=lots", "/v1/repos?limit=51", "/v1/repos?page=0"} {
		if w := get(t, r, url, nil, nil); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d; want 400", url, w.Code)
		}
	}
}

func TestSuggest(t *testing.T) {
	st := testStore(t)
	seed(t, st, ghRepo("o/React", "JS", 10, 0), ghRepo("p/react-dom", "JS", 99, 0), ghRepo("q/preact", "JS", 500, 0))
	r := router(st)
	var out struct {
		Data []Suggestion `json:"data"`
	}
	get(t, r, "/v1/search/suggest?q=rea", nil, &out)
	if len(out.Data) != 2 || out.Data[0].FullName != "p/react-dom" || out.Data[1].Name != "React" {
		t.Fatalf("suggest = %+v; want name prefixes, any case, most stars first", out.Data)
	}
	get(t, r, "/v1/search/suggest?q=r", nil, &out)
	if len(out.Data) != 0 {
		t.Fatalf("one letter suggested %+v", out.Data)
	}
}
