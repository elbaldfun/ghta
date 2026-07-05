package service

import (
	"context"
	"errors"
	"strings"
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

// CategoryTree is a nested view of the classification tree.
type CategoryTree struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Children []CategoryTree `json:"children,omitempty"`
}

type CategoryService struct {
	store *repository.Store
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

func (s *CategoryService) FindAll(ctx context.Context) ([]CategoryTree, error) {
	cur, err := s.store.Categories().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var cats []domain.Category
	if err := cur.All(ctx, &cats); err != nil {
		return nil, err
	}
	return buildTree(cats, ""), nil
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
		node := CategoryTree{ID: c.ID.Hex(), Name: c.Name, Path: c.Path}
		if children := buildTree(cats, c.ID.Hex()); len(children) > 0 {
			node.Children = children
		}
		tree = append(tree, node)
	}
	return tree
}
