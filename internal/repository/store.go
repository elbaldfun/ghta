// Package repository owns the MongoDB connection, collection accessors, and
// schema/index provisioning.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/config"
)

const (
	CollItems       = "tracked_items"
	CollCategories  = "categories"
	CollFetchRuns   = "fetch_runs"
	CollSnapshots   = "metric_snapshots"
	CollSuggestions = "category_suggestions"
	CollStarHistory = "star_history"
	CollDevelopers  = "developers"
)

type Store struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Connect dials MongoDB and pings to confirm the connection.
func Connect(ctx context.Context, cfg *config.Config) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	return &Store{Client: client, DB: client.Database(cfg.MongoDB)}, nil
}

func (s *Store) StarHistory() *mongo.Collection { return s.DB.Collection(CollStarHistory) }

// Close disconnects the client.
func (s *Store) Close(ctx context.Context) error {
	return s.Client.Disconnect(ctx)
}
