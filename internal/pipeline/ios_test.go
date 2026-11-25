package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/appstore"
	"github.com/elbaldfun/ghta/internal/store"
)

// fakeAppStore serves canned charts: key "country/chart/genre" → ids.
type fakeAppStore struct {
	charts    map[string][]string
	chartErr  map[string]error
	lookupErr map[string]error // by country
	ratings   float64
	lookups   []string // "country:n"
}

func (f *fakeAppStore) Chart(_ context.Context, country, chart, genre string) ([]appstore.ChartEntry, error) {
	k := country + "/" + chart + "/" + genre
	if err := f.chartErr[k]; err != nil {
		return nil, err
	}
	ids, ok := f.charts[k]
	if !ok {
		return nil, appstore.ErrNoChart
	}
	out := make([]appstore.ChartEntry, len(ids))
	for i, id := range ids {
		out[i] = appstore.ChartEntry{ID: id, Rank: i + 1}
	}
	return out, nil
}

func (f *fakeAppStore) Lookup(_ context.Context, country string, ids []string) ([]appstore.App, error) {
	f.lookups = append(f.lookups, fmt.Sprintf("%s:%d", country, len(ids)))
	if err := f.lookupErr[country]; err != nil {
		return nil, err
	}
	var out []appstore.App
	for _, id := range ids {
		if id == "999" {
			continue // no longer sold
		}
		var n int64
		fmt.Sscan(id, &n)
		out = append(out, appstore.App{Kind: "software", TrackID: n, TrackName: "app " + id,
			PrimaryGenreID: 6007, UserRatingCount: f.ratings + float64(n)})
	}
	return out, nil
}

func iosJob(st *store.Store, f *fakeAppStore, day string) *IOS {
	j := NewIOS(st, f, []string{"us", "jp"}, nil, nil)
	j.genres = []string{appstore.GenreAll, "6007"}
	at, _ := time.Parse(time.DateOnly, day)
	j.now = func() time.Time { return at.Add(9 * time.Hour) }
	return j
}

func TestIOSChartsAndListings(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	f := &fakeAppStore{ratings: 1000, charts: map[string][]string{
		"us/free/all":  {"1", "2", "999"},
		"us/free/6007": {"2", "1"},
		"us/paid/all":  {"3"},
		"jp/free/all":  {"4", "1"}, // 1 is already homed in us
	}}
	rep, err := iosJob(st, f, "2026-11-22").Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Processed != 4 || rep.Notes["charts"] != 4 || rep.Notes["notListed"] != 1 {
		t.Fatalf("report %+v", rep)
	}
	// One lookup per home storefront.
	if strings.Join(f.lookups, ",") != "us:4,jp:1" {
		t.Errorf("lookups %v", f.lookups)
	}

	c, ok, err := st.Chart(ctx, item.SourceIOS, "us", "free", "6007", "2026-11-22")
	if err != nil || !ok || strings.Join(c.Entries, ",") != "2,1" {
		t.Fatalf("chart %+v %v %v", c, ok, err)
	}

	var one item.Item
	if err := st.Items().FindOne(ctx, bson.M{"source": item.SourceIOS, "externalId": "1"}).Decode(&one); err != nil {
		t.Fatal(err)
	}
	if one.App == nil || one.App.HomeCountry != "us" || strings.Join(one.App.Countries, ",") != "us,jp" {
		t.Errorf("app %+v", one.App)
	}
	if one.Metrics["ratings"] != 1001 || one.Metrics["bestRank"] != 1 || len(one.History) != 1 {
		t.Errorf("metrics %v history %v", one.Metrics, one.History)
	}
	if one.Jobs.Judge.Status != item.StatusDone || one.Labels.Domain[0] != "ios/productivity" {
		t.Errorf("labels %+v jobs %+v", one.Labels, one.Jobs)
	}

	// The next day app 3 left every chart: its growth ages, the others grow.
	f.ratings = 1100
	delete(f.charts, "us/paid/all")
	if _, err := iosJob(st, f, "2026-11-23").Run(ctx); err != nil {
		t.Fatal(err)
	}
	var three item.Item
	_ = st.Items().FindOne(ctx, bson.M{"source": item.SourceIOS, "externalId": "3"}).Decode(&three)
	_ = st.Items().FindOne(ctx, bson.M{"source": item.SourceIOS, "externalId": "1"}).Decode(&one)
	if one.Growth.D1 == nil || *one.Growth.D1 != 100 {
		t.Errorf("growth of a charted app %+v", one.Growth)
	}
	if three.Growth.D1 == nil || *three.Growth.D1 != 0 {
		t.Errorf("growth of an app off the charts %+v", three.Growth)
	}
}

func TestIOSFailures(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	// Nothing readable: an error, and no item written.
	f := &fakeAppStore{charts: map[string][]string{}}
	if _, err := iosJob(st, f, "2026-11-22").Run(ctx); err == nil {
		t.Fatal("a run with no chart did not fail")
	}

	// Throttled while reading charts: what was read is kept, lookups stop.
	f = &fakeAppStore{
		charts:   map[string][]string{"us/free/all": {"1"}},
		chartErr: map[string]error{"us/free/6007": fmt.Errorf("%w: http 429", appstore.ErrThrottled)},
	}
	rep, err := iosJob(st, f, "2026-11-22").Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "throttled") || len(f.lookups) != 0 || rep.Notes["charts"] != 1 {
		t.Fatalf("throttled run: %v, lookups %v, report %+v", err, f.lookups, rep)
	}

	// A failed lookup in one storefront: the other is still written, and
	// growth is not aged.
	f = &fakeAppStore{
		charts:    map[string][]string{"us/free/all": {"1"}, "jp/free/all": {"4"}},
		lookupErr: map[string]error{"us": fmt.Errorf("appstore: http 502")},
	}
	rep, err = iosJob(st, f, "2026-11-22").Run(ctx)
	if err != nil || rep.Processed != 1 || rep.Failed != 1 {
		t.Fatalf("partial lookup: %v, report %+v", err, rep)
	}
}

// An app's home storefront is fixed the first day: charted only in jp the
// next day, it is still read in us, so its history stays one country's.
func TestIOSHomeCountryIsSticky(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	f := &fakeAppStore{ratings: 1000, charts: map[string][]string{"us/free/all": {"1"}, "jp/free/all": {"1"}}}
	if _, err := iosJob(st, f, "2026-11-22").Run(ctx); err != nil {
		t.Fatal(err)
	}
	f.charts = map[string][]string{"jp/free/all": {"1"}}
	f.lookups = nil
	if _, err := iosJob(st, f, "2026-11-23").Run(ctx); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.lookups, ",") != "us:1" {
		t.Fatalf("lookups %v, want the stored home storefront", f.lookups)
	}
	var one item.Item
	_ = st.Items().FindOne(ctx, bson.M{"source": item.SourceIOS, "externalId": "1"}).Decode(&one)
	if one.App.HomeCountry != "us" || strings.Join(one.App.Countries, ",") != "jp" {
		t.Errorf("app %+v", one.App)
	}
	if _, ok := one.Metrics["bestRank"]; ok {
		t.Errorf("a best rank outside the home storefront: %v", one.Metrics)
	}
}

// A failed chart may hide an app that is still charted: growth is not aged.
func TestIOSFailedChartKeepsGrowth(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	f := &fakeAppStore{ratings: 1000, charts: map[string][]string{"us/free/all": {"1"}, "us/paid/all": {"3"}}}
	if _, err := iosJob(st, f, "2026-11-22").Run(ctx); err != nil {
		t.Fatal(err)
	}
	f.ratings = 1100
	if _, err := iosJob(st, f, "2026-11-23").Run(ctx); err != nil {
		t.Fatal(err)
	}
	// Day three: the paid chart fails, so app 3 is not seen.
	f.chartErr = map[string]error{"us/paid/all": fmt.Errorf("appstore: http 503")}
	f.ratings = 1200
	if _, err := iosJob(st, f, "2026-11-24").Run(ctx); err != nil {
		t.Fatal(err)
	}
	var three item.Item
	_ = st.Items().FindOne(ctx, bson.M{"source": item.SourceIOS, "externalId": "3"}).Decode(&three)
	if three.Growth.D1 == nil || *three.Growth.D1 != 100 {
		t.Errorf("growth of an app behind a failed chart was aged: %+v", three.Growth)
	}
}

// app.charts lists every chart an app is on today, with its rank on the
// chart's previous snapshot; an app on no chart loses it on a clean day only.
func TestIOSAppCharts(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	get := func(id string) *item.StoreApp {
		t.Helper()
		var it item.Item
		if err := st.Items().FindOne(ctx, bson.M{"source": item.SourceIOS, "externalId": id}).Decode(&it); err != nil {
			t.Fatal(err)
		}
		return it.App
	}
	f := &fakeAppStore{ratings: 1000, charts: map[string][]string{
		"us/free/all":  {"1", "2"},
		"us/free/6007": {"2"},
		"us/paid/all":  {"3"},
		"jp/free/all":  {"2"},
	}}
	if _, err := iosJob(st, f, "2026-11-22").Run(ctx); err != nil {
		t.Fatal(err)
	}
	want := []item.ChartPos{
		{Country: "us", Chart: "free", Genre: "all", Rank: 2},
		{Country: "us", Chart: "free", Genre: "6007", Rank: 1},
		{Country: "jp", Chart: "free", Genre: "all", Rank: 1},
	}
	if a := get("2"); fmt.Sprint(a.Charts) != fmt.Sprint(want) || a.ChartsDay != "2026-11-22" {
		t.Fatalf("first day charts %+v day %q", a.Charts, a.ChartsDay)
	}

	// Day two (day 23 not read at all): 2 rises, 1 falls, 3 leaves.
	f.charts = map[string][]string{
		"us/free/all":  {"2", "1"},
		"us/free/6007": {"2"},
		"jp/free/all":  {"4", "2"},
	}
	if _, err := iosJob(st, f, "2026-11-24").Run(ctx); err != nil {
		t.Fatal(err)
	}
	want = []item.ChartPos{
		{Country: "us", Chart: "free", Genre: "all", Rank: 1, Prev: 2},
		{Country: "us", Chart: "free", Genre: "6007", Rank: 1, Prev: 1},
		{Country: "jp", Chart: "free", Genre: "all", Rank: 2, Prev: 1},
	}
	if a := get("2"); fmt.Sprint(a.Charts) != fmt.Sprint(want) || a.ChartsDay != "2026-11-24" {
		t.Errorf("second day charts %+v day %q", a.Charts, a.ChartsDay)
	}
	if a := get("4"); fmt.Sprint(a.Charts) != fmt.Sprint([]item.ChartPos{{Country: "jp", Chart: "free", Genre: "all", Rank: 1}}) {
		t.Errorf("new app charts %+v", a.Charts)
	}
	if a := get("3"); len(a.Charts) != 0 || a.ChartsDay != "2026-11-22" {
		t.Errorf("app off the charts kept %+v day %q", a.Charts, a.ChartsDay)
	}

	// Day three: a chart fails, so app 1 (only on it) keeps yesterday's.
	f.chartErr = map[string]error{"us/free/all": fmt.Errorf("appstore: http 503")}
	if _, err := iosJob(st, f, "2026-11-25").Run(ctx); err != nil {
		t.Fatal(err)
	}
	if a := get("1"); len(a.Charts) != 1 || a.ChartsDay != "2026-11-24" {
		t.Errorf("app hidden by a failed chart %+v day %q", a.Charts, a.ChartsDay)
	}
}
