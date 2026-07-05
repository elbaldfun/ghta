package job

import (
	"context"
	"log/slog"
	"os"
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

// fakeEmbedder maps known texts to fixed unit vectors so cosine is exact.
type fakeEmbedder struct{ byNeedle map[string][]float32 }

func (f fakeEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = []float32{0, 1} // default: orthogonal to everything relevant
		for needle, vec := range f.byNeedle {
			if contains(t, needle) {
				out[i] = vec
			}
		}
	}
	return out, nil
}

func contains(s, needle string) bool {
	return len(needle) > 0 && len(s) >= len(needle) && indexOf(s, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
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

// Tier 1: a topic-map hit classifies without any model call.
func TestPipelineRuleTier(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/ruled", "name": "vllm",
		"analysisStatus": domain.AnalysisPending,
		"sourceData":     bson.M{"topicNames": []string{"LLM"}}, // case-insensitive
	})

	fp := &fakeProvider{resp: `{"results":[]}`}
	ai := service.NewAIService(store, fp)
	embed := service.NewEmbedClassifier(nil, 0.35) // embedding tier disabled
	NewCategorizer(store, testRules, embed, ai, 15, slog.Default()).Run(ctx)

	got := findItem(t, store, "o/ruled")
	if got.AnalysisStatus != domain.AnalysisDone || got.ClassifiedBy != "rule" || got.CategoryPath != "ai/llm" {
		t.Errorf("got status=%q by=%q path=%q, want done/rule/ai-llm", got.AnalysisStatus, got.ClassifiedBy, got.CategoryPath)
	}
	if fp.calls != 0 {
		t.Errorf("LLM was called %d times for a rule-classified item", fp.calls)
	}
}

// Tier 2: embedding similarity above threshold classifies; LLM not called.
func TestPipelineEmbeddingTier(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/embedded", "name": "some-webthing",
		"description": "a fancy web framework", "analysisStatus": domain.AnalysisPending,
	})

	// category text contains "web development"; item text contains "web framework"
	emb := fakeEmbedder{byNeedle: map[string][]float32{
		"web development": {1, 0},
		"web framework":   {1, 0},
	}}
	fp := &fakeProvider{resp: `{"results":[]}`}
	ai := service.NewAIService(store, fp)
	embed := service.NewEmbedClassifier(emb, 0.35)
	NewCategorizer(store, testRules, embed, ai, 15, slog.Default()).Run(ctx)

	got := findItem(t, store, "o/embedded")
	if got.AnalysisStatus != domain.AnalysisDone || got.ClassifiedBy != "embedding" || got.CategoryPath != "web" {
		t.Errorf("got status=%q by=%q path=%q, want done/embedding/web", got.AnalysisStatus, got.ClassifiedBy, got.CategoryPath)
	}
	if fp.calls != 0 {
		t.Errorf("LLM was called %d times for an embedding-classified item", fp.calls)
	}
}

// Tier 3: unresolved items reach the LLM; isNewCategory files a suggestion (no
// category is created) and counts as a failed attempt.
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
		{"id":"o/llm-hit","categoryId":"","path":"ai/llm","isNewCategory":false},
		{"id":"o/new-cat","categoryId":"","path":"blockchain/defi","isNewCategory":true}
	]}`}
	ai := service.NewAIService(store, fp)
	embed := service.NewEmbedClassifier(nil, 0.35)
	NewCategorizer(store, testRules, embed, ai, 15, slog.Default()).Run(ctx)

	hit := findItem(t, store, "o/llm-hit")
	if hit.AnalysisStatus != domain.AnalysisDone || hit.ClassifiedBy != "llm" || hit.CategoryPath != "ai/llm" {
		t.Errorf("llm-hit = status %q by %q path %q", hit.AnalysisStatus, hit.ClassifiedBy, hit.CategoryPath)
	}

	sug := findItem(t, store, "o/new-cat")
	if sug.AnalysisStatus == domain.AnalysisDone || sug.AnalysisFailCount != 1 {
		t.Errorf("new-cat = status %q failCount %d, want not-done/1", sug.AnalysisStatus, sug.AnalysisFailCount)
	}
	// suggestion recorded, category NOT created
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

// Whole-batch parse failure escalates to failed at the threshold.
func TestCategorizerMarksFailed(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/bad",
		"analysisStatus": domain.AnalysisPending, "analysisFailCount": maxAnalysisFail - 1,
	})

	fp := &fakeProvider{resp: "garbage, no json"}
	ai := service.NewAIService(store, fp)
	embed := service.NewEmbedClassifier(nil, 0.35)
	NewCategorizer(store, testRules, embed, ai, 15, slog.Default()).Run(ctx)

	got := findItem(t, store, "o/bad")
	if got.AnalysisStatus != domain.AnalysisFailed || got.AnalysisFailCount != maxAnalysisFail {
		t.Errorf("got status %q failCount %d, want failed %d", got.AnalysisStatus, got.AnalysisFailCount, maxAnalysisFail)
	}
}
