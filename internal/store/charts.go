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

// CollChartSnapshots holds one document per store chart per day: the ranked
// external ids of an App Store (later Google Play) chart, as read that day
// (docs/MOBILE-APPS-DATA.md §6.2).
const CollChartSnapshots = "chart_snapshots"

// chartSnapshotTTL keeps a year of charts for year-over-year, plus a month.
const chartSnapshotTTL = 400 * 24 * time.Hour

func (s *Store) ChartSnapshots() *mongo.Collection { return s.DB.Collection(CollChartSnapshots) }

// ChartSnapshot is one chart on one day. Entries are external ids, best
// first: an entry's rank is its index plus one.
type ChartSnapshot struct {
	ID        string    `bson:"_id"`
	Source    string    `bson:"source"`
	Country   string    `bson:"country"`
	Chart     string    `bson:"chart"`
	Genre     string    `bson:"genre"`
	Day       string    `bson:"day"`
	FetchedAt time.Time `bson:"fetchedAt"`
	Entries   []string  `bson:"entries"`
}

// ChartKey is a chart snapshot's _id: "ios|us|free|6007|2026-11-21".
func ChartKey(source, country, chart, genre, day string) string {
	return strings.Join([]string{source, country, chart, genre, day}, "|")
}

func (s *Store) ensureChartIndexes(ctx context.Context) error {
	_, err := s.ChartSnapshots().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "source", Value: 1}, {Key: "country", Value: 1}, {Key: "chart", Value: 1},
			{Key: "genre", Value: 1}, {Key: "day", Value: -1}}},
		{Keys: bson.D{{Key: "fetchedAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(int32(chartSnapshotTTL.Seconds()))},
	})
	if err != nil {
		return fmt.Errorf("chart_snapshots indexes: %w", err)
	}
	return nil
}

// WriteChart stores a chart snapshot, replacing a same-day read of it.
func (s *Store) WriteChart(ctx context.Context, c ChartSnapshot) error {
	c.ID = ChartKey(c.Source, c.Country, c.Chart, c.Genre, c.Day)
	if c.Entries == nil {
		c.Entries = []string{}
	}
	_, err := s.ChartSnapshots().ReplaceOne(ctx, bson.M{"_id": c.ID}, c, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("write chart %s: %w", c.ID, err)
	}
	return nil
}

// Chart reads one chart snapshot; ok is false when that day was not read.
func (s *Store) Chart(ctx context.Context, source, country, chart, genre, day string) (c ChartSnapshot, ok bool, err error) {
	err = s.ChartSnapshots().FindOne(ctx, bson.M{"_id": ChartKey(source, country, chart, genre, day)}).Decode(&c)
	if err == mongo.ErrNoDocuments {
		return c, false, nil
	}
	if err != nil {
		return c, false, fmt.Errorf("read chart: %w", err)
	}
	return c, true, nil
}
