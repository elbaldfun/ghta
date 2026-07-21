package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
)

// LanguageStat aggregates the tracked corpus for one programming language.
type LanguageStat struct {
	Language    string  `bson:"_id" json:"language"`
	Repos       int     `bson:"repos" json:"repos"`
	TotalStars  int64   `bson:"totalStars" json:"totalStars"`
	MedianStars float64 `bson:"medianStars" json:"medianStars"`
	TopRepo     string  `bson:"topRepo" json:"topRepo"`
	TopStars    int64   `bson:"topStars" json:"topStars"`
}

// LanguageStats returns per-language totals for a source, most repos first.
//
// The median comes from $percentile rather than the mean: star counts are
// heavily skewed by a handful of giant repos, so an average would say more
// about the outliers than about a typical project in that language.
func (s *Store) LanguageStats(ctx context.Context, src domain.Source, limit int) ([]LanguageStat, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"source":   src,
			"language": bson.M{"$nin": bson.A{nil, ""}},
		}},
		bson.M{"$group": bson.M{
			"_id":        "$language",
			"repos":      bson.M{"$sum": 1},
			"totalStars": bson.M{"$sum": "$metrics.stars"},
			"medianStars": bson.M{"$percentile": bson.M{
				"input":  "$metrics.stars",
				"p":      bson.A{0.5},
				"method": "approximate",
			}},
			"topStars": bson.M{"$max": "$metrics.stars"},
			"top": bson.M{"$topN": bson.M{
				"n":      1,
				"sortBy": bson.M{"metrics.stars": -1},
				"output": "$externalId",
			}},
		}},
		// $percentile yields a single-element array; $topN likewise.
		bson.M{"$set": bson.M{
			"medianStars": bson.M{"$arrayElemAt": bson.A{"$medianStars", 0}},
			"topRepo":     bson.M{"$arrayElemAt": bson.A{"$top", 0}},
		}},
		bson.M{"$sort": bson.M{"repos": -1}},
		bson.M{"$limit": limit},
		bson.M{"$unset": "top"},
	}

	cur, err := s.Items().Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []LanguageStat{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
