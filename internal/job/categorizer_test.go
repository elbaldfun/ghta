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
)

type fakeProvider struct {
	resp string
	err  error
}

func (f fakeProvider) AnalyzeJSON(ctx context.Context, sys, user string) (string, error) {
	return f.resp, f.err
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
	_, _ = store.Items().DeleteMany(context.Background(), bson.M{})
	_, _ = store.Categories().DeleteMany(context.Background(), bson.M{})
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

// The categorizer batches pending items, creates the AI-proposed category, marks
// covered items done, and marks an item the model omitted as a failed attempt.
func TestCategorizerBatch(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	// two pending items; the model will only answer for o/covered
	_, _ = store.Items().InsertMany(ctx, []interface{}{
		bson.M{"source": domain.SourceGitHub, "externalId": "o/covered", "name": "langchain",
			"description": "LLM framework", "analysisStatus": domain.AnalysisPending},
		bson.M{"source": domain.SourceGitHub, "externalId": "o/omitted", "name": "mystery",
			"analysisStatus": domain.AnalysisPending},
	})

	ai := service.NewAIService(store, fakeProvider{
		resp: `{"results":[{"id":"o/covered","categoryId":"","path":"ai/llm","isNewCategory":true}]}`,
	})
	NewCategorizer(store, ai, 15, slog.Default()).Run(ctx)

	var covered domain.TrackedItem
	_ = store.Items().FindOne(ctx, bson.M{"externalId": "o/covered"}).Decode(&covered)
	if covered.AnalysisStatus != domain.AnalysisDone || covered.CategoryPath != "ai/llm" {
		t.Errorf("covered = status %q path %q, want done ai/llm", covered.AnalysisStatus, covered.CategoryPath)
	}

	var omitted domain.TrackedItem
	_ = store.Items().FindOne(ctx, bson.M{"externalId": "o/omitted"}).Decode(&omitted)
	if omitted.AnalysisFailCount != 1 || omitted.AnalysisStatus == domain.AnalysisDone {
		t.Errorf("omitted = failCount %d status %q, want 1 not-done", omitted.AnalysisFailCount, omitted.AnalysisStatus)
	}

	// the AI-created category tree exists
	if err := store.Categories().FindOne(ctx, bson.M{"path": "ai/llm"}).Err(); err != nil {
		t.Errorf("expected ai/llm category created: %v", err)
	}
}

// A whole-batch parse failure escalates an item to failed at the threshold.
func TestCategorizerMarksFailed(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/bad",
		"analysisStatus": domain.AnalysisPending, "analysisFailCount": maxAnalysisFail - 1,
	})

	ai := service.NewAIService(store, fakeProvider{resp: "garbage, no json"})
	NewCategorizer(store, ai, 15, slog.Default()).Run(ctx)

	var got domain.TrackedItem
	_ = store.Items().FindOne(ctx, bson.M{"externalId": "o/bad"}).Decode(&got)
	if got.AnalysisStatus != domain.AnalysisFailed || got.AnalysisFailCount != maxAnalysisFail {
		t.Errorf("got status %q failCount %d, want failed %d", got.AnalysisStatus, got.AnalysisFailCount, maxAnalysisFail)
	}
}
