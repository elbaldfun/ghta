package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// The app store's rows (boards.App), one per app, rewritten nightly by the
// boards job. Small enough that any filter the store page offers is a cheap
// query, which is why it is a collection and not a board per filter.
const CollApps = "apps"

// appSorts maps the store's sorts to row fields.
var appSorts = map[string]string{"hot": "growth", "popular": "stars", "new": "latestReleaseAt"}

const appsBatch = 1000

// AppsDepth is how far into any filter the store pages, as the old store did:
// a shopper does not page past 300 apps, and a crawler need not.
const AppsDepth = 300

func (s *Store) Apps() *mongo.Collection { return s.DB.Collection(CollApps) }

func (s *Store) ensureAppIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "alternatives.slug", Value: 1}, {Key: "stars", Value: -1}}},
		{Keys: bson.D{{Key: "at", Value: 1}}},
	}
	// A page with no filter walks a sort index; a narrow filter is found by
	// its own index and sorted in memory, which beats walking every row.
	for _, key := range appSorts {
		models = append(models, mongo.IndexModel{Keys: bson.D{{Key: "listed", Value: 1}, {Key: key, Value: -1}, {Key: "_id", Value: 1}}})
	}
	for _, field := range []string{"platforms", "kind", "shelf", "group", "domains"} {
		models = append(models, mongo.IndexModel{Keys: bson.D{{Key: "listed", Value: 1}, {Key: field, Value: 1}}})
	}
	if _, err := s.Apps().Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("apps indexes: %w", err)
	}
	return nil
}

// AppRow is a row of the apps collection; Key is its _id.
type AppRow interface{ Key() string }

// PutApps replaces the store's rows with rows, each carrying at as its "at":
// rows are upserted, then rows an earlier run wrote and this one did not are
// removed. A reader during the write sees some of each night's rows.
func (s *Store) PutApps(ctx context.Context, rows []AppRow, at time.Time) (removed int64, err error) {
	for start := 0; start < len(rows); start += appsBatch {
		batch := rows[start:min(start+appsBatch, len(rows))]
		models := make([]mongo.WriteModel, len(batch))
		for i, r := range batch {
			models[i] = mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": r.Key()}).SetReplacement(r).SetUpsert(true)
		}
		if _, err := s.Apps().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
			return 0, fmt.Errorf("put apps: %w", err)
		}
	}
	res, err := s.Apps().DeleteMany(ctx, bson.M{"at": bson.M{"$lt": at.UTC()}})
	if err != nil {
		return 0, fmt.Errorf("put apps: %w", err)
	}
	return res.DeletedCount, nil
}

// AppQuery is one page of the store. Empty fields do not filter. Shelf is a
// shelf slug, or a group: one segment, or "dev".
type AppQuery struct {
	OS, Kind, Domain, Shelf string
	Sort                    string // hot | popular | new
	Skip, Limit             int
}

// ListApps decodes one page of listed apps into rows and returns how many
// apps match, counting no further than AppsDepth.
func (s *Store) ListApps(ctx context.Context, q AppQuery, rows any) (int, error) {
	f := bson.M{"listed": true}
	if q.OS != "" {
		f["platforms"] = q.OS
	}
	if q.Kind != "" {
		f["kind"] = q.Kind
	}
	if q.Domain != "" {
		f["domains"] = q.Domain
	}
	if q.Shelf != "" {
		if strings.Contains(q.Shelf, "/") {
			f["shelf"] = q.Shelf
		} else {
			f["group"] = q.Shelf
		}
	}
	key, ok := appSorts[q.Sort]
	if !ok {
		return 0, fmt.Errorf("list apps: unknown sort %q", q.Sort)
	}
	total, err := s.Apps().CountDocuments(ctx, f, options.Count().SetLimit(AppsDepth))
	if err != nil {
		return 0, fmt.Errorf("list apps: %w", err)
	}
	limit := min(q.Limit, AppsDepth-q.Skip)
	if limit <= 0 {
		return int(total), nil
	}
	// The order is decided on ids alone, then only the page is read whole.
	cur, err := s.Apps().Find(ctx, f, options.Find().
		SetSort(bson.D{{Key: key, Value: -1}, {Key: "_id", Value: 1}}).
		SetLimit(AppsDepth).SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return 0, fmt.Errorf("list apps: %w", err)
	}
	var ids []struct {
		ID string `bson:"_id"`
	}
	if err := cur.All(ctx, &ids); err != nil {
		return 0, fmt.Errorf("list apps: %w", err)
	}
	order := make([]string, len(ids))
	for i, r := range ids {
		order[i] = r.ID
	}
	order = SpreadOwners(order, q.Limit, ownerPerPage)
	page := order[min(q.Skip, len(order)):min(q.Skip+limit, len(order))]
	cur, err = s.Apps().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"_id": bson.M{"$in": page}}}},
		{{Key: "$addFields", Value: bson.M{"_at": bson.M{"$indexOfArray": bson.A{page, "$_id"}}}}},
		{{Key: "$sort", Value: bson.M{"_at": 1}}},
		{{Key: "$project", Value: bson.M{"_at": 0}}},
	})
	if err != nil {
		return 0, fmt.Errorf("list apps: %w", err)
	}
	if err := cur.All(ctx, rows); err != nil {
		return 0, fmt.Errorf("list apps: %w", err)
	}
	return int(total), nil
}

// ownerPerPage is how many of one account's apps a page shows: some accounts
// publish dozens of near-identical repositories and would fill a shelf alone.
const ownerPerPage = 2

// SpreadOwners reorders ids ("owner/name", best first) so that no page of
// pageSize holds more than perPage ids of one owner. An id held back keeps its
// place ahead of everything after it, so it leads the first page with room.
func SpreadOwners(ids []string, pageSize, perPage int) []string {
	if pageSize <= 0 {
		return ids
	}
	out := make([]string, 0, len(ids))
	rest := ids
	for len(rest) > 0 {
		seen := map[string]int{}
		var held []string
		n := 0
		for i, id := range rest {
			if n == pageSize {
				held = append(held, rest[i:]...)
				break
			}
			owner, _, _ := strings.Cut(id, "/")
			if seen[owner] == perPage {
				held = append(held, id)
				continue
			}
			seen[owner]++
			out = append(out, id)
			n++
		}
		// A page nothing more fits on: the rest is one owner's, shown as is.
		if n == 0 {
			return append(out, held...)
		}
		rest = held
	}
	return out
}

// AppsReplacing decodes the apps that name the product slug as one they
// replace, most-starred first, listed or not.
func (s *Store) AppsReplacing(ctx context.Context, slug string, limit int, rows any) error {
	cur, err := s.Apps().Find(ctx, bson.M{"alternatives.slug": slug},
		options.Find().SetSort(bson.D{{Key: "stars", Value: -1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return fmt.Errorf("apps replacing %s: %w", slug, err)
	}
	if err := cur.All(ctx, rows); err != nil {
		return fmt.Errorf("apps replacing %s: %w", slug, err)
	}
	return nil
}
