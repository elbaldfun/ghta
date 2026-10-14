package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// ErrNoItem is returned for an item that is not tracked.
var ErrNoItem = errors.New("item not tracked")

// Item returns one item without its history ring, which has its own read.
func (s *Store) Item(ctx context.Context, source, externalID string) (item.Item, error) {
	var it item.Item
	err := s.Items().FindOne(ctx, bson.M{"source": source, "externalId": externalID},
		options.FindOne().SetProjection(bson.M{"history": 0})).Decode(&it)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return it, ErrNoItem
	}
	if err != nil {
		return it, fmt.Errorf("item %s: %w", externalID, err)
	}
	return it, nil
}

// Histories returns the history ring of each of the given items that has
// one: the last 35 daily closing values.
func (s *Store) Histories(ctx context.Context, source string, ids []string) (map[string][]item.Point, error) {
	cur, err := s.Items().Find(ctx, bson.M{"source": source, "externalId": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"externalId": 1, "history": 1}))
	if err != nil {
		return nil, fmt.Errorf("histories: %w", err)
	}
	var rows []struct {
		ExternalID string       `bson:"externalId"`
		History    []item.Point `bson:"history"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("histories: %w", err)
	}
	out := make(map[string][]item.Point, len(rows))
	for _, r := range rows {
		if len(r.History) > 0 {
			out[r.ExternalID] = r.History
		}
	}
	return out, nil
}

// Reading is one metric snapshot's value.
type Reading struct {
	At time.Time
	V  float64
}

// MetricSeries returns every snapshot of one metric of an item since from,
// oldest first: the long curve the history ring is too short for.
func (s *Store) MetricSeries(ctx context.Context, source, externalID, metric string, from time.Time) ([]Reading, error) {
	cur, err := s.Snapshots().Find(ctx,
		bson.M{"meta.source": source, "meta.externalId": externalID, "capturedAt": bson.M{"$gte": from}},
		options.Find().SetSort(bson.D{{Key: "capturedAt", Value: 1}}).
			SetProjection(bson.M{"capturedAt": 1, "metrics." + metric: 1}))
	if err != nil {
		return nil, fmt.Errorf("metric series %s: %w", externalID, err)
	}
	var rows []struct {
		CapturedAt time.Time          `bson:"capturedAt"`
		Metrics    map[string]float64 `bson:"metrics"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("metric series %s: %w", externalID, err)
	}
	out := make([]Reading, 0, len(rows))
	for _, r := range rows {
		if v, ok := r.Metrics[metric]; ok {
			out = append(out, Reading{At: r.CapturedAt, V: v})
		}
	}
	return out, nil
}
