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
