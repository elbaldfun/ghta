package store

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// Reads for the store-chart pages (/v1/ios). The writes are the ios job's
// (charts.go, pipeline/ios.go).

// LatestCharts returns the n most recent snapshots of one chart, newest
// first: today's and the baseline a move is measured against.
func (s *Store) LatestCharts(ctx context.Context, source, country, chart, genre string, n int) ([]ChartSnapshot, error) {
	cur, err := s.ChartSnapshots().Find(ctx,
		bson.M{"source": source, "country": country, "chart": chart, "genre": genre},
		options.Find().SetSort(bson.D{{Key: "day", Value: -1}}).SetLimit(int64(n)))
	if err != nil {
		return nil, fmt.Errorf("latest charts: %w", err)
	}
	var out []ChartSnapshot
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("latest charts: %w", err)
	}
	return out, nil
}

// LatestChartDay is the day of the newest chart snapshot of a source, ""
// when there is none.
func (s *Store) LatestChartDay(ctx context.Context, source string) (string, error) {
	var c struct {
		Day string `bson:"day"`
	}
	err := s.ChartSnapshots().FindOne(ctx, bson.M{"source": source},
		options.FindOne().SetSort(bson.D{{Key: "fetchedAt", Value: -1}}).SetProjection(bson.M{"day": 1})).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("latest chart day: %w", err)
	}
	return c.Day, nil
}

// StoreCardFields is what a store-chart row carries.
var StoreCardFields = bson.M{
	"externalId": 1, "name": 1, "metrics": 1, "growth": 1, "labels.domain": 1, "assets.iconUrl": 1,
	"app.developer": 1, "app.genreId": 1, "app.currency": 1, "app.homeCountry": 1, "app.contentRating": 1,
	"stale": 1,
}

// StoreCards reads the chart rows of the given store items, keyed by
// external id. Ids with no item are absent.
func (s *Store) StoreCards(ctx context.Context, source string, externalIDs []string) (map[string]item.Item, error) {
	out := make(map[string]item.Item, len(externalIDs))
	if len(externalIDs) == 0 {
		return out, nil
	}
	cur, err := s.Items().Find(ctx, bson.M{"source": source, "externalId": bson.M{"$in": externalIDs}},
		options.Find().SetProjection(StoreCardFields))
	if err != nil {
		return nil, fmt.Errorf("store cards: %w", err)
	}
	var rows []item.Item
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("store cards: %w", err)
	}
	for _, it := range rows {
		out[it.ExternalID] = it
	}
	return out, nil
}

// DomainCounts counts a source's live items per domain label.
func (s *Store) DomainCounts(ctx context.Context, source string) (map[string]int, error) {
	cur, err := s.Items().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"source": source, "stale": bson.M{"$ne": true}}}},
		{{Key: "$unwind", Value: "$labels.domain"}},
		{{Key: "$group", Value: bson.M{"_id": "$labels.domain", "n": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, fmt.Errorf("domain counts: %w", err)
	}
	var rows []struct {
		ID string `bson:"_id"`
		N  int    `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("domain counts: %w", err)
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.ID] = r.N
	}
	return out, nil
}
