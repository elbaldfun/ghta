package service

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// SplitContent migrates embedded READMEs out of tracked_items into item_content,
// up to `limit` items per call (limit 0 = all). It's the backfill for the P2
// storage split: the write path already stores new fetches this way, so this only
// needs to drain the pre-split corpus. Idempotent and resumable — once an item's
// sourceData.readme is unset it no longer matches, so an operator loops this until
// it returns 0. The site keeps serving throughout: the detail read falls back to
// the embedded blob until an item is migrated.
func SplitContent(ctx context.Context, store *repository.Store, limit int) (int64, error) {
	filter := bson.M{"sourceData.readme": bson.M{"$exists": true}}
	findOpts := options.Find().SetProjection(bson.M{"source": 1, "externalId": 1, "sourceData.readme": 1})
	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}

	cur, err := store.Items().Find(ctx, filter, findOpts)
	if err != nil {
		return 0, err
	}
	var rows []struct {
		ID         primitive.ObjectID `bson:"_id"`
		Source     domain.Source      `bson:"source"`
		ExternalID string             `bson:"externalId"`
		SourceData struct {
			Readme string `bson:"readme"`
		} `bson:"sourceData"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// Persist the non-empty READMEs to item_content before unsetting them, so the
	// blob is never in flight without a home. Empty READMEs need no content doc
	// (a missing content row reads back as "").
	content := make([]mongo.WriteModel, 0, len(rows))
	ids := make([]primitive.ObjectID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		if r.SourceData.Readme == "" {
			continue
		}
		content = append(content, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"source": r.Source, "externalId": r.ExternalID}).
			SetUpdate(bson.M{"$set": bson.M{"readme": r.SourceData.Readme}}).
			SetUpsert(true))
	}
	if len(content) > 0 {
		if _, err := store.ItemContent().BulkWrite(ctx, content, options.BulkWrite().SetOrdered(false)); err != nil {
			return 0, err
		}
	}

	res, err := store.Items().UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		bson.M{"$unset": bson.M{"sourceData.readme": ""}},
	)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}
