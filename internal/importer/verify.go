package importer

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

// Size limits of the rebuilt hot collection (docs/REBUILD.md §6). They are
// only meaningful on a production-sized corpus.
const (
	maxItemsStorageBytes = 250 << 20
	maxAvgItemBytes      = 2560
	sizeCheckMinItems    = 10000
	topGrowthN           = 500
	minTopGrowthOverlap  = 0.98
)

// Check is one line of the verification report.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Passed reports whether every check is OK.
func Passed(cs []Check) bool {
	for _, c := range cs {
		if !c.OK {
			return false
		}
	}
	return true
}

// Verify compares the imported collections against the old store: the gate
// for the cutover. sample is how many items are re-converted and compared
// field by field (0 = every item).
func Verify(ctx context.Context, db *mongo.Database, r Rules, sample int) ([]Check, error) {
	var out []Check
	add := func(name string, ok bool, format string, a ...any) {
		out = append(out, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, a...)})
	}
	st := store.New(db)
	old := db.Collection(oldItems)

	// 1. Item counts by source.
	oldBySource, err := groupCount(ctx, old, nil, "$source")
	if err != nil {
		return nil, err
	}
	newBySource, err := groupCount(ctx, st.Items(), nil, "$source")
	if err != nil {
		return nil, err
	}
	add("item counts by source", reflect.DeepEqual(oldBySource, newBySource), "old %v, new %v", oldBySource, newBySource)

	// 2. Shelf counts. The old effective shelf is the override when set;
	// shelves that left the tree are cleared by the import, so they are left
	// out of the old side too.
	oldShelves, err := groupCount(ctx, old, bson.M{"store.category": bson.M{"$exists": true}},
		bson.M{"$cond": bson.A{
			bson.M{"$gt": bson.A{"$store.categoryOverride", ""}}, "$store.categoryOverride", "$store.category",
		}})
	if err != nil {
		return nil, err
	}
	for slug := range oldShelves {
		if slug == "" || (slug != item.ShelfExcluded && !r.ValidPath(r.ShelfPath(slug))) {
			delete(oldShelves, slug)
		}
	}
	newShelves, err := groupCount(ctx, st.Items(), bson.M{"labels.shelf": bson.M{"$gt": ""}}, "$labels.shelf")
	if err != nil {
		return nil, err
	}
	add("shelf counts", reflect.DeepEqual(oldShelves, newShelves), "%d shelves; differing: %s", len(newShelves), diffKeys(oldShelves, newShelves))

	// 3. Blob count: one per old item that has a README or releases.
	wantBlobs, err := old.CountDocuments(ctx, bson.M{"$or": bson.A{
		bson.M{"sourceData.readme": bson.M{"$gt": ""}},
		bson.M{"sourceData.releases.0": bson.M{"$exists": true}},
	}})
	if err != nil {
		return nil, err
	}
	gotBlobs, err := st.Blobs().CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	add("blob count", wantBlobs == gotBlobs, "old items with cold text %d, blobs %d", wantBlobs, gotBlobs)

	// 4. Field-by-field comparison of re-converted items, README included.
	compared, mismatched, firstBad, err := compareSample(ctx, db, st, r, sample)
	if err != nil {
		return nil, err
	}
	add("items equal to a fresh conversion", mismatched == 0 && compared > 0, "%d compared, %d differ%s", compared, mismatched, firstBad)

	// 5. The top of the weekly growth ranking survives the recomputation.
	overlap, n, err := topGrowthOverlap(ctx, old, st.Items())
	if err != nil {
		return nil, err
	}
	if n < topGrowthN {
		add("weekly growth top list", true, "skipped: only %d items with weekly growth", n)
	} else {
		add("weekly growth top list", overlap >= minTopGrowthOverlap, "top %d overlap %.1f%%", topGrowthN, overlap*100)
	}

	// 6. Size of the hot collection.
	var stats struct {
		Count       int64   `bson:"count"`
		StorageSize int64   `bson:"storageSize"`
		AvgObjSize  float64 `bson:"avgObjSize"`
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "collStats", Value: item.CollItems}}).Decode(&stats); err != nil {
		return nil, fmt.Errorf("collStats: %w", err)
	}
	detail := fmt.Sprintf("%d items, storage %.1f MB, average %.0f bytes", stats.Count, float64(stats.StorageSize)/(1<<20), stats.AvgObjSize)
	if stats.Count < sizeCheckMinItems {
		add("items size", true, "skipped on a small corpus: %s", detail)
	} else {
		add("items size", stats.StorageSize < maxItemsStorageBytes && stats.AvgObjSize <= maxAvgItemBytes, "%s", detail)
	}
	return out, nil
}

func groupCount(ctx context.Context, c *mongo.Collection, match bson.M, by any) (map[string]int, error) {
	pipeline := mongo.Pipeline{}
	if match != nil {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: match}})
	}
	pipeline = append(pipeline, bson.D{{Key: "$group", Value: bson.M{"_id": by, "n": bson.M{"$sum": 1}}}})
	cur, err := c.Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, fmt.Errorf("group %s: %w", c.Name(), err)
	}
	var rows []struct {
		ID string `bson:"_id"`
		N  int    `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.ID] = r.N
	}
	return out, nil
}

func diffKeys(a, b map[string]int) string {
	var d []string
	for k, v := range a {
		if b[k] != v {
			d = append(d, fmt.Sprintf("%s old=%d new=%d", k, v, b[k]))
		}
	}
	for k, v := range b {
		if _, ok := a[k]; !ok {
			d = append(d, fmt.Sprintf("%s old=0 new=%d", k, v))
		}
	}
	if len(d) == 0 {
		return "none"
	}
	sort.Strings(d)
	if len(d) > 8 {
		d = append(d[:8], fmt.Sprintf("… %d more", len(d)-8))
	}
	return strings.Join(d, "; ")
}

// compareSample re-converts old items and checks the stored item and blob are
// identical, apart from history and growth, which the import derives from the
// snapshot series rather than from the old document.
func compareSample(ctx context.Context, db *mongo.Database, st *store.Store, r Rules, sample int) (compared, mismatched int, firstBad string, err error) {
	pipeline := mongo.Pipeline{}
	if sample > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$sample", Value: bson.M{"size": sample}}})
	}
	cur, err := db.Collection(oldItems).Aggregate(ctx, pipeline, options.Aggregate().SetBatchSize(100))
	if err != nil {
		return 0, 0, "", fmt.Errorf("sample %s: %w", oldItems, err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var old domain.TrackedItem
		if err := cur.Decode(&old); err != nil {
			continue
		}
		want, wantBlob, _ := Convert(old, r)
		key := bson.M{"source": want.Source, "externalId": want.ExternalID}

		var got item.Item
		problem := ""
		if e := st.Items().FindOne(ctx, key).Decode(&got); e != nil {
			problem = "item missing"
		} else {
			want.History, want.Growth = got.History, got.Growth
			if !sameStored(want, got) {
				problem = "item fields differ"
			}
		}
		if problem == "" {
			var gotBlob item.Blob
			e := st.Blobs().FindOne(ctx, key).Decode(&gotBlob)
			switch {
			case wantBlob == nil && e == nil:
				problem = "unexpected blob"
			case wantBlob != nil && e != nil:
				problem = "blob missing"
			case wantBlob != nil && !sameStored(*wantBlob, gotBlob):
				problem = "blob differs"
			}
		}
		compared++
		if problem != "" {
			if mismatched++; firstBad == "" {
				firstBad = fmt.Sprintf("; first: %s (%s)", want.ExternalID, problem)
			}
		}
	}
	return compared, mismatched, firstBad, cur.Err()
}

// sameStored compares two values as they would be stored: want is passed
// through BSON first, so time precision and empty-versus-nil slices match what
// came back from the database.
func sameStored[T any](want, got T) bool {
	raw, err := bson.Marshal(want)
	if err != nil {
		return false
	}
	var round T
	if err := bson.Unmarshal(raw, &round); err != nil {
		return false
	}
	return reflect.DeepEqual(round, got)
}

// topGrowthOverlap is the share of the old weekly top list (GitHub, by
// weeklyIncrease) that is also in the new one (by growth.d7), and how many
// items the old list could draw from.
func topGrowthOverlap(ctx context.Context, old, items *mongo.Collection) (float64, int, error) {
	top := func(c *mongo.Collection, field string) (map[string]bool, error) {
		cur, err := c.Find(ctx, bson.M{"source": item.SourceGitHub, field: bson.M{"$ne": nil}},
			options.Find().SetSort(bson.D{{Key: field, Value: -1}}).SetLimit(topGrowthN).
				SetProjection(bson.M{"externalId": 1}).SetAllowDiskUse(true))
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ExternalID string `bson:"externalId"`
		}
		if err := cur.All(ctx, &rows); err != nil {
			return nil, err
		}
		set := make(map[string]bool, len(rows))
		for _, r := range rows {
			set[r.ExternalID] = true
		}
		return set, nil
	}
	a, err := top(old, "weeklyIncrease")
	if err != nil {
		return 0, 0, err
	}
	b, err := top(items, "growth.d7")
	if err != nil {
		return 0, 0, err
	}
	if len(a) == 0 {
		return 0, 0, nil
	}
	both := 0
	for id := range a {
		if b[id] {
			both++
		}
	}
	return float64(both) / float64(len(a)), len(a), nil
}
