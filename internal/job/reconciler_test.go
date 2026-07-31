package job

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
	"github.com/elbaldfun/ghta/internal/source/github"
)

// fakeRepoChecker scripts upstream answers per externalId.
type fakeRepoChecker struct {
	repos map[string]*github.RepoInfo // nil value = 404
}

func (f *fakeRepoChecker) FetchRepo(_ context.Context, fullName string) (*github.RepoInfo, github.RateInfo, error) {
	info, ok := f.repos[fullName]
	if !ok || info == nil {
		return nil, github.RateInfo{Remaining: -1}, github.ErrRepoNotFound
	}
	return info, github.RateInfo{Remaining: -1}, nil
}

func reconcilerTestStore(t *testing.T) *repository.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	cfg := &config.Config{MongoURI: uri, MongoDB: "ghta_reconciler_test"}
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

func insertAged(t *testing.T, store *repository.Store, externalID string, stars float64, age time.Duration) {
	t.Helper()
	fetched := time.Now().UTC().Add(-age)
	_, err := store.Items().InsertOne(context.Background(), bson.M{
		"source":     domain.SourceGitHub,
		"externalId": externalID,
		"name":       externalID,
		"metrics":    bson.M{"stars": stars},
		"fetchedAt":  fetched,
		"stale":      false,
	})
	if err != nil {
		t.Fatalf("insert %s: %v", externalID, err)
	}
}

func getItem(t *testing.T, store *repository.Store, externalID string) *domain.TrackedItem {
	t.Helper()
	var it domain.TrackedItem
	err := store.Items().FindOne(context.Background(),
		bson.M{"source": domain.SourceGitHub, "externalId": externalID}).Decode(&it)
	if err != nil {
		return nil
	}
	return &it
}

func TestReconcilerVerdicts(t *testing.T) {
	store := reconcilerTestStore(t)
	ctx := context.Background()
	month := 30 * 24 * time.Hour

	// Stale candidates, one per verdict:
	insertAged(t, store, "old/deleted", 5000, month)      // 404 -> tombstone "gone"
	insertAged(t, store, "facebook/react", 230000, month) // renamed, new name already tracked -> ghost merge
	insertAged(t, store, "acme/moved", 800, month)        // renamed, new name NOT tracked -> rename in place
	insertAged(t, store, "small/alive", 900, month)       // alive, fell below cutoff -> refresh metrics
	// Fresh record — must not be touched at all:
	insertAged(t, store, "react/react", 246000, time.Hour)

	fake := &fakeRepoChecker{repos: map[string]*github.RepoInfo{
		"facebook/react": {ID: 1, FullName: "react/react", Stars: 246500},
		"acme/moved":     {ID: 2, FullName: "acme/renamed", Stars: 850},
		"small/alive":    {ID: 3, FullName: "small/alive", Stars: 950},
		"react/react":    {ID: 1, FullName: "react/react", Stars: 246500},
	}}

	j := NewReconciler(store, fake, 0, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	j.Run(ctx)

	// 404 -> tombstoned
	if it := getItem(t, store, "old/deleted"); it == nil || !it.Stale || it.StaleReason != "gone" {
		t.Errorf("old/deleted: want stale/gone, got %+v", it)
	}
	// rename ghost (target exists) -> tombstoned with pointer
	if it := getItem(t, store, "facebook/react"); it == nil || !it.Stale || it.StaleReason != "renamed:react/react" {
		t.Errorf("facebook/react: want stale/renamed:react/react, got %+v", it)
	}
	// live successor untouched
	if it := getItem(t, store, "react/react"); it == nil || it.Stale {
		t.Errorf("react/react must stay live")
	}
	// rename with no existing target -> moved in place, metrics refreshed
	if it := getItem(t, store, "acme/moved"); it != nil {
		t.Errorf("acme/moved should no longer exist (renamed away)")
	}
	if it := getItem(t, store, "acme/renamed"); it == nil || it.Stale || it.Metrics["stars"] != 850 || it.Name != "renamed" {
		t.Errorf("acme/renamed: want live with stars=850 name=renamed, got %+v", it)
	}
	// same-name alive -> metrics + fetchedAt refreshed
	it := getItem(t, store, "small/alive")
	if it == nil || it.Stale || it.Metrics["stars"] != 950 {
		t.Errorf("small/alive: want live stars=950, got %+v", it)
	}
	if it != nil && time.Since(it.FetchedAt) > time.Minute {
		t.Errorf("small/alive fetchedAt not refreshed: %v", it.FetchedAt)
	}

	// Idempotence: a second run finds no work (all candidates resolved or fresh).
	j2 := NewReconciler(store, &fakeRepoChecker{repos: map[string]*github.RepoInfo{}}, 0, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	j2.Run(ctx)
	if it := getItem(t, store, "acme/renamed"); it == nil || it.Stale {
		t.Errorf("acme/renamed tombstoned by idle second run")
	}
}
