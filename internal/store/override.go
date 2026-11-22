package store

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// LabelOverride is a person's domain, form and shelf for one GitHub item
// (taxonomy/overrides.yaml). Domain already carries the shelf's leaf.
type LabelOverride struct {
	ExternalID string
	Type       string
	Domain     []string
	Shelf      string
}

// ApplyOverride writes a person's labels over an item's and marks them
// manual, which the judge and the migration requeues leave alone. The
// taglines, alternatives and tags the judge wrote stay. An item already
// carrying exactly these labels is not touched, so running it every night
// changes nothing. found is false when no such item is stored.
func (s *Store) ApplyOverride(ctx context.Context, o LabelOverride, version int, at time.Time) (found, changed bool, err error) {
	id := bson.M{"source": item.SourceGitHub, "externalId": o.ExternalID}
	domain := nonNil(o.Domain)
	differs := bson.A{
		bson.M{"labels.type": bson.M{"$ne": o.Type}},
		bson.M{"labels.domain": bson.M{"$ne": domain}},
		bson.M{"labels.shelf": bson.M{"$ne": o.Shelf}},
		bson.M{"labels.shelfOverride": bson.M{"$ne": o.Shelf}},
		bson.M{"labels.by": bson.M{"$ne": item.ByManual}},
		bson.M{"labels.version": bson.M{"$ne": version}},
	}
	at = at.UTC()
	res, err := s.Items().UpdateOne(ctx, bson.M{"source": id["source"], "externalId": id["externalId"], "$or": differs},
		bson.M{"$set": bson.M{
			"labels.type": o.Type, "labels.domain": domain,
			"labels.shelf": o.Shelf, "labels.shelfOverride": o.Shelf,
			"labels.by": item.ByManual, "labels.version": version, "labels.at": at,
			"updatedAt": at,
		}})
	if err != nil {
		return false, false, fmt.Errorf("apply override %s: %w", o.ExternalID, err)
	}
	if res.MatchedCount > 0 {
		return true, true, nil
	}
	n, err := s.Items().CountDocuments(ctx, id)
	if err != nil {
		return false, false, fmt.Errorf("apply override %s: %w", o.ExternalID, err)
	}
	return n > 0, false, nil
}

// DropClientLeaves takes the client-development leaves (prefix, "client/")
// off every GitHub item filed on a shelf of the apps branch: an app a person
// installs is not a library for building one. judge.Verdict.Domain applies
// the same rule to new verdicts; this brings the stored labels in line.
func (s *Store) DropClientLeaves(ctx context.Context, prefix string) (int64, error) {
	re := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(prefix)}
	res, err := s.Items().UpdateMany(ctx,
		bson.M{"source": item.SourceGitHub, "labels.domain": bson.M{"$all": bson.A{re, primitive.Regex{Pattern: "^" + regexp.QuoteMeta(taxonomy.ShelfPrefix)}}}},
		bson.M{"$pull": bson.M{"labels.domain": re}})
	if err != nil {
		return 0, fmt.Errorf("drop client leaves: %w", err)
	}
	return res.ModifiedCount, nil
}
