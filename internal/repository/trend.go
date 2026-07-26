package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
)

// TrendPoint is one dated star reading — a real snapshot, not an interpolation.
type TrendPoint struct {
	T int64 `json:"t"` // unix milliseconds
	V int   `json:"v"` // stars
}

// SnapshotSeries returns the star time series for each requested repo since
// `from`, ascending by time, keyed by externalId. Repos with no snapshots in the
// window are simply absent from the map (the caller decides whether that's
// enough points to draw). One indexed query covers the whole batch.
func (s *Store) SnapshotSeries(ctx context.Context, src domain.Source, ids []string, from time.Time) (map[string][]TrendPoint, error) {
	if len(ids) == 0 {
		return map[string][]TrendPoint{}, nil
	}
	cur, err := s.Snapshots().Find(ctx,
		bson.M{
			"meta.source":     src,
			"meta.externalId": bson.M{"$in": ids},
			"capturedAt":      bson.M{"$gte": from},
		},
		options.Find().
			SetSort(bson.D{{Key: "capturedAt", Value: 1}}).
			SetProjection(bson.M{"meta.externalId": 1, "capturedAt": 1, "metrics.stars": 1}),
	)
	if err != nil {
		return nil, fmt.Errorf("snapshot series: %w", err)
	}
	var rows []struct {
		Meta       domain.SnapshotMeta `bson:"meta"`
		CapturedAt time.Time           `bson:"capturedAt"`
		Metrics    struct {
			Stars float64 `bson:"stars"`
		} `bson:"metrics"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("snapshot series decode: %w", err)
	}
	out := make(map[string][]TrendPoint, len(ids))
	for _, r := range rows {
		id := r.Meta.ExternalID
		out[id] = append(out[id], TrendPoint{T: r.CapturedAt.UnixMilli(), V: int(r.Metrics.Stars)})
	}
	return out, nil
}
