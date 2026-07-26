package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
)

// item_content stores the one heavyweight field — the rendered README markdown —
// separately from tracked_items. The board/list/aggregation queries never touch
// it, so keeping it out of the hot collection lets far more items-per-page stay
// resident in the small WiredTiger cache. It's read only on the repo detail page.

// UpsertReadme writes (or clears) one item's README, keyed by (source,
// externalId). An empty README is stored as an empty doc rather than skipped, so
// a repo that dropped its README doesn't keep serving a stale one.
func (s *Store) UpsertReadme(ctx context.Context, source domain.Source, externalID, readme string) error {
	_, err := s.ItemContent().UpdateOne(ctx,
		bson.M{"source": source, "externalId": externalID},
		bson.M{"$set": bson.M{"readme": readme, "updatedAt": time.Now().UTC()}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert readme: %w", err)
	}
	return nil
}

// upsertReadmes bulk-writes READMEs for a fetch page. Items without a README key
// are skipped (nothing to store). Used by UpsertItems before it strips the blob
// from the items it writes.
func (s *Store) upsertReadmes(ctx context.Context, items []domain.TrackedItem) error {
	now := time.Now().UTC()
	models := make([]mongo.WriteModel, 0, len(items))
	for _, it := range items {
		readme, ok := readmeOf(it)
		if !ok {
			continue
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"source": it.Source, "externalId": it.ExternalID}).
			SetUpdate(bson.M{"$set": bson.M{"readme": readme, "updatedAt": now}}).
			SetUpsert(true))
	}
	if len(models) == 0 {
		return nil
	}
	if _, err := s.ItemContent().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
		return fmt.Errorf("bulk upsert readmes: %w", err)
	}
	return nil
}

// Readme returns the stored README for an item, or "" if none exists.
func (s *Store) Readme(ctx context.Context, source domain.Source, externalID string) (string, error) {
	var doc struct {
		Readme string `bson:"readme"`
	}
	err := s.ItemContent().FindOne(ctx,
		bson.M{"source": source, "externalId": externalID},
		options.FindOne().SetProjection(bson.M{"readme": 1}),
	).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find readme: %w", err)
	}
	return doc.Readme, nil
}

// readmeOf pulls the README string out of an item's sourceData, reporting whether
// the key was present at all (so a genuine empty README round-trips distinctly
// from "no README field").
func readmeOf(it domain.TrackedItem) (string, bool) {
	v, ok := it.SourceData["readme"]
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}
