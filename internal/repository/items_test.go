package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
)

// testStore connects to MONGODB_URI (a throwaway db) or skips the integration test.
func testStore(t *testing.T) *Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	cfg := &config.Config{MongoURI: uri, MongoDB: "ghta_repo_test"}
	store, err := Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	// clean slate for the collections we touch
	_, _ = store.Items().DeleteMany(context.Background(), bson.M{"source": domain.SourceGitHub})
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func sampleItem(id string, stars float64) domain.TrackedItem {
	return domain.TrackedItem{
		Source:          domain.SourceGitHub,
		ExternalID:      id,
		Name:            id,
		PrimaryMetric:   "stars",
		MetricDirection: domain.DirectionDescBetter,
		Metrics:         map[string]float64{"stars": stars},
	}
}

// A re-fetch must not wipe categorization written by the categorizer job.
func TestUpsertPreservesCategorization(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if _, err := store.UpsertItems(ctx, []domain.TrackedItem{sampleItem("o/r", 100)}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// simulate the categorizer assigning a category
	if _, err := store.Items().UpdateOne(ctx,
		bson.M{"source": domain.SourceGitHub, "externalId": "o/r"},
		bson.M{"$set": bson.M{"categoryId": []string{"cat1"}, "categoryPath": []string{"ai/llm"}, "analysisStatus": domain.AnalysisDone}},
	); err != nil {
		t.Fatalf("categorize: %v", err)
	}
	// re-fetch with updated stars
	if _, err := store.UpsertItems(ctx, []domain.TrackedItem{sampleItem("o/r", 250)}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var got domain.TrackedItem
	if err := store.Items().FindOne(ctx, bson.M{"source": domain.SourceGitHub, "externalId": "o/r"}).Decode(&got); err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Metrics["stars"] != 250 {
		t.Errorf("stars = %v, want 250 (fetch should update)", got.Metrics["stars"])
	}
	if len(got.CategoryPath) != 1 || got.CategoryPath[0] != "ai/llm" || len(got.CategoryID) != 1 || got.CategoryID[0] != "cat1" {
		t.Errorf("categorization lost: path=%v id=%v", got.CategoryPath, got.CategoryID)
	}
	if got.AnalysisStatus != domain.AnalysisDone {
		t.Errorf("analysisStatus = %q, want done", got.AnalysisStatus)
	}
}

// A second fetch on the same UTC day must not create a duplicate snapshot.
func TestAppendSnapshotsDedupePerDay(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, _ = store.Snapshots().DeleteMany(ctx, bson.M{"meta.externalId": "o/snap"})

	items := []domain.TrackedItem{sampleItem("o/snap", 10)}
	n1, err := store.AppendSnapshots(ctx, items)
	if err != nil {
		t.Fatalf("first append: %v", err)
	}
	n2, err := store.AppendSnapshots(ctx, items)
	if err != nil {
		t.Fatalf("second append: %v", err)
	}
	if n1 != 1 || n2 != 0 {
		t.Errorf("appended n1=%d n2=%d, want 1 then 0", n1, n2)
	}

	count, err := store.Snapshots().CountDocuments(ctx, bson.M{
		"meta.externalId": "o/snap",
		"capturedAt":      bson.M{"$gte": startOfUTCDay(time.Now())},
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("today snapshot count = %d, want 1", count)
	}
}
