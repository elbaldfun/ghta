package job

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

type fakeProvider struct {
	resp  string
	err   error
	calls int
}

func (f *fakeProvider) AnalyzeJSON(ctx context.Context, sys, user string) (string, error) {
	f.calls++
	return f.resp, f.err
}

var testNodes = []taxonomy.Node{
	{Path: "ai", Name: "AI", Desc: "artificial intelligence"},
	{Path: "ai/llm", Name: "LLM", Desc: "large language models"},
	{Path: "web", Name: "Web", Desc: "web development frameworks"},
}

var testRules = &taxonomy.Rules{
	Topics:    map[string]string{"llm": "ai/llm"},
	Languages: map[string]string{},
}

func testFacets(t *testing.T) *taxonomy.Facets {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	f, err := taxonomy.LoadFacets(filepath.Join(filepath.Dir(file), "..", "..", "taxonomy", "facets.yaml"))
	if err != nil {
		t.Fatalf("load facets: %v", err)
	}
	return f
}

func newTestCategorizer(t *testing.T, store *repository.Store, ai *service.AIService) *Categorizer {
	return NewCategorizer(store, testRules, testFacets(t), ai, 15, 3, slog.Default())
}

func jobTestStore(t *testing.T) *repository.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	store, err := repository.Connect(context.Background(), &config.Config{MongoURI: uri, MongoDB: "ghta_job_test"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := context.Background()
	_, _ = store.Items().DeleteMany(ctx, bson.M{})
	_, _ = store.Categories().DeleteMany(ctx, bson.M{})
	_, _ = store.Suggestions().DeleteMany(ctx, bson.M{})
	if err := taxonomy.Sync(ctx, store, testNodes); err != nil {
		t.Fatalf("taxonomy sync: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func findItem(t *testing.T, store *repository.Store, externalID string) domain.TrackedItem {
	t.Helper()
	var got domain.TrackedItem
	if err := store.Items().FindOne(context.Background(), bson.M{"externalId": externalID}).Decode(&got); err != nil {
		t.Fatalf("find %s: %v", externalID, err)
	}
	return got
}

func hasPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// Tier 1: a topic-map hit classifies without any model call, and the type facet
// is derived deterministically (a cli topic -> type cli).
func TestPipelineRuleTier(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/ruled", "name": "vllm",
		"analysisStatus": domain.AnalysisPending,
		"sourceData":     bson.M{"topicNames": []string{"LLM", "cli"}},
	})

	fp := &fakeProvider{resp: `{"results":[]}`}
	newTestCategorizer(t, store, service.NewAIService(store, fp)).Run(ctx)

	got := findItem(t, store, "o/ruled")
	if got.AnalysisStatus != domain.AnalysisDone || got.ClassifiedBy != "rule" || !hasPath(got.CategoryPath, "ai/llm") {
		t.Errorf("got status=%q by=%q path=%v, want done/rule/[ai/llm]", got.AnalysisStatus, got.ClassifiedBy, got.CategoryPath)
	}
	if got.Type != "cli" {
		t.Errorf("type=%q, want cli", got.Type)
	}
	if fp.calls != 0 {
		t.Errorf("LLM was called %d times for a rule-classified item", fp.calls)
	}
}

// Tier 2: unresolved items reach the LLM, which returns multi-label domain +
// type + tags; isNewCategory files a suggestion (no category created) and counts
// as a failed attempt.
func TestPipelineLLMTierAndSuggestion(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertMany(ctx, []interface{}{
		bson.M{"source": domain.SourceGitHub, "externalId": "o/llm-hit", "name": "mystery-a",
			"analysisStatus": domain.AnalysisPending},
		bson.M{"source": domain.SourceGitHub, "externalId": "o/new-cat", "name": "defi-thing",
			"analysisStatus": domain.AnalysisPending},
	})

	fp := &fakeProvider{resp: `{"results":[
		{"id":"o/llm-hit","paths":["ai/llm","web"],"type":"library","tags":["inference","serving"],"isNewCategory":false},
		{"id":"o/new-cat","paths":["blockchain/defi"],"type":"software","isNewCategory":true}
	]}`}
	newTestCategorizer(t, store, service.NewAIService(store, fp)).Run(ctx)

	hit := findItem(t, store, "o/llm-hit")
	if hit.AnalysisStatus != domain.AnalysisDone || hit.ClassifiedBy != "llm" || !hasPath(hit.CategoryPath, "ai/llm") || !hasPath(hit.CategoryPath, "web") {
		t.Errorf("llm-hit = status %q by %q path %v", hit.AnalysisStatus, hit.ClassifiedBy, hit.CategoryPath)
	}
	if hit.Type != "library" {
		t.Errorf("llm-hit type=%q, want library", hit.Type)
	}
	if len(hit.GeneratedTopics) != 2 {
		t.Errorf("generatedTopics=%v, want 2 tags", hit.GeneratedTopics)
	}

	sug := findItem(t, store, "o/new-cat")
	if sug.AnalysisStatus == domain.AnalysisDone || sug.AnalysisFailCount != 1 {
		t.Errorf("new-cat = status %q failCount %d, want not-done/1", sug.AnalysisStatus, sug.AnalysisFailCount)
	}
	var s domain.CategorySuggestion
	if err := store.Suggestions().FindOne(ctx, bson.M{"path": "blockchain/defi"}).Decode(&s); err != nil {
		t.Fatalf("suggestion not recorded: %v", err)
	}
	if s.Count != 1 {
		t.Errorf("suggestion count = %d, want 1", s.Count)
	}
	if err := store.Categories().FindOne(ctx, bson.M{"path": "blockchain/defi"}).Err(); err == nil {
		t.Errorf("category was created but must not be")
	}
}

// A parent path returned by the LLM (e.g. "ai") is not assignable — only leaves
// are — so a non-resource item left with only a parent gets no domain.
func TestParentPathDropped(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/parenty", "name": "mystery-lib",
		"analysisStatus": domain.AnalysisPending,
	})

	// "ai" is a parent (ai/llm is its child) — must be rejected.
	fp := &fakeProvider{resp: `{"results":[{"id":"o/parenty","paths":["ai"],"type":"library"}]}`}
	newTestCategorizer(t, store, service.NewAIService(store, fp)).Run(ctx)

	got := findItem(t, store, "o/parenty")
	if len(got.CategoryPath) != 0 {
		t.Errorf("parent path was assigned: %v (want empty)", got.CategoryPath)
	}
	if got.AnalysisStatus == domain.AnalysisDone {
		t.Errorf("non-resource item with only a parent path should not be done")
	}
}

// A resource-class item (awesome by name) that the LLM leaves without a domain
// is marked done with its type, not failed.
func TestResourceClassNoFail(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/awesome-foo", "name": "awesome-foo",
		"analysisStatus": domain.AnalysisPending,
	})

	// LLM omits the item entirely.
	fp := &fakeProvider{resp: `{"results":[]}`}
	newTestCategorizer(t, store, service.NewAIService(store, fp)).Run(ctx)

	got := findItem(t, store, "o/awesome-foo")
	if got.AnalysisStatus != domain.AnalysisDone || got.Type != "awesome" || len(got.CategoryPath) != 0 {
		t.Errorf("got status=%q type=%q path=%v, want done/awesome/empty", got.AnalysisStatus, got.Type, got.CategoryPath)
	}
	if got.AnalysisFailCount != 0 {
		t.Errorf("failCount=%d, want 0 (resource class must not fail)", got.AnalysisFailCount)
	}
}

// Whole-batch parse failure escalates a non-resource item to failed at threshold.
func TestCategorizerMarksFailed(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/bad", "name": "mystery",
		"analysisStatus": domain.AnalysisPending, "analysisFailCount": maxAnalysisFail - 1,
	})

	fp := &fakeProvider{resp: "garbage, no json"}
	newTestCategorizer(t, store, service.NewAIService(store, fp)).Run(ctx)

	got := findItem(t, store, "o/bad")
	if got.AnalysisStatus != domain.AnalysisFailed || got.AnalysisFailCount != maxAnalysisFail {
		t.Errorf("got status %q failCount %d, want failed %d", got.AnalysisStatus, got.AnalysisFailCount, maxAnalysisFail)
	}
}
