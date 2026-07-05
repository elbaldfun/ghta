package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

func fptr(f float64) *float64 { return &f }

func TestIncreaseSince(t *testing.T) {
	now := time.Now().UTC()
	history := []domain.MetricSnapshot{
		{CapturedAt: now.AddDate(0, 0, -8), Metrics: map[string]float64{"stars": 1000}},
		{CapturedAt: now.AddDate(0, 0, -2), Metrics: map[string]float64{"stars": 1400}},
	}
	current := 1500.0

	if got := increaseSince(history, "stars", current, now.AddDate(0, 0, -1)); got == nil || *got != 100 {
		t.Errorf("daily = %v, want 100", got)
	}
	if got := increaseSince(history, "stars", current, now.AddDate(0, 0, -7)); got == nil || *got != 500 {
		t.Errorf("weekly = %v, want 500", got)
	}
	if got := increaseSince(history, "stars", current, now.AddDate(0, 0, -30)); got != nil {
		t.Errorf("monthly = %v, want nil (no baseline)", got)
	}
}

func metricsTestStore(t *testing.T) *repository.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	store, err := repository.Connect(context.Background(), &config.Config{MongoURI: uri, MongoDB: "ghta_metrics_test"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("schema: %v", err)
	}
	_, _ = store.Items().DeleteMany(context.Background(), bson.M{})
	_, _ = store.Snapshots().DeleteMany(context.Background(), bson.M{})
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

// End-to-end: compute growth from snapshots, then rank with /rising semantics.
func TestMetricsAndRising(t *testing.T) {
	store := metricsTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	item := domain.TrackedItem{
		Source: domain.SourceGitHub, ExternalID: "o/r", Name: "r",
		PrimaryMetric: "stars", MetricDirection: domain.DirectionDescBetter,
		Metrics: map[string]float64{"stars": 1500},
	}
	if _, err := store.Items().InsertOne(ctx, item); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	// a brand-new item with only a fresh snapshot -> weekly increase is null
	newItem := domain.TrackedItem{
		Source: domain.SourceGitHub, ExternalID: "new/one", Name: "new",
		PrimaryMetric: "stars", MetricDirection: domain.DirectionDescBetter,
		Metrics: map[string]float64{"stars": 100},
	}
	if _, err := store.Items().InsertOne(ctx, newItem); err != nil {
		t.Fatalf("insert new item: %v", err)
	}

	snaps := []interface{}{
		domain.MetricSnapshot{Meta: domain.SnapshotMeta{Source: domain.SourceGitHub, ExternalID: "o/r"}, Metrics: map[string]float64{"stars": 1000}, CapturedAt: now.AddDate(0, 0, -8)},
		domain.MetricSnapshot{Meta: domain.SnapshotMeta{Source: domain.SourceGitHub, ExternalID: "o/r"}, Metrics: map[string]float64{"stars": 1400}, CapturedAt: now.AddDate(0, 0, -2)},
		domain.MetricSnapshot{Meta: domain.SnapshotMeta{Source: domain.SourceGitHub, ExternalID: "new/one"}, Metrics: map[string]float64{"stars": 100}, CapturedAt: now.Add(-time.Hour)},
	}
	if _, err := store.Snapshots().InsertMany(ctx, snaps); err != nil {
		t.Fatalf("insert snapshots: %v", err)
	}

	if err := NewMetricsService(store, slog.Default()).Run(ctx); err != nil {
		t.Fatalf("metrics run: %v", err)
	}

	var got domain.TrackedItem
	if err := store.Items().FindOne(ctx, bson.M{"externalId": "o/r"}).Decode(&got); err != nil {
		t.Fatalf("find item: %v", err)
	}
	if got.WeeklyIncrease == nil || *got.WeeklyIncrease != 500 {
		t.Errorf("weeklyIncrease = %v, want 500", got.WeeklyIncrease)
	}
	if got.DailyIncrease == nil || *got.DailyIncrease != 100 {
		t.Errorf("dailyIncrease = %v, want 100", got.DailyIncrease)
	}
	if got.MonthlyIncrease != nil {
		t.Errorf("monthlyIncrease = %v, want nil", got.MonthlyIncrease)
	}

	// Rising weekly: o/r included, new/one excluded (null weekly increase).
	svc := NewTrendService(store)
	rising, err := svc.Rising(ctx, RisingQuery{Window: "weekly"})
	if err != nil {
		t.Fatalf("rising: %v", err)
	}
	if len(rising) != 1 || rising[0].ExternalID != "o/r" {
		ids := make([]string, len(rising))
		for i, r := range rising {
			ids[i] = r.ExternalID
		}
		t.Errorf("rising = %v, want [o/r]", ids)
	}
}
