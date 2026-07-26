package service

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
)

// legacyItem is an item still carrying an embedded README, as pre-split fetches
// wrote it. Inserted directly (not via UpsertItems, which would strip it).
func legacyItem(id, readme string) domain.TrackedItem {
	it := item(id, 100)
	it.SourceData = map[string]any{"owner": "o", "readme": readme}
	return it
}

// SplitContent moves embedded READMEs to item_content, unsets them on the item,
// and is idempotent (a second pass finds nothing to do).
func TestSplitContentMigratesAndIsIdempotent(t *testing.T) {
	svc, store := testTrendService(t)
	ctx := context.Background()
	_, _ = store.ItemContent().DeleteMany(ctx, bson.M{})
	seed(t, store, legacyItem("o/a", "readme-a"), legacyItem("o/b", ""), legacyItem("o/c", "readme-c"))

	n, err := SplitContent(ctx, store, 0)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if n != 3 {
		t.Fatalf("migrated %d, want 3", n)
	}

	// Embedded blobs are gone from every item.
	left, _ := store.Items().CountDocuments(ctx, bson.M{"sourceData.readme": bson.M{"$exists": true}})
	if left != 0 {
		t.Errorf("%d items still embed a readme after migration", left)
	}
	// Non-empty READMEs landed in content and read back through the detail path.
	if r, _ := store.Readme(ctx, domain.SourceGitHub, "o/a"); r != "readme-a" {
		t.Errorf("o/a readme = %q, want readme-a", r)
	}
	// Idempotent: nothing left to migrate.
	n2, err := SplitContent(ctx, store, 0)
	if err != nil {
		t.Fatalf("split rerun: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second pass migrated %d, want 0", n2)
	}

	// The detail read re-attaches the migrated README under sourceData (API-shape
	// preserved for the frontend).
	got, _, err := svc.Item(ctx, "github", "o/a")
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	if got.SourceData["readme"] != "readme-a" {
		t.Errorf("Item re-attach: readme = %v, want readme-a", got.SourceData["readme"])
	}
}

// Item falls back to a still-embedded README for an item not yet migrated, so the
// split is safe to deploy before the backfill finishes.
func TestItemFallsBackToEmbeddedReadme(t *testing.T) {
	_, store := testTrendService(t) // svc unused; build one that shares this store
	ctx := context.Background()
	_, _ = store.ItemContent().DeleteMany(ctx, bson.M{})
	seed(t, store, legacyItem("o/pre", "still-embedded")) // no content row written

	svc := NewTrendService(store, nil)
	got, _, err := svc.Item(ctx, "github", "o/pre")
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	if got.SourceData["readme"] != "still-embedded" {
		t.Errorf("fallback readme = %v, want still-embedded", got.SourceData["readme"])
	}
}
