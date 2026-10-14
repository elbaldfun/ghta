package store

import (
	"context"
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// The methods here are the steps of a taxonomy migration (taxonomy/migrations.yaml)
// on the rebuilt collections. Each is keyed on the old value and on the item
// still being behind the target tree version, so running one again changes
// nothing. Before a label set is first rewritten its domain is kept in
// labels.prev.
//
// No step stamps the new version: an item can be hit by several (a renamed
// leaf and a merged shelf), and the first must not hide it from the next.
// StampVersion closes the migration once every step has run.

// behind matches items labelled on a tree older than version, including ones
// from before versions were recorded.
func behind(version int) bson.M { return bson.M{"$not": bson.M{"$gte": version}} }

// keepPrev records labels.domain in labels.prev unless a snapshot is already there.
func keepPrev() bson.D {
	return bson.D{{Key: "$set", Value: bson.M{"labels.prev": bson.M{"$cond": bson.A{
		bson.M{"$gt": bson.A{bson.M{"$size": bson.M{"$ifNull": bson.A{"$labels.prev", bson.A{}}}}, 0}},
		"$labels.prev", "$labels.domain",
	}}}}}
}

// swapLeaf is labels.domain with one leaf replaced by another, order kept and
// duplicates dropped (a merge can land the same leaf twice). add, when true,
// puts the new leaf in even where the old one was absent.
func swapLeaf(old, new string, add bool) bson.M {
	mapped := bson.M{"$map": bson.M{
		"input": "$labels.domain",
		"in":    bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$$this", old}}, new, "$$this"}},
	}}
	var input any = mapped
	if add {
		input = bson.M{"$concatArrays": bson.A{mapped, bson.A{new}}}
	}
	return bson.M{"$reduce": bson.M{
		"input": input, "initialValue": bson.A{},
		"in": bson.M{"$cond": bson.A{
			bson.M{"$in": bson.A{"$$this", "$$value"}}, "$$value",
			bson.M{"$concatArrays": bson.A{"$$value", bson.A{"$$this"}}},
		}},
	}}
}

// RenameLeaf replaces a domain leaf that was renamed or merged into another.
func (s *Store) RenameLeaf(ctx context.Context, old, new string, version int) (int64, error) {
	res, err := s.Items().UpdateMany(ctx,
		bson.M{"labels.domain": old, "labels.version": behind(version)},
		mongo.Pipeline{keepPrev(), {{Key: "$set", Value: bson.M{"labels.domain": swapLeaf(old, new, false)}}}})
	if err != nil {
		return 0, fmt.Errorf("rename leaf %s: %w", old, err)
	}
	return res.ModifiedCount, nil
}

// requeue sends judged items matching filter back to the judge. They keep
// their labels, and their old version, until it has answered, so the site
// goes on serving them where they were.
func (s *Store) requeue(ctx context.Context, filter bson.M, version int) (int64, error) {
	filter["labels.version"] = behind(version)
	filter["jobs.judge.status"] = item.StatusDone
	filter["labels.by"] = bson.M{"$ne": item.ByManual}
	res, err := s.Items().UpdateMany(ctx, filter, mongo.Pipeline{keepPrev(), {{Key: "$set", Value: bson.M{
		"jobs.judge.status": item.StatusPending, "jobs.judge.fail": 0,
	}}}})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// RequeueLeaf sends the items under a domain leaf that was split back to the judge.
func (s *Store) RequeueLeaf(ctx context.Context, old string, version int) (int64, error) {
	n, err := s.requeue(ctx, bson.M{"labels.domain": old}, version)
	if err != nil {
		return 0, fmt.Errorf("requeue leaf %s: %w", old, err)
	}
	return n, nil
}

// RequeueShelf sends the items on a shelf that needs a fresh verdict back to
// the judge. An item a person put there is left alone.
func (s *Store) RequeueShelf(ctx context.Context, old string, version int) (int64, error) {
	n, err := s.requeue(ctx, bson.M{"labels.shelf": old, "labels.shelfOverride": bson.M{"$in": bson.A{nil, ""}}}, version)
	if err != nil {
		return 0, fmt.Errorf("requeue shelf %s: %w", old, err)
	}
	return n, nil
}

// RenameShelf moves a shelf slug, the judged one and a person's override
// alike, and the shelf's leaf in the domain with it. oldLeaf and newLeaf are
// the taxonomy leaves of the two slugs.
func (s *Store) RenameShelf(ctx context.Context, old, new, oldLeaf, newLeaf string, version int) (int64, error) {
	var total int64
	for _, field := range []string{"labels.shelf", "labels.shelfOverride"} {
		res, err := s.Items().UpdateMany(ctx,
			bson.M{field: old, "labels.version": behind(version)},
			mongo.Pipeline{keepPrev(), {{Key: "$set", Value: bson.M{field: new, "labels.domain": swapLeaf(oldLeaf, newLeaf, true)}}}})
		if err != nil {
			return total, fmt.Errorf("rename shelf %s: %w", old, err)
		}
		total += res.ModifiedCount
	}
	return total, nil
}

// RetireAppsLeaf removes a domain leaf of the apps branch that no longer
// exists. An item on a shelf gets that shelf's leaf (shelfLeaf maps a slug to
// it) in its place; one without a shelf gets fallback's when fallback is set.
// An item left with no domain at all goes back to the judge.
func (s *Store) RetireAppsLeaf(ctx context.Context, old, fallback string, shelfLeaf func(slug string) string, version int) (relabelled, requeued int64, err error) {
	cur, err := s.Items().Find(ctx, bson.M{"labels.domain": old, "labels.version": behind(version)},
		options.Find().SetProjection(bson.M{"labels.domain": 1, "labels.prev": 1, "labels.shelf": 1}))
	if err != nil {
		return 0, 0, fmt.Errorf("retire %s: %w", old, err)
	}
	defer cur.Close(ctx)
	var writes []mongo.WriteModel
	flush := func() error {
		if len(writes) == 0 {
			return nil
		}
		_, err := s.Items().BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
		writes = writes[:0]
		return err
	}
	for cur.Next(ctx) {
		var it struct {
			ID     primitive.ObjectID `bson:"_id"`
			Labels struct {
				Domain []string `bson:"domain"`
				Prev   []string `bson:"prev"`
				Shelf  string   `bson:"shelf"`
			} `bson:"labels"`
		}
		if err := cur.Decode(&it); err != nil {
			return relabelled, requeued, fmt.Errorf("retire %s: %w", old, err)
		}
		shelf := it.Labels.Shelf
		if shelf == item.ShelfExcluded {
			shelf = ""
		} else if shelf == "" {
			shelf = fallback
		}
		next := slices.DeleteFunc(slices.Clone(it.Labels.Domain), func(p string) bool { return p == old })
		if shelf != "" {
			if leaf := shelfLeaf(shelf); !slices.Contains(next, leaf) {
				next = append(next, leaf)
			}
		}
		set := bson.M{"labels.domain": next}
		if len(it.Labels.Prev) == 0 {
			set["labels.prev"] = it.Labels.Domain
		}
		if len(next) == 0 {
			set["jobs.judge.status"], set["jobs.judge.fail"] = item.StatusPending, 0
			requeued++
		} else {
			relabelled++
		}
		writes = append(writes, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": it.ID}).SetUpdate(bson.M{"$set": set}))
		if len(writes) >= 500 {
			if err := flush(); err != nil {
				return relabelled, requeued, fmt.Errorf("retire %s: %w", old, err)
			}
		}
	}
	if err := cur.Err(); err != nil {
		return relabelled, requeued, fmt.Errorf("retire %s: %w", old, err)
	}
	if err := flush(); err != nil {
		return relabelled, requeued, fmt.Errorf("retire %s: %w", old, err)
	}
	return relabelled, requeued, nil
}

// StampVersion closes a migration: every judged item still behind version is
// marked current. Its labels were rewritten mechanically or did not need to
// change. Items sent back to the judge are pending and stay behind until it
// has answered. Items labelled by their source are outside the tree and carry
// no version.
func (s *Store) StampVersion(ctx context.Context, version int) (int64, error) {
	res, err := s.Items().UpdateMany(ctx,
		bson.M{"jobs.judge.status": item.StatusDone, "labels.by": bson.M{"$ne": item.BySource}, "labels.version": behind(version)},
		bson.M{"$set": bson.M{"labels.version": version}})
	if err != nil {
		return 0, fmt.Errorf("stamp version: %w", err)
	}
	return res.ModifiedCount, nil
}
