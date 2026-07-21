package repository

import (
	"context"
	"time"

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

// StalenessBucket groups repositories by how long since their last push.
type StalenessBucket struct {
	Bucket       string  `bson:"_id" json:"bucket"`
	Repos        int     `bson:"repos" json:"repos"`
	MedianStars  float64 `bson:"medianStars" json:"medianStars"`
	MedianIssues float64 `bson:"medianIssues" json:"medianIssues"`
	// Open issues per 1,000 stars: normalises backlog against audience size, so
	// a big project isn't flagged just for being big.
	IssuesPerKStar float64 `json:"issuesPerKStar"`
}

// StaleRepo is a named example of a long-dormant repository.
type StaleRepo struct {
	ExternalID string    `bson:"externalId" json:"externalId"`
	Language   string    `bson:"language" json:"language"`
	Stars      int64     `bson:"stars" json:"stars"`
	OpenIssues int64     `bson:"openIssues" json:"openIssues"`
	PushedAt   time.Time `bson:"pushedAt" json:"pushedAt"`
}

// Boundaries in days. A repo lands in the first bucket whose cutoff it clears.
var stalenessCutoffs = []struct {
	id   string
	days int
}{
	{"active", 90},
	{"slowing", 365},
	{"dormant", 730},
	{"stale", 0}, // everything older
}

// Staleness reports the push-recency distribution.
//
// Median open issues accompanies each bucket on purpose: "not pushed recently"
// alone cannot separate an abandoned project from a finished one. A stable
// library that needs no changes carries a small backlog; an abandoned project
// with users keeps accumulating issues nobody closes.
func (s *Store) Staleness(ctx context.Context, src domain.Source, examples int) ([]StalenessBucket, []StaleRepo, error) {
	now := time.Now().UTC()
	branches := bson.A{}
	for _, c := range stalenessCutoffs {
		if c.days == 0 {
			continue
		}
		branches = append(branches, bson.M{
			"case": bson.M{"$gte": bson.A{"$sourceData.pushedAt", now.AddDate(0, 0, -c.days)}},
			"then": c.id,
		})
	}

	match := bson.M{"source": src, "sourceData.pushedAt": bson.M{"$type": "date"}}
	pipeline := bson.A{
		bson.M{"$match": match},
		bson.M{"$group": bson.M{
			"_id":          bson.M{"$switch": bson.M{"branches": branches, "default": "stale"}},
			"repos":        bson.M{"$sum": 1},
			"medianStars":  bson.M{"$percentile": bson.M{"input": "$metrics.stars", "p": bson.A{0.5}, "method": "approximate"}},
			"medianIssues": bson.M{"$percentile": bson.M{"input": "$metrics.openIssues", "p": bson.A{0.5}, "method": "approximate"}},
		}},
		bson.M{"$set": bson.M{
			"medianStars":  bson.M{"$arrayElemAt": bson.A{"$medianStars", 0}},
			"medianIssues": bson.M{"$arrayElemAt": bson.A{"$medianIssues", 0}},
		}},
	}

	cur, err := s.Items().Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	byID := map[string]StalenessBucket{}
	if err := func() error {
		var rows []StalenessBucket
		if err := cur.All(ctx, &rows); err != nil {
			return err
		}
		for _, r := range rows {
			if r.MedianStars > 0 {
				r.IssuesPerKStar = r.MedianIssues / r.MedianStars * 1000
			}
			byID[r.Bucket] = r
		}
		return nil
	}(); err != nil {
		return nil, nil, err
	}

	// Return buckets in chronological order, including any that came back empty.
	buckets := make([]StalenessBucket, 0, len(stalenessCutoffs))
	for _, c := range stalenessCutoffs {
		b, ok := byID[c.id]
		if !ok {
			b = StalenessBucket{Bucket: c.id}
		}
		buckets = append(buckets, b)
	}

	if examples <= 0 {
		return buckets, nil, nil
	}

	// Most-starred repositories that have gone quiet — the recognisable names.
	exCur, err := s.Items().Aggregate(ctx, bson.A{
		bson.M{"$match": bson.M{
			"source":              src,
			"sourceData.pushedAt": bson.M{"$type": "date", "$lt": now.AddDate(0, 0, -730)},
		}},
		bson.M{"$sort": bson.M{"metrics.stars": -1}},
		bson.M{"$limit": examples},
		bson.M{"$project": bson.M{
			"_id":        0,
			"externalId": 1,
			"language":   1,
			"stars":      "$metrics.stars",
			"openIssues": "$metrics.openIssues",
			"pushedAt":   "$sourceData.pushedAt",
		}},
	})
	if err != nil {
		return nil, nil, err
	}
	defer exCur.Close(ctx)

	repos := []StaleRepo{}
	if err := exCur.All(ctx, &repos); err != nil {
		return nil, nil, err
	}
	return buckets, repos, nil
}
