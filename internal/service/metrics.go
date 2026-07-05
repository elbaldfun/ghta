package service

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// window durations for growth metrics.
const (
	dailyWindow   = 24 * time.Hour
	weeklyWindow  = 7 * 24 * time.Hour
	monthlyWindow = 30 * 24 * time.Hour
	// fetch a little beyond the longest window so a monthly baseline snapshot is found.
	lookback = 35 * 24 * time.Hour
)

const metricsBatch = 500

// MetricsService computes each item's daily/weekly/monthly growth of its primary
// metric from the snapshot time series and backfills the increase fields.
type MetricsService struct {
	store *repository.Store
	log   *slog.Logger
}

func NewMetricsService(store *repository.Store, log *slog.Logger) *MetricsService {
	return &MetricsService{store: store, log: log}
}

// Run recomputes growth metrics for every item. It is source-agnostic: each item
// is measured on its own primaryMetric.
func (s *MetricsService) Run(ctx context.Context) error {
	now := time.Now().UTC()
	cur, err := s.store.Items().Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	var models []mongo.WriteModel
	processed := 0
	flush := func() error {
		if len(models) == 0 {
			return nil
		}
		if _, err := s.store.Items().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false)); err != nil {
			return err
		}
		models = models[:0]
		return nil
	}

	for cur.Next(ctx) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var item domain.TrackedItem
		if err := cur.Decode(&item); err != nil {
			s.log.Warn("metrics: decode item failed", "err", err)
			continue
		}

		current, ok := item.Metrics[item.PrimaryMetric]
		if !ok {
			continue
		}
		history, err := s.snapshotHistory(ctx, item, now.Add(-lookback))
		if err != nil {
			s.log.Warn("metrics: snapshot history failed", "item", item.ExternalID, "err", err)
			continue
		}

		set := bson.M{
			"dailyIncrease":   increaseSince(history, item.PrimaryMetric, current, now.Add(-dailyWindow)),
			"weeklyIncrease":  increaseSince(history, item.PrimaryMetric, current, now.Add(-weeklyWindow)),
			"monthlyIncrease": increaseSince(history, item.PrimaryMetric, current, now.Add(-monthlyWindow)),
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": item.ID}).
			SetUpdate(bson.M{"$set": set}))

		if len(models) >= metricsBatch {
			if err := flush(); err != nil {
				return err
			}
		}
		processed++
	}
	if err := flush(); err != nil {
		return err
	}
	s.log.Info("metrics computed", "items", processed)
	return cur.Err()
}

// snapshotHistory returns an item's snapshots since `from`, ascending by time.
func (s *MetricsService) snapshotHistory(ctx context.Context, item domain.TrackedItem, from time.Time) ([]domain.MetricSnapshot, error) {
	cur, err := s.store.Snapshots().Find(ctx,
		bson.M{
			"meta.source":     item.Source,
			"meta.externalId": item.ExternalID,
			"capturedAt":      bson.M{"$gte": from},
		},
		options.Find().SetSort(bson.D{{Key: "capturedAt", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var snaps []domain.MetricSnapshot
	if err := cur.All(ctx, &snaps); err != nil {
		return nil, err
	}
	return snaps, nil
}

// increaseSince returns current minus the baseline: the most recent snapshot at
// or before startTime. Returns nil when no such baseline exists (item younger
// than the window), so the item is excluded from that window's ranking.
func increaseSince(history []domain.MetricSnapshot, metric string, current float64, startTime time.Time) *float64 {
	var baseline *float64
	for _, snap := range history {
		if snap.CapturedAt.After(startTime) {
			break // history is ascending; nothing more at/before startTime
		}
		if v, ok := snap.Metrics[metric]; ok {
			val := v
			baseline = &val
		}
	}
	if baseline == nil {
		return nil
	}
	inc := current - *baseline
	return &inc
}
