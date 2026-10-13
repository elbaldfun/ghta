package taxonomy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// Op kinds. Domain ops rewrite categoryPath; shelf ops rewrite store.category.
const (
	OpRename  = "rename"  // 1:1 — mechanical
	OpMerge   = "merge"   // n:1 — mechanical
	OpSplit   = "split"   // 1:n — items re-queued for classification
	OpShelf   = "shelf"   // old apps/* domain leaf -> apps/<effective shelf>
	OpMoved   = "moved"   // shelf left the apps branch; store.category becomes a domain leaf
	OpRejudge = "rejudge" // shelf needs the LLM to pick among candidates
)

// Op is one line of migrations.yaml.
type Op struct {
	Kind       string   `yaml:"kind"`
	Old        string   `yaml:"old"`
	New        string   `yaml:"new,omitempty"`
	Candidates []string `yaml:"candidates,omitempty"`
	Fallback   string   `yaml:"fallback,omitempty"` // shelf op: slug to use when the item has no shelf
}

// Migration moves items from tree version From to To.
type Migration struct {
	From    int  `yaml:"from"`
	To      int  `yaml:"to"`
	Domain  []Op `yaml:"domain"`
	Shelves []Op `yaml:"shelves"`
}

// LoadMigrations parses migrations.yaml and validates it against the tree:
// every rename/merge/moved target and every split/rejudge candidate must be a
// leaf of the current tree, so a typo fails at boot instead of silently
// scattering items.
func LoadMigrations(path string, tree Tree) ([]Migration, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var ms []Migration
	if err := yaml.Unmarshal(raw, &ms); err != nil {
		return nil, fmt.Errorf("parse migrations: %w", err)
	}
	leaves := map[string]bool{}
	for _, n := range tree.Leaves() {
		leaves[n.Path] = true
	}
	shelfOK := func(slug string) bool { return leaves[ShelfPath(slug)] }
	for _, m := range ms {
		if m.To != m.From+1 {
			return nil, fmt.Errorf("migration %d->%d: must step one version", m.From, m.To)
		}
		if m.To != tree.Version {
			continue // older step: targets may since have moved again; only the current step is validated
		}
		for _, op := range m.Domain {
			switch op.Kind {
			case OpRename, OpMerge:
				if !leaves[op.New] {
					return nil, fmt.Errorf("domain %s %s: target %q is not a leaf", op.Kind, op.Old, op.New)
				}
				if op.Old == op.New {
					return nil, fmt.Errorf("domain %s %s: old and new are identical", op.Kind, op.Old)
				}
			case OpSplit:
				if len(op.Candidates) < 2 {
					return nil, fmt.Errorf("domain split %s: needs >= 2 candidates", op.Old)
				}
				for _, c := range op.Candidates {
					if !leaves[c] {
						return nil, fmt.Errorf("domain split %s: candidate %q is not a leaf", op.Old, c)
					}
				}
			case OpShelf:
				if op.Fallback != "" && !shelfOK(op.Fallback) {
					return nil, fmt.Errorf("domain shelf %s: fallback %q is not a shelf", op.Old, op.Fallback)
				}
			default:
				return nil, fmt.Errorf("domain op %s: unknown kind %q", op.Old, op.Kind)
			}
		}
		for _, op := range m.Shelves {
			switch op.Kind {
			case OpRename, OpMerge:
				if !shelfOK(op.New) {
					return nil, fmt.Errorf("shelf %s %s: target %q is not a shelf", op.Kind, op.Old, op.New)
				}
			case OpMoved:
				if !leaves[op.New] || IsShelfPath(op.New) {
					return nil, fmt.Errorf("shelf moved %s: target %q must be a non-apps leaf", op.Old, op.New)
				}
			case OpRejudge:
				if len(op.Candidates) < 2 {
					return nil, fmt.Errorf("shelf rejudge %s: needs >= 2 candidates", op.Old)
				}
				for _, c := range op.Candidates {
					if !leaves[c] {
						return nil, fmt.Errorf("shelf rejudge %s: candidate %q is not a leaf", op.Old, c)
					}
				}
			default:
				return nil, fmt.Errorf("shelf op %s: unknown kind %q", op.Old, op.Kind)
			}
		}
	}
	return ms, nil
}

// Aliases returns old-path -> new-path for the mechanical ops of every step up
// to the tree version, for URL redirects and golden normalization. A split
// maps to its first candidate (the largest expected leaf).
func Aliases(ms []Migration) map[string]string {
	out := map[string]string{}
	for _, m := range ms {
		for _, op := range m.Domain {
			switch op.Kind {
			case OpRename, OpMerge:
				out[op.Old] = op.New
			case OpSplit:
				out[op.Old] = op.Candidates[0]
			}
		}
	}
	// Chase chains (v2->v3->v4) to the final path.
	for k, v := range out {
		for i := 0; i < 8; i++ {
			next, ok := out[v]
			if !ok || next == v {
				break
			}
			v = next
		}
		out[k] = v
	}
	return out
}

// Stats is what one Migrate run did.
type Stats struct {
	DomainRewritten int64 `json:"domainRewritten"`
	DomainRequeued  int64 `json:"domainRequeued"`
	ShelfRewritten  int64 `json:"shelfRewritten"`
	ShelfRequeued   int64 `json:"shelfRequeued"`
	AppsRelabelled  int64 `json:"appsRelabelled"`
	Stamped         int64 `json:"stamped"`
}

// Migrator applies one migration step. It is idempotent: every write is keyed
// on the OLD value, so a second run (or a run resumed after a crash) finds
// nothing left to touch. The categorizer and storefinder do the LLM part later
// by picking up what Migrate re-queued; Migrate itself never calls a model.
type Migrator struct {
	store *repository.Store
	tree  Tree
	log   *slog.Logger
}

func NewMigrator(store *repository.Store, tree Tree, log *slog.Logger) *Migrator {
	return &Migrator{store: store, tree: tree, log: log}
}

// Run applies m. storeVersion is the store judgement generation to stamp on
// mechanically migrated shelves so the storefinder does not re-judge them.
func (mg *Migrator) Run(ctx context.Context, m Migration, storeVersion int) (Stats, error) {
	var st Stats
	if m.To != mg.tree.Version {
		return st, fmt.Errorf("migration targets v%d but the loaded tree is v%d", m.To, mg.tree.Version)
	}
	idByPath, err := mg.leafIDs(ctx)
	if err != nil {
		return st, err
	}

	for _, op := range m.Domain {
		switch op.Kind {
		case OpRename, OpMerge:
			n, err := mg.rewritePath(ctx, op.Old, op.New, idByPath)
			if err != nil {
				return st, fmt.Errorf("%s %s: %w", op.Kind, op.Old, err)
			}
			st.DomainRewritten += n
		case OpSplit:
			n, err := mg.requeuePath(ctx, op.Old)
			if err != nil {
				return st, fmt.Errorf("split %s: %w", op.Old, err)
			}
			st.DomainRequeued += n
		case OpShelf:
			n, rq, err := mg.relabelApps(ctx, op, idByPath)
			if err != nil {
				return st, fmt.Errorf("shelf %s: %w", op.Old, err)
			}
			st.AppsRelabelled += n
			st.DomainRequeued += rq
		}
		mg.log.Info("taxonomy migrate: domain op done", "kind", op.Kind, "old", op.Old)
	}

	for _, op := range m.Shelves {
		switch op.Kind {
		case OpRename, OpMerge:
			n, err := mg.rewriteShelf(ctx, op.Old, op.New, ShelfPath(op.New), idByPath, storeVersion)
			if err != nil {
				return st, fmt.Errorf("shelf %s %s: %w", op.Kind, op.Old, err)
			}
			st.ShelfRewritten += n
		case OpMoved:
			n, err := mg.rewriteShelf(ctx, op.Old, op.New, op.New, idByPath, storeVersion)
			if err != nil {
				return st, fmt.Errorf("shelf moved %s: %w", op.Old, err)
			}
			st.ShelfRewritten += n
		case OpRejudge:
			n, err := mg.requeueShelf(ctx, op.Old)
			if err != nil {
				return st, fmt.Errorf("shelf rejudge %s: %w", op.Old, err)
			}
			st.ShelfRequeued += n
		}
		mg.log.Info("taxonomy migrate: shelf op done", "kind", op.Kind, "old", op.Old)
	}

	// Everything still on the old version that no op touched (leaves that kept
	// their path) is simply stamped current: nothing about it changed.
	res, err := mg.store.Items().UpdateMany(ctx,
		bson.M{
			"analysisStatus": domain.AnalysisDone,
			"$or": bson.A{
				bson.M{"taxonomyVersion": bson.M{"$exists": false}},
				bson.M{"taxonomyVersion": bson.M{"$lt": m.To}},
			},
		},
		bson.M{"$set": bson.M{"taxonomyVersion": m.To}},
	)
	if err != nil {
		return st, fmt.Errorf("stamp version: %w", err)
	}
	st.Stamped = res.ModifiedCount
	return st, nil
}

// leafIDs maps every leaf path of the synced tree to its category id, for
// keeping categoryId parallel to categoryPath.
func (mg *Migrator) leafIDs(ctx context.Context) (map[string]string, error) {
	out, err := LeafIDs(ctx, mg.store)
	if err != nil {
		return nil, err
	}
	for _, n := range mg.tree.Leaves() {
		if _, ok := out[n.Path]; !ok {
			return nil, fmt.Errorf("leaf %s is not synced into categories (run Sync first)", n.Path)
		}
	}
	return out, nil
}

// LeafIDs maps every synced taxonomy path to its category id.
func LeafIDs(ctx context.Context, store *repository.Store) (map[string]string, error) {
	cur, err := store.Categories().Find(ctx, bson.M{"createdBy": "taxonomy"})
	if err != nil {
		return nil, err
	}
	var cats []domain.Category
	if err := cur.All(ctx, &cats); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(cats))
	for _, c := range cats {
		out[c.Path] = c.ID.Hex()
	}
	return out, nil
}

// pathArr reads categoryPath as an array whatever shape it was stored in.
// Items classified before multi-label (change 12) still hold a bare string —
// "" for uncategorized, or one path — which domain.PathList tolerates on read
// but array operators ($map, $setUnion) reject outright.
func pathArr() bson.M {
	return bson.M{"$switch": bson.M{
		"branches": bson.A{
			bson.M{"case": bson.M{"$isArray": bson.A{"$categoryPath"}}, "then": "$categoryPath"},
			bson.M{"case": bson.M{"$in": bson.A{bson.M{"$ifNull": bson.A{"$categoryPath", ""}}, bson.A{""}}}, "then": bson.A{}},
		},
		"default": bson.A{"$categoryPath"},
	}}
}

// snapshotPrev is the $set fragment that records categoryPathPrev once.
func snapshotPrev() bson.M {
	return bson.M{"$ifNull": bson.A{"$categoryPathPrev", "$categoryPath"}}
}

// pathRewrite is an aggregation expression replacing old with new inside
// categoryPath and deduping (a merge can land the same path twice).
func pathRewrite(old, new string) bson.M {
	return bson.M{"$setUnion": bson.A{
		bson.M{"$map": bson.M{
			"input": pathArr(),
			"in":    bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$$this", old}}, new, "$$this"}},
		}},
		bson.A{},
	}}
}

// idsFromPaths rebuilds categoryId from categoryPath via a $switch over the
// leaf table — categoryId is only ever derived, never authoritative.
func idsFromPaths(idByPath map[string]string) bson.M {
	branches := make(bson.A, 0, len(idByPath))
	for p, id := range idByPath {
		branches = append(branches, bson.M{"case": bson.M{"$eq": bson.A{"$$this", p}}, "then": id})
	}
	return bson.M{"$filter": bson.M{
		"input": bson.M{"$map": bson.M{
			"input": pathArr(),
			"in":    bson.M{"$switch": bson.M{"branches": branches, "default": nil}},
		}},
		"cond": bson.M{"$ne": bson.A{"$$this", nil}},
	}}
}

// rewritePath is rename/merge: items carrying old get new in its place, keep
// everything else, and are stamped current — they need no re-classification.
func (mg *Migrator) rewritePath(ctx context.Context, old, new string, idByPath map[string]string) (int64, error) {
	pipeline := bson.A{
		bson.D{{Key: "$set", Value: bson.M{"categoryPathPrev": snapshotPrev()}}},
		bson.D{{Key: "$set", Value: bson.M{"categoryPath": pathRewrite(old, new)}}},
		bson.D{{Key: "$set", Value: bson.M{"categoryId": idsFromPaths(idByPath), "taxonomyVersion": mg.tree.Version}}},
	}
	res, err := mg.store.Items().UpdateMany(ctx, bson.M{"categoryPath": old}, pipeline)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// requeuePath is split: the item keeps its old label (the site keeps serving
// it under the old leaf, which Sync leaves in place while referenced) and goes
// back to pending so the categorizer re-files it on the new leaves. The
// version stays old until that happens.
func (mg *Migrator) requeuePath(ctx context.Context, old string) (int64, error) {
	pipeline := bson.A{
		bson.D{{Key: "$set", Value: bson.M{
			"categoryPathPrev":  snapshotPrev(),
			"analysisStatus":    domain.AnalysisPending,
			"analysisFailCount": 0,
		}}},
	}
	res, err := mg.store.Items().UpdateMany(ctx,
		bson.M{"categoryPath": old, "analysisStatus": bson.M{"$ne": domain.AnalysisPending}},
		pipeline)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// relabelApps retires one old apps/* domain leaf. An item with an effective
// shelf (override, else LLM verdict, else the op's fallback) gets
// apps/<shelf> in place of the old label; an item with none loses the label,
// and if that leaves it with no domain at all it is re-queued. Excluded items
// (frameworks and lists the store cleanse caught) just lose the label.
func (mg *Migrator) relabelApps(ctx context.Context, op Op, idByPath map[string]string) (relabelled, requeued int64, err error) {
	cur, err := mg.store.Items().Find(ctx, bson.M{"categoryPath": op.Old},
		options.Find().SetProjection(bson.M{"categoryPath": 1, "categoryPathPrev": 1, "store.category": 1, "store.categoryOverride": 1}))
	if err != nil {
		return 0, 0, err
	}
	defer cur.Close(ctx)
	var models []mongo.WriteModel
	flush := func() error {
		if len(models) == 0 {
			return nil
		}
		_, err := mg.store.Items().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
		models = models[:0]
		return err
	}
	for cur.Next(ctx) {
		var it struct {
			ID               primitive.ObjectID `bson:"_id"`
			CategoryPath     domain.PathList    `bson:"categoryPath"`
			CategoryPathPrev domain.PathList    `bson:"categoryPathPrev"`
			Store            *domain.StoreInfo  `bson:"store"`
		}
		if err := cur.Decode(&it); err != nil {
			return relabelled, requeued, err
		}
		shelf := ""
		if it.Store != nil {
			shelf = it.Store.CategoryOverride
			if shelf == "" {
				shelf = it.Store.Category
			}
		}
		if shelf == "excluded" {
			shelf = ""
		} else if shelf == "" && op.Fallback != "" {
			shelf = op.Fallback
		}
		newPaths := make([]string, 0, len(it.CategoryPath)+1)
		for _, p := range it.CategoryPath {
			if p == op.Old {
				continue
			}
			newPaths = append(newPaths, p)
		}
		if shelf != "" {
			if sp := ShelfPath(shelf); idByPath[sp] != "" && !contains(newPaths, sp) {
				newPaths = append(newPaths, sp)
			}
		}
		set := bson.M{"categoryPath": newPaths, "categoryId": idsOf(newPaths, idByPath)}
		if len(it.CategoryPathPrev) == 0 {
			set["categoryPathPrev"] = it.CategoryPath
		}
		if len(newPaths) == 0 {
			set["analysisStatus"] = domain.AnalysisPending
			set["analysisFailCount"] = 0
			requeued++
		} else {
			set["taxonomyVersion"] = mg.tree.Version
			relabelled++
		}
		models = append(models, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": it.ID}).SetUpdate(bson.M{"$set": set}))
		if len(models) >= 500 {
			if err := flush(); err != nil {
				return relabelled, requeued, err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return relabelled, requeued, err
	}
	return relabelled, requeued, flush()
}

// rewriteShelf moves store.category (and a matching override) from old to new,
// stamps the store version so the storefinder leaves it alone, and makes sure
// the new path is also a domain label (one tree: the shelf IS the domain leaf).
func (mg *Migrator) rewriteShelf(ctx context.Context, old, newSlug, newPath string, idByPath map[string]string, storeVersion int) (int64, error) {
	var total int64
	for _, field := range []string{"store.category", "store.categoryOverride"} {
		pipeline := bson.A{
			bson.D{{Key: "$set", Value: bson.M{"categoryPathPrev": snapshotPrev()}}},
			bson.D{{Key: "$set", Value: bson.M{
				field:          newSlug,
				"categoryPath": bson.M{"$setUnion": bson.A{pathArr(), bson.A{newPath}}},
			}}},
			bson.D{{Key: "$set", Value: bson.M{"categoryId": idsFromPaths(idByPath)}}},
		}
		if field == "store.category" {
			pipeline = append(pipeline, bson.D{{Key: "$set", Value: bson.M{"store.version": storeVersion}}})
		}
		res, err := mg.store.Items().UpdateMany(ctx, bson.M{field: old}, pipeline)
		if err != nil {
			return total, err
		}
		total += res.ModifiedCount
	}
	return total, nil
}

// requeueShelf drops the store verdict's version so the storefinder re-judges
// the item (its prompt now lists the new shelves). Overrides are kept.
func (mg *Migrator) requeueShelf(ctx context.Context, old string) (int64, error) {
	res, err := mg.store.Items().UpdateMany(ctx,
		bson.M{"store.category": old},
		bson.M{"$set": bson.M{"store.version": 0}, "$unset": bson.M{"store.failCount": "", "store.status": ""}})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// ClearRollbackSnapshots drops categoryPathPrev on items migrated before
// `before` — call it once the rollback window (30 days) has passed.
func (mg *Migrator) ClearRollbackSnapshots(ctx context.Context, before time.Time) (int64, error) {
	res, err := mg.store.Items().UpdateMany(ctx,
		bson.M{"categoryPathPrev": bson.M{"$exists": true}, "updatedAt": bson.M{"$lt": before}},
		bson.M{"$unset": bson.M{"categoryPathPrev": ""}})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// Rollback restores categoryPath from categoryPathPrev for every item that has
// one and marks them done on `toVersion` — the whole-step undo.
func (mg *Migrator) Rollback(ctx context.Context, toVersion int, idByPath map[string]string) (int64, error) {
	pipeline := bson.A{
		bson.D{{Key: "$set", Value: bson.M{"categoryPath": "$categoryPathPrev", "analysisStatus": domain.AnalysisDone, "taxonomyVersion": toVersion}}},
		bson.D{{Key: "$set", Value: bson.M{"categoryId": idsFromPaths(idByPath)}}},
		bson.D{{Key: "$unset", Value: "categoryPathPrev"}},
	}
	res, err := mg.store.Items().UpdateMany(ctx, bson.M{"categoryPathPrev": bson.M{"$exists": true}}, pipeline)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+){1,2}$`)

// ValidPath is the shape check shared by handlers that accept a path from a URL.
func ValidPath(p string) bool { return slugRe.MatchString(p) }

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func idsOf(paths []string, idByPath map[string]string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if id := idByPath[p]; id != "" {
			out = append(out, id)
		}
	}
	return out
}
