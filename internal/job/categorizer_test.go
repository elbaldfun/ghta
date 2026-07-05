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

func (f fakeProvider) Analyze(ctx context.Context, sys, user string) (string, error) {
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

// The categorizer must pick up an item with no categoryId, create the AI-proposed
// category, and mark the item done.
func TestCategorizerAssignsNewCategory(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	// item with categoryId entirely absent (pre-existing document shape)
	if _, err := store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/r", "name": "langchain",
		"description": "LLM framework", "language": "Python",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	ai := service.NewAIService(store, fakeProvider{
		resp: `{"categoryId":"","path":"ai/llm","isNewCategory":true}`,
	})
	NewCategorizer(store, ai, slog.Default()).Run(ctx)

	var got domain.TrackedItem
	if err := store.Items().FindOne(ctx, bson.M{"externalId": "o/r"}).Decode(&got); err != nil {
		t.Fatalf("find item: %v", err)
	}
	if got.AnalysisStatus != domain.AnalysisDone {
		t.Errorf("status = %q, want done", got.AnalysisStatus)
	}
	if got.CategoryPath != "ai/llm" || len(got.CategoryID) != 1 {
		t.Errorf("categorization = path %q id %v", got.CategoryPath, got.CategoryID)
	}

	// the new category tree should have been created (root ai + child ai/llm)
	count, _ := store.Categories().CountDocuments(ctx, bson.M{})
	if count < 1 {
		t.Errorf("expected category created, got %d", count)
	}
	var child domain.Category
	if err := store.Categories().FindOne(ctx, bson.M{"path": "ai/llm"}).Decode(&child); err != nil {
		t.Errorf("child category not found: %v", err)
	} else if child.CreatedBy != "ai" || child.Level != 2 {
		t.Errorf("child = createdBy %q level %d", child.CreatedBy, child.Level)
	}
}

// A failing provider must increment the fail count and, after the threshold,
// mark the item failed so it leaves the queue.
func TestCategorizerMarksFailed(t *testing.T) {
	store := jobTestStore(t)
	ctx := context.Background()

	_, _ = store.Items().InsertOne(ctx, bson.M{
		"source": domain.SourceGitHub, "externalId": "o/bad", "categoryId": []string{},
		"analysisFailCount": maxAnalysisFail - 1,
	})

	ai := service.NewAIService(store, fakeProvider{resp: "garbage, no json"})
	NewCategorizer(store, ai, slog.Default()).Run(ctx)

	var got domain.TrackedItem
	if err := store.Items().FindOne(ctx, bson.M{"externalId": "o/bad"}).Decode(&got); err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.AnalysisStatus != domain.AnalysisFailed {
		t.Errorf("status = %q, want failed", got.AnalysisStatus)
	}
	if got.AnalysisFailCount != maxAnalysisFail {
		t.Errorf("failCount = %d, want %d", got.AnalysisFailCount, maxAnalysisFail)
	}
}
