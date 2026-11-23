package store

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/item"
)

// LabelOverride is a person's domain, form and shelf for one GitHub item
// (taxonomy/overrides.yaml). Domain already carries the shelf's leaf.
type LabelOverride struct {
	ExternalID string
	Type       string
	Domain     []string
	Shelf      string
}

// ApplyOverride writes a person's labels over an item's. The taglines,
// alternatives and tags stay as the judge wrote them.
//
// An item the judge has answered for is also marked manual, which the judge
// and the migration requeues leave alone from then on. One it has not (new,
// or failed so far) keeps its place in the judge's queue, so those other
// fields still get written; the job runs after the judge every night and
// puts the person's labels back the same night.
//
// An item already carrying exactly these labels is not touched, so running it
// every night changes nothing. found is false when no such item is stored.
func (s *Store) ApplyOverride(ctx context.Context, o LabelOverride, version int, at time.Time) (found, changed bool, err error) {
	id := bson.M{"source": item.SourceGitHub, "externalId": o.ExternalID}
	domain := nonNil(o.Domain)
	judged := bson.M{"$eq": bson.A{"$jobs.judge.status", item.StatusDone}}
	differs := bson.A{
		bson.M{"labels.type": bson.M{"$ne": o.Type}},
		bson.M{"labels.domain": bson.M{"$ne": domain}},
		bson.M{"labels.shelf": bson.M{"$ne": o.Shelf}},
		bson.M{"labels.shelfOverride": bson.M{"$ne": o.Shelf}},
		bson.M{"labels.version": bson.M{"$ne": version}},
		bson.M{"labels.by": bson.M{"$ne": item.ByManual}, "jobs.judge.status": item.StatusDone},
	}
	at = at.UTC()
	res, err := s.Items().UpdateOne(ctx,
		bson.M{"source": item.SourceGitHub, "externalId": o.ExternalID, "$or": differs},
		mongo.Pipeline{{{Key: "$set", Value: bson.M{
			"labels.type": bson.M{"$literal": o.Type}, "labels.domain": bson.M{"$literal": domain},
			"labels.shelf": bson.M{"$literal": o.Shelf}, "labels.shelfOverride": bson.M{"$literal": o.Shelf},
			"labels.by":      bson.M{"$cond": bson.A{judged, item.ByManual, "$labels.by"}},
			"labels.version": version, "labels.at": at, "updatedAt": at,
		}}}})
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
// off every GitHub item on a consumer shelf, one that is neither excluded nor
// among devShelves: an app a person installs is not a library for building
// one. judge.Verdict.Domain applies the same rule to new verdicts; this
// brings the stored labels in line.
func (s *Store) DropClientLeaves(ctx context.Context, prefix string, devShelves []string) (int64, error) {
	re := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(prefix)}
	notConsumer := bson.A{nil, "", item.ShelfExcluded}
	for _, d := range devShelves {
		notConsumer = append(notConsumer, d)
	}
	res, err := s.Items().UpdateMany(ctx,
		bson.M{"source": item.SourceGitHub, "labels.domain": re, "labels.shelf": bson.M{"$nin": notConsumer}},
		bson.M{"$pull": bson.M{"labels.domain": re}})
	if err != nil {
		return 0, fmt.Errorf("drop client leaves: %w", err)
	}
	return res.ModifiedCount, nil
}
