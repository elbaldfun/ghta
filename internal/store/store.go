// Package store is the persistence layer of the rebuilt data model
// (docs/REBUILD.md §4). It only ever touches the new collections.
package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// Store gives access to the rebuilt collections of one database.
type Store struct {
	DB *mongo.Database
}

func New(db *mongo.Database) *Store { return &Store{DB: db} }

func (s *Store) Items() *mongo.Collection { return s.DB.Collection(item.CollItems) }
func (s *Store) Blobs() *mongo.Collection { return s.DB.Collection(item.CollBlobs) }

// EnsureIndexes creates the indexes the write paths rely on. Read paths get
// theirs with the boards that need them (REBUILD M4); rankings are
// materialised, so the hot collection carries few indexes on purpose.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	key := bson.D{{Key: "source", Value: 1}, {Key: "externalId", Value: 1}}
	items := []mongo.IndexModel{
		{Keys: key, Options: options.Index().SetUnique(true)},
		// Label lookups: a category page, a store shelf.
		{Keys: bson.D{{Key: "labels.domain", Value: 1}}},
		{Keys: bson.D{{Key: "labels.shelf", Value: 1}}, Options: options.Index().SetSparse(true)},
		// The judge's queue.
		{Keys: bson.D{{Key: "jobs.judge.status", Value: 1}, {Key: "metrics.stars", Value: -1}}},
	}
	if _, err := s.Items().Indexes().CreateMany(ctx, items); err != nil {
		return fmt.Errorf("items indexes: %w", err)
	}
	if _, err := s.Blobs().Indexes().CreateOne(ctx, mongo.IndexModel{Keys: key, Options: options.Index().SetUnique(true)}); err != nil {
		return fmt.Errorf("item_blobs indexes: %w", err)
	}
	return nil
}

// Reset drops the rebuilt collections. The importer calls it so that every
// import starts from nothing and is repeatable.
func (s *Store) Reset(ctx context.Context) error {
	for _, c := range []*mongo.Collection{s.Items(), s.Blobs()} {
		if err := c.Drop(ctx); err != nil {
			return fmt.Errorf("drop %s: %w", c.Name(), err)
		}
	}
	return nil
}
