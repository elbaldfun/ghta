package service

import (
	"context"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// bestOfN caps a collection page — a "best of" list that runs past a dozen
// stops being a recommendation.
const bestOfN = 12

// qualityScore is the explainable ranking behind /apps/best/* (change 15 §6):
// magnitude (log-scaled stars), momentum (log-scaled daily growth), and two
// freshness/deliverability bonuses. Constants live here, not in a pipeline, so
// the formula is reviewable in one place.
func qualityScore(stars, growth float64, latestRelease *time.Time, hasAssets bool) float64 {
	score := lnPlus(stars)      // ~12 at 100k stars
	score += 2 * lnPlus(growth) // momentum counts double per unit of ln
	if latestRelease != nil && time.Since(*latestRelease) < 90*24*time.Hour {
		score += 2 // actively shipped within a quarter
	}
	if hasAssets {
		score += 1 // something to actually download
	}
	return score
}

func lnPlus(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Log(v + 1)
}

// BestOf returns the quality-ranked collection for one full shelf slug.
func (s *AppService) BestOf(ctx context.Context, shelf string) ([]AppItem, error) {
	if !ValidShelfSlug(shelf) || shelf == StoreExcluded {
		return nil, badInput("unknown shelf")
	}
	return s.cache.get(ctx, "best|"+shelf, func(ctx context.Context) ([]AppItem, error) {
		match := appMatch("", "", "", shelf)
		pipeline := mongo.Pipeline{
			bson.D{{Key: "$match", Value: match}},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "metrics.stars", Value: -1}}}},
			bson.D{{Key: "$limit", Value: 60}}, // score the shelf's head, not the tail
			bson.D{{Key: "$project", Value: appProjection()}},
		}
		cur, err := s.store.Items().Aggregate(ctx, pipeline)
		if err != nil {
			return nil, err
		}
		var raw []appRow
		if err := cur.All(ctx, &raw); err != nil {
			return nil, err
		}
		rows := appItemsFromRows(raw)
		sort.SliceStable(rows, func(i, j int) bool {
			return itemScore(rows[i]) > itemScore(rows[j])
		})
		if len(rows) > bestOfN {
			rows = rows[:bestOfN]
		}
		return rows, nil
	})
}

func itemScore(a AppItem) float64 {
	return qualityScore(float64(a.Stars), float64(a.Growth), a.LatestReleaseAt, a.HasDownloads)
}
