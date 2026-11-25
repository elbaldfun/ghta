package api

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func iosApp(id, name, genreID string, ratings float64, fetched time.Time) item.Item {
	return item.Item{
		Source: item.SourceIOS, ExternalID: id, Name: name, Description: "cut", Topics: []string{},
		PrimaryMetric: "ratings", Metrics: map[string]float64{"ratings": ratings, "rating": 4.5, "price": 0},
		Labels: item.Labels{Domain: []string{"ios/productivity"}, By: item.BySource},
		App: &item.StoreApp{Developer: "Dev " + name, GenreID: genreID, GenreIDs: []string{genreID, "6000"},
			Currency: "USD", HomeCountry: "us", Countries: []string{"us"}, Screenshots: []string{"s1"}},
		History: []item.Point{}, FetchedAt: fetched,
	}
}

func TestIOSChart(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	gone := iosApp("4", "Gone", "6007", 1, now)
	gone.Stale = true
	seed(t, st, iosApp("1", "One", "6007", 900, now), iosApp("2", "Two", "6007", 50, now), iosApp("3", "Three", "6007", 0, now), gone)
	for _, c := range []store.ChartSnapshot{
		{Source: item.SourceIOS, Country: "us", Chart: "free", Genre: "6007", Day: "2026-11-24", FetchedAt: now.Add(-24 * time.Hour), Entries: []string{"2", "1", "4"}},
		// "9" was never looked up; "4" is gone: both leave a gap in the ranks.
		{Source: item.SourceIOS, Country: "us", Chart: "free", Genre: "6007", Day: "2026-11-25", FetchedAt: now, Entries: []string{"1", "9", "2", "3", "4"}},
		{Source: item.SourceIOS, Country: "us", Chart: "free", Genre: "all", Day: "2026-11-25", FetchedAt: now, Entries: []string{"3"}},
	} {
		if err := st.WriteChart(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	r := router(st)

	var p struct{ Data ChartPage }
	if w := get(t, r, "/v1/ios/charts/us/free?genre=productivity", nil, &p); w.Code != http.StatusOK {
		t.Fatalf("chart = %d %s", w.Code, w.Body)
	}
	up1, down2 := 1, -2
	type row struct {
		Rank   int
		Change *int
		New    bool
		ID     string
	}
	var got []row
	for _, e := range p.Data.Entries {
		got = append(got, row{e.Rank, e.Change, e.New, e.App.ID})
	}
	want := []row{{1, &up1, false, "1"}, {3, &down2, false, "2"}, {4, nil, true, "3"}}
	if !reflect.DeepEqual(got, want) || p.Data.Day != "2026-11-25" || p.Data.PrevDay != "2026-11-24" || p.Data.Genre != "productivity" {
		t.Fatalf("chart = %+v %+v", p.Data, got)
	}
	if a := p.Data.Entries[0].App; a.Name != "One" || a.Developer != "Dev One" || a.Genre != "productivity" || a.Ratings != 900 || a.HomeCountry != "us" {
		t.Fatalf("card = %+v", a)
	}

	// The first snapshot of a chart has no baseline: no moves at all.
	var all struct{ Data ChartPage }
	get(t, r, "/v1/ios/charts/us/free", nil, &all)
	if len(all.Data.Entries) != 1 || all.Data.Entries[0].New || all.Data.Entries[0].Change != nil || all.Data.PrevDay != "" {
		t.Fatalf("first snapshot = %+v", all.Data)
	}
	for url, code := range map[string]int{
		"/v1/ios/charts/fr/free":                 http.StatusBadRequest,
		"/v1/ios/charts/us/top":                  http.StatusBadRequest,
		"/v1/ios/charts/us/free?genre=nope":      http.StatusBadRequest,
		"/v1/ios/charts/jp/paid?genre=games":     http.StatusNotFound,
		"/v1/ios/charts/us/grossing?genre=all":   http.StatusNotFound,
		"/v1/ios/charts/us/free?genre=education": http.StatusNotFound,
	} {
		if w := get(t, r, url, nil, nil); w.Code != code {
			t.Errorf("%s = %d; want %d", url, w.Code, code)
		}
	}
}

func TestIOSApp(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	old := iosApp("2", "Old", "6007", 10, now.Add(-10*24*time.Hour))
	seed(t, st, iosApp("1", "One", "6007", 900, now), old)
	if _, err := st.Blobs().InsertOne(ctx, item.Blob{Source: item.SourceIOS, ExternalID: "1", Readme: "  The whole text.  "}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteChart(ctx, store.ChartSnapshot{Source: item.SourceIOS, Country: "us", Chart: "free", Genre: "all",
		Day: "2026-11-25", FetchedAt: now, Entries: []string{"1"}}); err != nil {
		t.Fatal(err)
	}
	charts := bson.A{
		bson.M{"country": "us", "chart": "free", "genre": "all", "rank": 3, "prev": 5},
		bson.M{"country": "us", "chart": "free", "genre": "6007", "rank": 1},
		bson.M{"country": "jp", "chart": "paid", "genre": "6999", "rank": 2}, // outside the charted genres
	}
	for id, day := range map[string]string{"1": "2026-11-25", "2": "2026-11-15"} {
		if _, err := st.Items().UpdateOne(ctx, bson.M{"source": item.SourceIOS, "externalId": id},
			bson.M{"$set": bson.M{"app.charts": charts, "app.chartsDay": day}}); err != nil {
			t.Fatal(err)
		}
	}
	r := router(st)

	var d struct{ Data IOSDetail }
	if w := get(t, r, "/v1/ios/apps/1", nil, &d); w.Code != http.StatusOK {
		t.Fatalf("app = %d %s", w.Code, w.Body)
	}
	up2 := 2
	want := []ChartPlace{{"us", "free", "all", 3, &up2, false}, {"us", "free", "productivity", 1, nil, true}}
	if !reflect.DeepEqual(d.Data.Charts, want) || d.Data.ChartsDay != "2026-11-25" {
		t.Fatalf("charts = %+v", d.Data.Charts)
	}
	if d.Data.Description != "The whole text." || !d.Data.Listed || !reflect.DeepEqual(d.Data.Genres, []string{"productivity", "business"}) ||
		d.Data.Screenshots[0] != "s1" || d.Data.Name != "One" {
		t.Fatalf("app = %+v", d.Data)
	}

	// Off every chart for days: its stored positions are stale, and its
	// page is not worth indexing.
	var o struct{ Data IOSDetail }
	get(t, r, "/v1/ios/apps/2", nil, &o)
	if len(o.Data.Charts) != 0 || o.Data.Listed || o.Data.Description != "cut" {
		t.Fatalf("stale app = %+v", o.Data)
	}
	for _, url := range []string{"/v1/ios/apps/9", "/v1/ios/apps/abc", "/v1/ios/apps/9/history"} {
		if w := get(t, r, url, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s = %d; want 404", url, w.Code)
		}
	}

	for i, m := range []bson.M{{"ratings": 880.0, "bestRank": 4.0}, {"ratings": 900.0}} {
		if _, err := st.Snapshots().InsertOne(ctx, bson.M{"capturedAt": now.Add(time.Duration(i-2) * 24 * time.Hour),
			"meta": bson.M{"source": item.SourceIOS, "externalId": "1"}, "metrics": m}); err != nil {
			t.Fatal(err)
		}
	}
	var h struct{ Data IOSHistory }
	get(t, r, "/v1/ios/apps/1/history", nil, &h)
	if len(h.Data.Ratings) != 2 || h.Data.Ratings[1].V != 900 || len(h.Data.BestRank) != 1 || h.Data.BestRank[0].V != 4 {
		t.Fatalf("history = %+v", h.Data)
	}

	var g struct {
		Data struct {
			Countries []string
			Charts    []string
			Genres    []IOSGenre
		}
	}
	get(t, r, "/v1/ios/genres", nil, &g)
	if len(g.Data.Genres) != 25 || len(g.Data.Countries) != 5 || len(g.Data.Charts) != 3 {
		t.Fatalf("genres = %+v", g.Data)
	}
	for _, x := range g.Data.Genres {
		if x.Slug == "productivity" && x.Count != 2 || x.Slug == "games" && x.Count != 0 {
			t.Errorf("genre %+v", x)
		}
	}
}

func TestIOSNew(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	gone := iosApp("3", "Gone", "6007", 1, now)
	gone.Stale = true
	seed(t, st, iosApp("1", "One", "6007", 900, now), iosApp("2", "Two", "6015", 50, now), gone)
	released := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	rows := []boards.IOSNewRow{
		{ID: "2", ReleasedAt: released, Best: item.ChartPos{Country: "us", Chart: "paid", Genre: "6015", Rank: 1}, Charts: 3},
		{ID: "3", ReleasedAt: released, Best: item.ChartPos{Country: "us", Chart: "free", Genre: "all", Rank: 2}, Charts: 1},
		{ID: "1", ReleasedAt: released, Best: item.ChartPos{Country: "us", Chart: "free", Genre: "all", Rank: 4}, Charts: 2},
	}
	// An older day's board is not the one served.
	if err := st.PutBoard(ctx, boards.IOSNewKey("us", "2026-11-24"), rows[:1], now); err != nil {
		t.Fatal(err)
	}
	if err := st.PutBoard(ctx, boards.IOSNewKey("us", "2026-11-25"), rows, now); err != nil {
		t.Fatal(err)
	}
	r := router(st)

	var p struct{ Data NewPage }
	if w := get(t, r, "/v1/ios/new/us", nil, &p); w.Code != http.StatusOK {
		t.Fatalf("new = %d %s", w.Code, w.Body)
	}
	d := p.Data
	if !d.Synthetic || d.Sort != "best" || d.WindowDays != 90 || d.Day != "2026-11-25" || d.Total != 3 || len(d.Entries) != 2 {
		t.Fatalf("page = %+v", d)
	}
	// The gone app leaves no gap: ranks are places in this view.
	e := d.Entries[0]
	if e.Rank != 1 || e.App.ID != "2" || e.App.Name != "Two" || e.Best != (ChartPlace{Country: "us", Chart: "paid", Genre: "finance", Rank: 1}) || e.Charts != 3 || !e.ReleasedAt.Equal(released) {
		t.Errorf("entry 0 = %+v", e)
	}
	if e := d.Entries[1]; e.Rank != 2 || e.App.ID != "1" || e.Best.Genre != "all" || e.Best.Rank != 4 {
		t.Errorf("entry 1 = %+v", e)
	}
	var one struct{ Data NewPage }
	get(t, r, "/v1/ios/new/us?limit=1", nil, &one)
	if len(one.Data.Entries) != 1 || one.Data.Total != 3 {
		t.Errorf("limit 1 = %+v", one.Data)
	}
	for url, code := range map[string]int{
		"/v1/ios/new/fr":           http.StatusBadRequest,
		"/v1/ios/new/us?limit=0":   http.StatusBadRequest,
		"/v1/ios/new/us?limit=501": http.StatusBadRequest,
		"/v1/ios/new/jp":           http.StatusNotFound,
	} {
		if w := get(t, r, url, nil, nil); w.Code != code {
			t.Errorf("%s = %d, want %d", url, w.Code, code)
		}
	}
}
