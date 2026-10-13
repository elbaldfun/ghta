package service

import (
	"context"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

func countItem(id string, lang, typ string, stars float64, stale bool, paths ...string) any {
	if paths == nil {
		paths = []string{}
	}
	return bson.M{
		"source":       domain.SourceGitHub,
		"externalId":   id,
		"name":         id,
		"language":     lang,
		"type":         typ,
		"categoryPath": paths,
		"metrics":      bson.M{"stars": stars},
		"stale":        stale,
	}
}

func seedCountCorpus(t *testing.T, store *repository.Store) {
	t.Helper()
	ctx := context.Background()
	_, _ = store.Categories().DeleteMany(ctx, bson.M{})

	ai, web := primitive.NewObjectID(), primitive.NewObjectID()
	cats := []any{
		domain.Category{ID: ai, Name: "AI", Path: "ai", Level: 1, CreatedBy: "taxonomy"},
		domain.Category{ID: web, Name: "Web", Path: "web", Level: 1, CreatedBy: "taxonomy"},
		domain.Category{Name: "Agents", Path: "ai/agents", Level: 2, ParentID: &ai, CreatedBy: "taxonomy"},
		domain.Category{Name: "LLM", Path: "ai/llm", Level: 2, ParentID: &ai, CreatedBy: "taxonomy"},
		domain.Category{Name: "Frontend", Path: "web/frontend", Level: 2, ParentID: &web, CreatedBy: "taxonomy"},
	}
	if _, err := store.Categories().InsertMany(ctx, cats); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	items := []any{
		countItem("a/one", "Go", "library", 900, false, "ai/agents"),
		countItem("a/two", "Go", "library", 800, false, "ai/agents", "ai/llm"), // two leaves, one parent
		countItem("a/three", "Go", "app", 700, false, "ai/llm", "web/frontend"),
		countItem("a/four", "Python", "app", 600, false, "web/frontend"),
		countItem("a/five", "Python", "tutorial", 500, false),         // uncategorised
		countItem("a/dead", "Go", "library", 9999, true, "ai/agents"), // tombstoned: never counted
		countItem("a/six", "Go", "library", 100, false, "web/frontend"),
	}
	if _, err := store.Items().InsertMany(ctx, items); err != nil {
		t.Fatalf("seed items: %v", err)
	}
}

func TestCategoryTreeCounts(t *testing.T) {
	store := searchTestStore(t)
	seedCountCorpus(t, store)

	tree, err := NewCategoryService(store).FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	got := map[string]int{}
	var walk func([]CategoryTree)
	walk = func(ns []CategoryTree) {
		for _, n := range ns {
			got[n.Path] = n.Count
			walk(n.Children)
		}
	}
	walk(tree)

	want := map[string]int{
		"ai/agents":    2, // one, two
		"ai/llm":       2, // two, three
		"web/frontend": 3, // three, four, six
		"ai":           3, // one, two, three — "two" sits in both leaves but counts once
		"web":          3,
	}
	for path, n := range want {
		if got[path] != n {
			t.Errorf("count[%s] = %d, want %d", path, got[path], n)
		}
	}
}

func TestTypeFacetCounts(t *testing.T) {
	store := searchTestStore(t)
	seedCountCorpus(t, store)

	order := []TypeFacet{{Key: "library"}, {Key: "app"}, {Key: "tutorial"}, {Key: "awesome"}}
	facets, err := NewCategoryService(store).TypeFacets(context.Background(), order)
	if err != nil {
		t.Fatalf("TypeFacets: %v", err)
	}
	want := []int{3, 2, 1, 0} // the tombstoned library is excluded; empty values still listed
	for i, f := range facets {
		if f.Count != want[i] {
			t.Errorf("facet %s = %d, want %d", f.Key, f.Count, want[i])
		}
	}
}

// Every hot count must be answered from an index alone. tracked_items documents
// carry README blobs, so a count that fetches documents reads megabytes per
// call; under crawler traffic that is what ran mongod out of memory in prod.
func TestHotCountsNeverFetchDocuments(t *testing.T) {
	store := searchTestStore(t)
	seedCountCorpus(t, store)
	ctx := context.Background()

	rank := func(extra bson.M) bson.M {
		f := liveFilter(bson.M{"source": domain.SourceGitHub, "metrics.stars": bson.M{"$gt": 500.0}})
		for k, v := range extra {
			f[k] = v
		}
		return f
	}
	cases := map[string]bson.M{
		"rank overall":        rank(nil),
		"rank in language":    rank(bson.M{"language": "Go"}),
		"rank in category":    rank(bson.M{"categoryPath": "ai/agents"}),
		"board total by lang": liveFilter(bson.M{"source": domain.SourceGitHub, "language": "Go"}),
		"board total by cat":  liveFilter(bson.M{"source": domain.SourceGitHub, "categoryPath": "ai/llm"}),
		"board total by type": liveFilter(bson.M{"source": domain.SourceGitHub, "type": "library"}),
		"tree leaf":           liveFilter(bson.M{"categoryPath": "ai/agents"}),
		"tree parent":         liveFilter(bson.M{"categoryPath": bson.M{"$in": []string{"ai", "ai/agents", "ai/llm"}}}),
		"type facet":          liveFilter(bson.M{"type": "library"}),
	}
	for name, filter := range cases {
		docs, keys, err := explainCount(ctx, store, filter)
		if err != nil {
			t.Fatalf("%s: explain: %v", name, err)
		}
		if keys == 0 {
			t.Errorf("%s: scanned no index keys — the case no longer exercises the index", name)
		}
		if docs != 0 {
			t.Errorf("%s: examined %d documents, want an index-only count", name, docs)
		}
	}
}

// explainCount explains the pipeline CountDocuments sends and returns how many
// documents and index keys the winning plan examined.
func explainCount(ctx context.Context, store *repository.Store, filter bson.M) (docsExamined, keysExamined int64, err error) {
	cmd := bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "aggregate", Value: repository.CollItems},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: filter}},
				{{Key: "$group", Value: bson.M{"_id": 1, "n": bson.M{"$sum": 1}}}},
			}},
			{Key: "cursor", Value: bson.M{}},
		}},
		{Key: "verbosity", Value: "executionStats"},
	}
	var out bson.M
	if err := store.DB.RunCommand(ctx, cmd).Decode(&out); err != nil {
		return 0, 0, err
	}
	stats, ok := out["executionStats"].(bson.M)
	if !ok {
		// Older planners nest the stats under the $cursor stage.
		if stages, _ := out["stages"].(bson.A); len(stages) > 0 {
			if cur, _ := stages[0].(bson.M)["$cursor"].(bson.M); cur != nil {
				stats, ok = cur["executionStats"].(bson.M)
			}
		}
	}
	if !ok {
		return 0, 0, fmt.Errorf("no executionStats in explain output")
	}
	return toInt64(stats["totalDocsExamined"]), toInt64(stats["totalKeysExamined"]), nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	}
	return 0
}
