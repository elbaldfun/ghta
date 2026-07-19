// Package repository owns the MongoDB connection, collection accessors, and
// schema/index provisioning.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/config"
)

const (
	CollItems       = "tracked_items"
	CollCategories  = "categories"
	CollUsers       = "users"
	CollFetchRuns   = "fetch_runs"
	CollSnapshots   = "metric_snapshots"
	CollSuggestions = "category_suggestions"
	CollStarHistory = "star_history"
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

func (s *Store) Items() *mongo.Collection       { return s.DB.Collection(CollItems) }
func (s *Store) Categories() *mongo.Collection  { return s.DB.Collection(CollCategories) }
func (s *Store) Users() *mongo.Collection       { return s.DB.Collection(CollUsers) }
func (s *Store) FetchRuns() *mongo.Collection   { return s.DB.Collection(CollFetchRuns) }
func (s *Store) Snapshots() *mongo.Collection   { return s.DB.Collection(CollSnapshots) }
func (s *Store) Suggestions() *mongo.Collection { return s.DB.Collection(CollSuggestions) }
func (s *Store) StarHistory() *mongo.Collection { return s.DB.Collection(CollStarHistory) }

// EnsureSchema creates the time-series snapshot collection (if absent) and all
// indexes. It is idempotent and safe to run on every startup.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if err := s.ensureSnapshotCollection(ctx); err != nil {
		return err
	}
	return s.ensureIndexes(ctx)
}

func (s *Store) ensureSnapshotCollection(ctx context.Context) error {
	names, err := s.DB.ListCollectionNames(ctx, bson.M{"name": CollSnapshots})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	if len(names) > 0 {
		return nil // already exists
	}
	// Retain snapshots ~400 days (covers year-over-year), then auto-expire.
	const retentionSeconds = int64(400 * 24 * 60 * 60)
	opts := options.CreateCollection().
		SetTimeSeriesOptions(
			options.TimeSeries().
				SetTimeField("capturedAt").
				SetMetaField("meta").
				SetGranularity("hours"),
		).
		SetExpireAfterSeconds(retentionSeconds)
	if err := s.DB.CreateCollection(ctx, CollSnapshots, opts); err != nil {
		return fmt.Errorf("create timeseries collection: %w", err)
	}
	return nil
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	itemIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "source", Value: 1}, {Key: "externalId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_source_externalId"),
		},
		{Keys: bson.D{{Key: "source", Value: 1}}},
		{Keys: bson.D{{Key: "language", Value: 1}}},
		{Keys: bson.D{{Key: "fetchedAt", Value: -1}}},
		{Keys: bson.D{{Key: "categoryId", Value: 1}}},
		{Keys: bson.D{{Key: "analysisStatus", Value: 1}}},
		{Keys: bson.D{{Key: "weeklyIncrease", Value: -1}}},
		{Keys: bson.D{{Key: "metrics.stars", Value: -1}}},
		{Keys: bson.D{{Key: "sourceData.topicNames", Value: 1}}},
	}
	if _, err := s.Items().Indexes().CreateMany(ctx, itemIndexes); err != nil {
		return fmt.Errorf("item indexes: %w", err)
	}

	if _, err := s.Categories().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "path", Value: 1}},
		Options: options.Index().SetName("category_path"),
	}); err != nil {
		return fmt.Errorf("category indexes: %w", err)
	}

	if _, err := s.Suggestions().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "path", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_suggestion_path"),
	}); err != nil {
		return fmt.Errorf("suggestion indexes: %w", err)
	}

	if _, err := s.FetchRuns().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "source", Value: 1}, {Key: "date", Value: 1}, {Key: "shard", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_source_date_shard"),
	}); err != nil {
		return fmt.Errorf("fetchrun indexes: %w", err)
	}

	if _, err := s.StarHistory().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "source", Value: 1}, {Key: "externalId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_history_source_externalId"),
	}); err != nil {
		return fmt.Errorf("star history indexes: %w", err)
	}
	return nil
}

// Close disconnects the client.
func (s *Store) Close(ctx context.Context) error {
	return s.Client.Disconnect(ctx)
}
