package service

import (
	"context"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// searchTestStore connects to MONGODB_URI (a throwaway db) or skips.
func searchTestStore(t *testing.T) *repository.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	cfg := &config.Config{MongoURI: uri, MongoDB: "ghta_search_test"}
	store, err := repository.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	_, _ = store.Items().DeleteMany(context.Background(), bson.M{})
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func searchItem(externalID, name, desc string, topics []string, stars float64) any {
	return bson.M{
		"source":      domain.SourceGitHub,
		"externalId":  externalID,
		"name":        name,
		"description": desc,
		"language":    "JavaScript",
		"metrics":     bson.M{"stars": stars, "forks": stars / 5},
		"sourceData":  bson.M{"topicNames": topics},
	}
}

// The canonical repo (exact name, huge stars, keyword absent from description)
// must outrank low-star repos that repeat the query term across every field —
// the failure mode that buried facebook/react on a "react" search.
func TestListRelevanceRanking(t *testing.T) {
	store := searchTestStore(t)
	ctx := context.Background()

	docs := []any{
		// canonical: name hit + topics hit, description doesn't mention react
		searchItem("facebook/react", "react", "The library for web and native user interfaces.", []string{"javascript", "react"}, 230000),
		// keyword stuffers: query term in name, topics and description, tiny stars
		searchItem("acme/react-basics", "react-basics", "React basics: learn react with react examples for react beginners.", []string{"react", "react-tutorial", "learn-react"}, 1600),
		searchItem("acme/namaste-react", "Namaste-React", "Namaste React course notes. React react react.", []string{"react", "course"}, 2000),
		// mid-size legit ecosystem repo
		searchItem("remix-run/react-router", "react-router", "Declarative routing for React", []string{"react", "router"}, 53000),
		// the production failure mode: a popular repo whose raw textScore beats
		// the canonical one (query term dense in every field + real stars) —
		// only the exact-name bonus keeps the canonical repo on top
		searchItem("alan2207/bulletproof-react", "bulletproof-react", "React react react: architecture for production React apps.", []string{"react", "react-applications", "react-best-practice"}, 36000),
		// unrelated control
		searchItem("vuejs/vue", "vue", "Progressive JavaScript framework", []string{"vue"}, 210000),
	}
	if _, err := store.Items().InsertMany(ctx, docs); err != nil {
		t.Fatalf("insert: %v", err)
	}

	svc := NewTrendService(store, nil)

	// Default search sort = blended relevance.
	items, total, err := svc.List(ctx, TrendQuery{Q: "react", Source: string(domain.SourceGitHub)})
	if err != nil {
		t.Fatalf("List(q=react): %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5 (vue must not match)", total)
	}
	if len(items) == 0 || items[0].ExternalID != "facebook/react" {
		got := []string{}
		for _, it := range items {
			got = append(got, it.ExternalID)
		}
		t.Errorf("relevance order = %v, want facebook/react first", got)
	}

	// sort=relevance is an accepted alias for the default.
	items2, _, err := svc.List(ctx, TrendQuery{Q: "react", Sort: "relevance:desc"})
	if err != nil {
		t.Fatalf("List(sort=relevance): %v", err)
	}
	if items2[0].ExternalID != items[0].ExternalID {
		t.Errorf("sort=relevance differs from default search order")
	}

	// An explicit concrete sort must be honored, not overridden by textScore.
	items3, _, err := svc.List(ctx, TrendQuery{Q: "react", Sort: "stars:asc"})
	if err != nil {
		t.Fatalf("List(q + sort=stars:asc): %v", err)
	}
	if items3[0].ExternalID != "acme/react-basics" {
		t.Errorf("stars:asc first = %s, want acme/react-basics (1.6k)", items3[0].ExternalID)
	}

	// relevance without a query degrades to the default sort instead of erroring.
	if _, _, err := svc.List(ctx, TrendQuery{Sort: "relevance"}); err != nil {
		t.Errorf("List(sort=relevance, no q): %v, want nil", err)
	}

	// Pagination stays stable under the aggregation path.
	p1, _, err := svc.List(ctx, TrendQuery{Q: "react", Limit: 2, Page: 1})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	p2, _, err := svc.List(ctx, TrendQuery{Q: "react", Limit: 2, Page: 2})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(p1) != 2 || len(p2) != 2 {
		t.Fatalf("page sizes = %d,%d, want 2,2", len(p1), len(p2))
	}
	seen := map[string]bool{}
	for _, it := range append(p1, p2...) {
		if seen[it.ExternalID] {
			t.Errorf("repo %s appears on both pages", it.ExternalID)
		}
		seen[it.ExternalID] = true
	}

	// A tombstoned ghost must vanish from results and the total.
	if _, err := store.Items().UpdateOne(ctx,
		bson.M{"externalId": "acme/namaste-react"},
		bson.M{"$set": bson.M{"stale": true, "staleReason": "gone"}}); err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	itemsLive, totalLive, err := svc.List(ctx, TrendQuery{Q: "react"})
	if err != nil {
		t.Fatalf("List after tombstone: %v", err)
	}
	if totalLive != 4 {
		t.Errorf("total after tombstone = %d, want 4", totalLive)
	}
	for _, it := range itemsLive {
		if it.ExternalID == "acme/namaste-react" {
			t.Errorf("stale item leaked into results")
		}
	}
}

// Ranks: overall / language / category positions count only live repos with
// strictly more stars.
func TestRanks(t *testing.T) {
	store := searchTestStore(t)
	ctx := context.Background()

	insert := func(id string, stars float64, lang string, paths []string, stale bool) {
		doc := bson.M{
			"source": domain.SourceGitHub, "externalId": id, "name": id,
			"language": lang, "metrics": bson.M{"stars": stars}, "categoryPath": paths,
		}
		if stale {
			doc["stale"] = true
		}
		if _, err := store.Items().InsertOne(ctx, doc); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	insert("a/top", 100000, "Go", []string{"infra/devops"}, false)
	insert("a/mid", 50000, "Go", []string{"infra/devops"}, false)
	insert("a/low", 10000, "Rust", []string{"infra/devops"}, false)
	insert("a/ghost", 200000, "Go", []string{"infra/devops"}, true) // must not count

	svc := NewTrendService(store, nil)
	var mid domain.TrackedItem
	if err := store.Items().FindOne(ctx, bson.M{"externalId": "a/mid"}).Decode(&mid); err != nil {
		t.Fatalf("load: %v", err)
	}
	ranks := svc.Ranks(ctx, &mid)

	want := map[string]int64{"overall:": 2, "language:Go": 2, "category:infra/devops": 2}
	if len(ranks) != len(want) {
		t.Fatalf("ranks = %+v, want %d entries", ranks, len(want))
	}
	for _, r := range ranks {
		if got := want[r.Scope+":"+r.Key]; got != r.Rank {
			t.Errorf("%s:%s rank = %d, want %d (ghost must not count)", r.Scope, r.Key, r.Rank, got)
		}
	}
}
