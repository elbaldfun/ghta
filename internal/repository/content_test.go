package repository

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
)

func itemWithReadme(id, readme string) domain.TrackedItem {
	it := sampleItem(id, 100)
	it.SourceData = map[string]any{"owner": "o", "readme": readme, "topicNames": []string{"x"}}
	return it
}

// UpsertItems must keep the README out of tracked_items and put it in item_content.
func TestUpsertItemsSplitsReadmeOut(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, _ = store.ItemContent().DeleteMany(ctx, bson.M{})

	if _, err := store.UpsertItems(ctx, []domain.TrackedItem{itemWithReadme("o/r", "# Hello\nbody")}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// tracked_items must NOT carry the blob...
	var raw bson.M
	if err := store.Items().FindOne(ctx, bson.M{"source": domain.SourceGitHub, "externalId": "o/r"}).Decode(&raw); err != nil {
		t.Fatalf("find item: %v", err)
	}
	sd, _ := raw["sourceData"].(bson.M)
	if _, present := sd["readme"]; present {
		t.Errorf("tracked_items still embeds sourceData.readme: %v", sd["readme"])
	}
	if sd["owner"] != "o" {
		t.Errorf("other sourceData fields lost: owner=%v", sd["owner"])
	}

	// ...but item_content must have it, and Readme() must read it back.
	got, err := store.Readme(ctx, domain.SourceGitHub, "o/r")
	if err != nil {
		t.Fatalf("readme: %v", err)
	}
	if got != "# Hello\nbody" {
		t.Errorf("readme = %q, want the stored markdown", got)
	}
}

// A re-fetch with a changed README updates item_content in place.
func TestUpsertItemsUpdatesReadme(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, _ = store.ItemContent().DeleteMany(ctx, bson.M{})

	_, _ = store.UpsertItems(ctx, []domain.TrackedItem{itemWithReadme("o/r", "v1")})
	_, _ = store.UpsertItems(ctx, []domain.TrackedItem{itemWithReadme("o/r", "v2")})

	got, _ := store.Readme(ctx, domain.SourceGitHub, "o/r")
	if got != "v2" {
		t.Errorf("readme = %q, want v2 (re-fetch should update)", got)
	}
	n, err := store.ItemContent().CountDocuments(ctx, bson.M{"source": domain.SourceGitHub, "externalId": "o/r"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("content docs = %d, want 1 (upsert, not insert)", n)
	}
}

// Readme() on an item with no stored content returns "" without error.
func TestReadmeMissingIsEmpty(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	_, _ = store.ItemContent().DeleteMany(ctx, bson.M{})

	got, err := store.Readme(ctx, domain.SourceGitHub, "nobody/nothing")
	if err != nil {
		t.Fatalf("readme: %v", err)
	}
	if got != "" {
		t.Errorf("readme = %q, want empty", got)
	}
}
