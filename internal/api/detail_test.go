package api

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func TestRepoDetail(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	gui := true
	it := ghRepo("o/app", "Go", 5000, 10, "apps/productivity/notes", "devtools/terminal")
	it.Ranks = &item.Ranks{Overall: 40, Language: 7, Domain: map[string]int{"devtools/terminal": 3, "apps/productivity/notes": 1}}
	it.Labels.Shelf, it.Labels.HasGui = "productivity/notes", &gui
	it.Labels.Alternatives = []item.Alternative{{Name: "Notion", Slug: "notion"}}
	it.Gh.Platforms = []string{"macos"}
	it.Gh.LatestRelease = &item.ReleaseRef{Tag: "v1.2"}
	it.History = []item.Point{{D: "2026-10-13", V: 4990}}
	excluded := ghRepo("o/lib", "Go", 10, 0)
	excluded.Labels.Shelf = item.ShelfExcluded
	seed(t, st, it, excluded)
	if _, err := st.Blobs().InsertOne(ctx, item.Blob{Source: item.SourceGitHub, ExternalID: "o/app", Readme: "# App"}); err != nil {
		t.Fatal(err)
	}
	r := router(st)

	var d struct {
		Data RepoDetail `json:"data"`
	}
	w := get(t, r, "/v1/repos/o/app", nil, &d)
	want := []Rank{{"overall", "", 40}, {"language", "Go", 7}, {"domain", "apps/productivity/notes", 1}, {"domain", "devtools/terminal", 3}}
	if w.Code != http.StatusOK || !reflect.DeepEqual(d.Data.Ranks, want) {
		t.Fatalf("ranks = %d %+v", w.Code, d.Data.Ranks)
	}
	if d.Data.Shelf != "productivity/notes" || !*d.Data.HasGui || d.Data.Alternatives[0].Slug != "notion" ||
		d.Data.Platforms[0] != "macos" || d.Data.LatestRelease.Tag != "v1.2" || d.Data.Stars != 5000 {
		t.Fatalf("detail = %+v", d.Data)
	}
	if w.Header().Get("Cache-Control") != "public, max-age=600" {
		t.Fatalf("cache = %q", w.Header().Get("Cache-Control"))
	}
	var lib struct {
		Data RepoDetail `json:"data"`
	}
	get(t, r, "/v1/repos/o/lib", nil, &lib)
	if lib.Data.Shelf != "" || lib.Data.Ranks == nil || len(lib.Data.Ranks) != 0 || lib.Data.LatestRelease != nil {
		t.Fatalf("an excluded, unranked repo = %+v", lib.Data)
	}

	var rd struct {
		Data struct{ Markdown string } `json:"data"`
	}
	if get(t, r, "/v1/repos/o/app/readme", nil, &rd); rd.Data.Markdown != "# App" {
		t.Fatalf("readme = %+v", rd)
	}
	if get(t, r, "/v1/repos/o/lib/readme", nil, &rd); rd.Data.Markdown != "" {
		t.Fatalf("no blob = %+v", rd)
	}
	for _, url := range []string{"/v1/repos/o/none", "/v1/repos/o/none/readme", "/v1/repos/o/none/history"} {
		if w := get(t, r, url, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s = %d; want 404", url, w.Code)
		}
	}
}

func TestRepoHistory(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	seed(t, st, ghRepo("o/app", "Go", 1100, 0))
	day := func(d int) time.Time { return time.Date(2026, 10, d, 3, 30, 0, 0, time.UTC) }
	for d, v := range map[int]float64{10: 1000, 11: 1050} {
		if _, err := st.Snapshots().InsertOne(ctx, bson.M{"meta": bson.M{"source": "github", "externalId": "o/app"},
			"metrics": bson.M{"stars": v}, "capturedAt": day(d)}); err != nil {
			t.Fatal(err)
		}
	}
	// GH Archive counted 500 where GitHub says 1000: the backfill is scaled
	// up to meet the first snapshot, and its points after it are dropped.
	back := []Point{{T: day(1).UnixMilli(), V: 250}, {T: day(5).UnixMilli(), V: 500}, {T: day(11).UnixMilli(), V: 520}}
	plain := router(st)
	r := routerWith(New(st, nil).WithBackfill(func(context.Context, string) []Point { return back }))
	var h struct {
		Data []Point `json:"data"`
	}
	get(t, r, "/v1/repos/o/app/history", nil, &h)
	want := []Point{{day(1).UnixMilli(), 500}, {day(5).UnixMilli(), 1000}, {day(10).UnixMilli(), 1000}, {day(11).UnixMilli(), 1050}}
	if !reflect.DeepEqual(h.Data, want) {
		t.Fatalf("history = %+v", h.Data)
	}
	get(t, plain, "/v1/repos/o/app/history", nil, &h)
	if len(h.Data) != 2 {
		t.Fatalf("without a backfill = %+v", h.Data)
	}
}

func TestMergeCurveWithoutSnapshots(t *testing.T) {
	got := mergeCurve([]Point{{T: 1, V: 10}, {T: 2, V: 20}}, nil, 40)
	if !reflect.DeepEqual(got, []Point{{1, 20}, {2, 40}}) {
		t.Fatalf("merge = %+v; want the backfill scaled to the current count", got)
	}
	if got := mergeCurve(nil, []store.Reading{}, 5); got == nil || len(got) != 0 {
		t.Fatalf("empty = %#v; want an empty list, not null", got)
	}
}

func TestTrend(t *testing.T) {
	st := testStore(t)
	today := item.DayOf(time.Now())
	old := item.DayOf(time.Now().AddDate(0, 0, -20))
	a := ghRepo("o/a", "Go", 10, 0)
	a.History = []item.Point{{D: old, V: 1}, {D: today, V: 10}}
	seed(t, st, a, ghRepo("o/b", "Go", 5, 0))
	r := router(st)
	var tr struct {
		Data map[string][]Point `json:"data"`
	}
	get(t, r, "/v1/trend?ids=o/a,o/b,o/none&days=7", nil, &tr)
	if len(tr.Data) != 1 || len(tr.Data["o/a"]) != 1 || tr.Data["o/a"][0].V != 10 {
		t.Fatalf("7 days = %+v; only o/a has points in the window", tr.Data)
	}
	get(t, r, "/v1/trend?ids=o/a", nil, &tr)
	if len(tr.Data["o/a"]) != 2 {
		t.Fatalf("30 days = %+v", tr.Data)
	}
	for _, url := range []string{"/v1/trend", "/v1/trend?ids=o/a&days=36"} {
		if w := get(t, r, url, nil, nil); w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d; want 400", url, w.Code)
		}
	}
}
