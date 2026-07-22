package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// ErrNotFound is returned when a document lookup misses (mapped to HTTP 404).
var ErrNotFound = errors.New("not found")

// CategoryInput is the create/update payload.
type CategoryInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ParentID    *string `json:"parentId"`
	Path        string  `json:"path"`
}

// CategoryTree is a nested view of the classification tree. Count is the number
// of items filed under the node (distinct items for a parent; direct items for a
// leaf), added by FindAll.
type CategoryTree struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	NameEn   string         `json:"nameEn,omitempty"`
	Path     string         `json:"path"`
	Count    int            `json:"count"`
	Children []CategoryTree `json:"children,omitempty"`
}

// TypeFacet is one form-facet value with its item count, for the filter UI.
type TypeFacet struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// categoryCacheTTL bounds how stale the tree/facet counts may be. The count
// aggregations scan the whole tracked_items collection (~20s on the prod box),
// so they must not run per-request; counts only shift once a day (categorize
// job), making a few minutes of staleness invisible.
const categoryCacheTTL = 5 * time.Minute

type CategoryService struct {
	store *repository.Store

	mu         sync.Mutex
	treeCache  []CategoryTree
	treeAt     time.Time
	facetCache []TypeFacet
	facetAt    time.Time
}

func NewCategoryService(store *repository.Store) *CategoryService {
	return &CategoryService{store: store}
}

func (s *CategoryService) Create(ctx context.Context, in CategoryInput) (*domain.Category, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Path) == "" {
		return nil, badInput("name and path are required")
	}
	var parentID *primitive.ObjectID
	if in.ParentID != nil && *in.ParentID != "" {
		oid, err := primitive.ObjectIDFromHex(*in.ParentID)
		if err != nil {
			return nil, badInput("invalid parentId")
		}
		parentID = &oid
	}
	now := time.Now().UTC()
	cat := domain.Category{
		Name:        in.Name,
		Description: in.Description,
		ParentID:    parentID,
		Path:        in.Path,
		Level:       len(strings.Split(strings.Trim(in.Path, "/"), "/")),
		CreatedBy:   "human",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	res, err := s.store.Categories().InsertOne(ctx, cat)
	if err != nil {
		return nil, err
	}
	cat.ID = res.InsertedID.(primitive.ObjectID)
	return &cat, nil
}

// FindAll returns the controlled domain tree (createdBy=taxonomy only — legacy
// AI-created categories are excluded) as a nested tree with per-node item counts.
// Result is cached for categoryCacheTTL: the count aggregation is expensive and
// counts move slowly.
func (s *CategoryService) FindAll(ctx context.Context) ([]CategoryTree, error) {
	s.mu.Lock()
	if s.treeCache != nil && time.Since(s.treeAt) < categoryCacheTTL {
		cached := s.treeCache
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	tree, err := s.buildTreeWithCounts(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.treeCache, s.treeAt = tree, time.Now()
	s.mu.Unlock()
	return tree, nil
}

func (s *CategoryService) buildTreeWithCounts(ctx context.Context) ([]CategoryTree, error) {
	cur, err := s.store.Categories().Find(ctx, bson.M{"createdBy": "taxonomy"})
	if err != nil {
		return nil, err
	}
	var cats []domain.Category
	if err := cur.All(ctx, &cats); err != nil {
		return nil, err
	}
	counts, err := s.categoryCounts(ctx)
	if err != nil {
		return nil, err
	}
	tree := buildTree(cats, "")
	applyCounts(tree, counts)
	return tree, nil
}

// categoryCounts returns item counts keyed by category path. Leaf counts come
// from unwinding categoryPath (an item genuinely in two leaves counts in each).
// Parent counts are distinct items across the subtree — computed from the
// top-level path segment so an item in two leaves of one parent counts once.
func (s *CategoryService) categoryCounts(ctx context.Context) (map[string]int, error) {
	counts := map[string]int{}

	// Leaf counts: one row per (item, leaf path).
	leafCur, err := s.store.Items().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$unwind", Value: "$categoryPath"}},
		{{Key: "$group", Value: bson.M{"_id": "$categoryPath", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	if err := accumulate(ctx, leafCur, counts); err != nil {
		return nil, err
	}

	// Parent counts: distinct items per top-level segment.
	parentCur, err := s.store.Items().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$unwind", Value: "$categoryPath"}},
		{{Key: "$group", Value: bson.M{"_id": bson.M{
			"item":   "$_id",
			"parent": bson.M{"$arrayElemAt": []interface{}{bson.M{"$split": []interface{}{"$categoryPath", "/"}}, 0}},
		}}}},
		{{Key: "$group", Value: bson.M{"_id": "$_id.parent", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	if err := accumulate(ctx, parentCur, counts); err != nil {
		return nil, err
	}
	return counts, nil
}

func accumulate(ctx context.Context, cur *mongo.Cursor, into map[string]int) error {
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var row struct {
			ID string `bson:"_id"`
			N  int    `bson:"n"`
		}
		if err := cur.Decode(&row); err != nil {
			return err
		}
		if row.ID != "" {
			into[row.ID] = row.N // leaf and parent keys never collide (leaves have "/")
		}
	}
	return cur.Err()
}

func applyCounts(tree []CategoryTree, counts map[string]int) {
	for i := range tree {
		tree[i].Count = counts[tree[i].Path]
		applyCounts(tree[i].Children, counts)
	}
}

// ResetAnalysis sets up to `limit` done/failed items back to pending (limit 0 =
// all), for change 12's re-classification on the new tree. Returns the count
// reset. Only analysisStatus/analysisFailCount change — existing categoryPath/
// type stay until the categorizer overwrites them.
func ResetAnalysis(ctx context.Context, store *repository.Store, limit int) (int64, error) {
	filter := bson.M{"analysisStatus": bson.M{"$in": []string{domain.AnalysisDone, domain.AnalysisFailed}}}
	set := bson.M{"$set": bson.M{"analysisStatus": domain.AnalysisPending, "analysisFailCount": 0}}

	if limit <= 0 {
		res, err := store.Items().UpdateMany(ctx, filter, set)
		if err != nil {
			return 0, err
		}
		return res.ModifiedCount, nil
	}

	// Staged: reset only `limit` ids this call.
	cur, err := store.Items().Find(ctx, filter, options.Find().SetLimit(int64(limit)).SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return 0, err
	}
	var docs []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return 0, err
	}
	ids := make([]primitive.ObjectID, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res, err := store.Items().UpdateMany(ctx, bson.M{"_id": bson.M{"$in": ids}}, set)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// TypeFacets returns each facet type value (in facets.yaml priority order) with
// its item count, for the filter chips. Values with no items are still listed.
func (s *CategoryService) TypeFacets(ctx context.Context, order []TypeFacet) ([]TypeFacet, error) {
	s.mu.Lock()
	if s.facetCache != nil && time.Since(s.facetAt) < categoryCacheTTL {
		cached := s.facetCache
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	cur, err := s.store.Items().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$type", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	if err := accumulate(ctx, cur, counts); err != nil {
		return nil, err
	}
	out := make([]TypeFacet, len(order))
	for i, f := range order {
		out[i] = TypeFacet{Key: f.Key, Name: f.Name, Count: counts[f.Key]}
	}
	s.mu.Lock()
	s.facetCache, s.facetAt = out, time.Now()
	s.mu.Unlock()
	return out, nil
}

func (s *CategoryService) FindOne(ctx context.Context, id string) (*domain.Category, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, badInput("invalid id")
	}
	var cat domain.Category
	err = s.store.Categories().FindOne(ctx, bson.M{"_id": oid}).Decode(&cat)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func (s *CategoryService) Update(ctx context.Context, id string, in CategoryInput) (*domain.Category, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, badInput("invalid id")
	}
	set := bson.M{"updatedAt": time.Now().UTC()}
	if in.Name != "" {
		set["name"] = in.Name
	}
	if in.Description != "" {
		set["description"] = in.Description
	}
	if in.Path != "" {
		set["path"] = in.Path
		set["level"] = len(strings.Split(strings.Trim(in.Path, "/"), "/"))
	}
	res := s.store.Categories().FindOneAndUpdate(ctx,
		bson.M{"_id": oid},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var cat domain.Category
	if err := res.Decode(&cat); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &cat, nil
}

func (s *CategoryService) Remove(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return badInput("invalid id")
	}
	res, err := s.store.Categories().DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// buildTree assembles a nested tree from a flat category list, matching each
// node's parentId to the given parent ("" == root, i.e. nil parentId).
func buildTree(cats []domain.Category, parentID string) []CategoryTree {
	tree := []CategoryTree{}
	for _, c := range cats {
		cp := ""
		if c.ParentID != nil {
			cp = c.ParentID.Hex()
		}
		if cp != parentID {
			continue
		}
		node := CategoryTree{ID: c.ID.Hex(), Name: c.Name, NameEn: c.NameEn, Path: c.Path}
		if children := buildTree(cats, c.ID.Hex()); len(children) > 0 {
			node.Children = children
		}
		tree = append(tree, node)
	}
	return tree
}
