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

func (s *Store) Items() *mongo.Collection       { return s.DB.Collection(CollItems) }
func (s *Store) Categories() *mongo.Collection  { return s.DB.Collection(CollCategories) }
func (s *Store) Users() *mongo.Collection       { return s.DB.Collection(CollUsers) }
func (s *Store) FetchRuns() *mongo.Collection   { return s.DB.Collection(CollFetchRuns) }
func (s *Store) Snapshots() *mongo.Collection   { return s.DB.Collection(CollSnapshots) }
func (s *Store) Suggestions() *mongo.Collection { return s.DB.Collection(CollSuggestions) }
func (s *Store) StarHistory() *mongo.Collection { return s.DB.Collection(CollStarHistory) }
func (s *Store) Developers() *mongo.Collection  { return s.DB.Collection(CollDevelopers) }

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
		{Keys: bson.D{{Key: "source", Value: 1}}}, // source-only filter + the board's cached count
		{Keys: bson.D{{Key: "language", Value: 1}}},
		{Keys: bson.D{{Key: "categoryId", Value: 1}}},
		{Keys: bson.D{{Key: "categoryPath", Value: 1}}},
		{Keys: bson.D{{Key: "type", Value: 1}}},
		{Keys: bson.D{{Key: "analysisStatus", Value: 1}}},
		{Keys: bson.D{{Key: "sourceData.topicNames", Value: 1}}},
		{Keys: bson.D{{Key: "sourceData.platforms", Value: 1}}},                  // app directory: filter by OS
		{Keys: bson.D{{Key: "sourceData.latestRelease.publishedAt", Value: -1}}}, // app directory: "new" sort
		{Keys: bson.D{{Key: "alternativeTo.slug", Value: 1}}},                    // app directory: /alternatives/<slug> reverse lookup

		// Board sorts. Each sort field carries an _id tiebreaker in the SAME
		// direction. That does two things at once: (1) it gives every board a
		// total, deterministic order, so skip-paginating over tied sort values
		// (common at the low-star tail) can't duplicate or drop rows across pages;
		// (2) because _id is in the index, the sort {field, _id} stays fully
		// index-satisfied — no blocking in-memory sort, and one index still serves
		// both directions (forward = desc, reverse = asc). An unindexed sort would
		// otherwise load the whole match into memory and hard-fail past 32 MB.
		{Keys: bson.D{{Key: "metrics.stars", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_stars")},
		{Keys: bson.D{{Key: "metrics.forks", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_forks")},
		{Keys: bson.D{{Key: "metrics.openIssues", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_issues")},
		{Keys: bson.D{{Key: "fetchedAt", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_fetchedAt")},
		{Keys: bson.D{{Key: "dailyIncrease", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_daily")},
		{Keys: bson.D{{Key: "weeklyIncrease", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_weekly")},
		{Keys: bson.D{{Key: "monthlyIncrease", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_monthly")},

		// Filtered boards: the two primary navigation filters (category tree, type
		// facet) compounded with the default stars sort, so a filtered board seeks
		// straight to its slice instead of walking the whole stars-ordered index
		// and discarding non-matches. source leads because the site always scopes
		// to source=github.
		{Keys: bson.D{{Key: "source", Value: 1}, {Key: "categoryPath", Value: 1}, {Key: "metrics.stars", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_category_stars")},
		{Keys: bson.D{{Key: "source", Value: 1}, {Key: "type", Value: 1}, {Key: "metrics.stars", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("board_type_stars")},
	}
	if _, err := s.Items().Indexes().CreateMany(ctx, itemIndexes); err != nil {
		return fmt.Errorf("item indexes: %w", err)
	}

	// Reclaim the RAM held by the pre-tiebreaker single-field sort indexes now
	// superseded by the _id-terminated board_* indexes above (each board_* index
	// has the old key as a prefix, so it serves every query the old one did).
	// Best-effort and idempotent: a fresh database never had these, so a
	// "not found" here is expected and ignored.
	for _, name := range []string{
		"metrics.stars_-1", "metrics.forks_-1", "metrics.openIssues_-1",
		"fetchedAt_-1", "dailyIncrease_-1", "weeklyIncrease_-1", "monthlyIncrease_-1",
	} {
		_, _ = s.Items().Indexes().DropOne(ctx, name)
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

	// A time-series collection's automatic index covers the whole `meta` object,
	// which a query on meta.externalId cannot use. The metrics job does one such
	// lookup per tracked item, so without this index a pass degrades into ~67k
	// unindexed scans — measured at ~111ms each, over two hours per run.
	if _, err := s.Snapshots().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "meta.externalId", Value: 1},
			{Key: "meta.source", Value: 1},
			{Key: "capturedAt", Value: 1},
		},
		Options: options.Index().SetName("snapshot_lookup"),
	}); err != nil {
		return fmt.Errorf("snapshot indexes: %w", err)
	}

	devIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "login", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_developer_login"),
		},
		{Keys: bson.D{{Key: "type", Value: 1}}},       // filter User vs Organization
		{Keys: bson.D{{Key: "followers", Value: -1}}}, // rank by reach
		{Keys: bson.D{{Key: "fetchedAt", Value: 1}}},  // find stale/unfetched for refresh
		{Keys: bson.D{{Key: "twitterUsername", Value: 1}}, Options: options.Index().SetSparse(true)},
	}
	if _, err := s.Developers().Indexes().CreateMany(ctx, devIndexes); err != nil {
		return fmt.Errorf("developer indexes: %w", err)
	}
	return nil
}

// Close disconnects the client.
func (s *Store) Close(ctx context.Context) error {
	return s.Client.Disconnect(ctx)
}
