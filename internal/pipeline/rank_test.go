package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("ghta_pipeline_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Drop(ctx); _ = client.Disconnect(ctx) })
	st := store.New(db)
	if err := st.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}

func gh(id, lang string, stars float64, domain ...string) item.Item {
	return item.Item{
		Source: item.SourceGitHub, ExternalID: id, Language: lang, Topics: []string{},
		PrimaryMetric: "stars", MetricDirection: "desc-better", Metrics: map[string]float64{"stars": stars},
		History: []item.Point{}, Labels: item.Labels{Domain: domain},
	}
}

func ranksOf(t *testing.T, st *store.Store) map[string]*item.Ranks {
	t.Helper()
	cur, err := st.Items().Find(context.Background(), bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	var items []item.Item
	if err := cur.All(context.Background(), &items); err != nil {
		t.Fatal(err)
	}
	out := map[string]*item.Ranks{}
	for _, it := range items {
		out[it.ExternalID] = it.Ranks
	}
	return out
}

func TestRank(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	stale := gh("o/gone", "Go", 9999, "web/frontend")
	stale.Stale = true
	stale.Ranks = &item.Ranks{Overall: 1}
	noMetric := gh("o/empty", "Go", 0)
	noMetric.Metrics = map[string]float64{}
	model := item.Item{
		Source: item.SourceHuggingFace, ExternalID: "a/model", Topics: []string{}, History: []item.Point{},
		PrimaryMetric: "downloads", MetricDirection: "desc-better", Metrics: map[string]float64{"downloads": 5},
		Labels: item.Labels{Domain: []string{"hf/text-generation"}},
	}
	docs := []any{
		gh("o/a", "Go", 300, "web/frontend", "ai/agents"),
		gh("o/b", "Rust", 500, "ai/agents"),
		gh("o/c", "Go", 100, "web/frontend", "web/frontend"), // a repeated path counts once
		gh("o/d", "", 300),                                   // ties with o/a; later by id
		stale, noMetric, model,
	}
	if _, err := st.Items().InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}

	job := NewRank(st)
	t1 := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)
	job.now = func() time.Time { return t1 }
	rep, err := job.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Processed != 5 || rep.Notes["written"] != 5 || rep.Notes["cleared"] != 1 {
		t.Fatalf("report = %+v", rep)
	}

	got := ranksOf(t, st)
	want := map[string]item.Ranks{
		"o/b":     {Overall: 1, Language: 1, Domain: map[string]int{"ai/agents": 1}},
		"o/a":     {Overall: 2, Language: 1, Domain: map[string]int{"web/frontend": 1, "ai/agents": 2}},
		"o/d":     {Overall: 3},
		"o/c":     {Overall: 4, Language: 2, Domain: map[string]int{"web/frontend": 2}},
		"a/model": {Overall: 1, Domain: map[string]int{"hf/text-generation": 1}}, // ranked within its own source
	}
	for id, w := range want {
		g := got[id]
		if g == nil || !sameRanks(g, w) || !g.At.Equal(t1) {
			t.Errorf("%s ranks = %+v, want %+v at %v", id, g, w, t1)
		}
	}
	if got["o/gone"] != nil || got["o/empty"] != nil {
		t.Errorf("stale or metric-less items keep ranks: %+v %+v", got["o/gone"], got["o/empty"])
	}

	// The next day every ranked item takes its first positions as the
	// baseline; a rerun the same day with nothing moved writes nothing.
	t2 := t1.Add(24 * time.Hour)
	job.now = func() time.Time { return t2 }
	if rep, err = job.Run(ctx); err != nil || rep.Notes["written"] != 5 || rep.Notes["cleared"] != 0 {
		t.Fatalf("next day = %+v, %v; want every baseline written", rep, err)
	}
	if rep, err = job.Run(ctx); err != nil || rep.Notes["written"] != 0 {
		t.Fatalf("idle rerun = %+v, %v", rep, err)
	}
	// One item overtaking another rewrites only the items whose positions
	// changed, against the same baseline.
	if _, err := st.Items().UpdateOne(ctx, bson.M{"externalId": "o/c"}, bson.M{"$set": bson.M{"metrics.stars": 400}}); err != nil {
		t.Fatal(err)
	}
	if rep, err = job.Run(ctx); err != nil || rep.Notes["written"] != 3 {
		t.Fatalf("after o/c overtakes = %+v, %v; want a, c, d rewritten", rep, err)
	}
	got = ranksOf(t, st)
	if c := got["o/c"]; c.Overall != 2 || c.Language != 1 || c.Prev == nil || c.Prev.Overall != 4 || c.Prev.Language != 2 || !c.At.Equal(t2) {
		t.Errorf("after overtaking: c=%+v prev=%+v", c, c.Prev)
	}
	if a := got["o/a"]; a.Domain["web/frontend"] != 2 || a.Prev.Domain["web/frontend"] != 1 {
		t.Errorf("after overtaking: a=%+v prev=%+v", a, a.Prev)
	}
	if b := got["o/b"]; !b.At.Equal(t1) || b.Prev.Overall != 1 {
		t.Errorf("o/b did not move: at %v prev %+v", b.At, b.Prev)
	}

	// The day after, the moves become the baseline: only the movers rewrite.
	job.now = func() time.Time { return t2.Add(24 * time.Hour) }
	if rep, err = job.Run(ctx); err != nil || rep.Notes["written"] != 3 {
		t.Fatalf("third day = %+v, %v; want a, c, d rebased", rep, err)
	}
	if c := ranksOf(t, st)["o/c"]; c.Prev.Overall != 2 || !c.At.Equal(t2) {
		t.Errorf("third day c=%+v prev=%+v", c, c.Prev)
	}
}
