package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/repository"
)

const (
	topicRankTTL   = time.Hour
	topicTopN      = 60
	topicRepoFloor = 5 // drop topics on fewer than this many repos (noise)
)

// HotTopic is one row of the "hot topics in a domain" board: an author topic
// (e.g. "llm", "mcp") with how many repos carry it, their combined stars, and —
// the interesting signal — how fast those repos are gaining stars right now.
type HotTopic struct {
	Topic        string  `json:"topic"`
	RepoCount    int     `json:"repoCount"`
	TotalStars   int     `json:"totalStars"`
	Growth       int     `json:"growth"` // Σ daily star gain across repos with this topic
	TopRepo      string  `json:"topRepo,omitempty"`
	TopRepoStars int     `json:"topRepoStars"`
	Score        float64 `json:"score"`
}

// TopicService ranks author topics within a classification domain. Cached per
// domain+sort.
type TopicService struct {
	store *repository.Store
	cache *ttlCache[[]HotTopic]
}

func NewTopicService(store *repository.Store) *TopicService {
	return &TopicService{store: store, cache: newTTLCache[[]HotTopic](topicRankTTL)}
}

// Ranking returns hot topics for a domain. sort "hot" (recent growth — default,
// "what's rising") or "popular" (repo count). domain defaults to "ai" at the
// caller; "" spans all domains.
func (s *TopicService) Ranking(ctx context.Context, domain, sort string, limit int) ([]HotTopic, error) {
	if sort != "popular" {
		sort = "hot"
	}
	if domain == "all" {
		domain = ""
	}
	if limit <= 0 || limit > topicTopN {
		limit = topicTopN
	}

	rows, err := s.board(ctx, domain, sort)
	if err != nil {
		return nil, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (s *TopicService) board(ctx context.Context, domain, sort string) ([]HotTopic, error) {
	return s.cache.get(ctx, domain+"|"+sort, func(ctx context.Context) ([]HotTopic, error) {
		return s.compute(ctx, domain, sort)
	})
}

func (s *TopicService) compute(ctx context.Context, domain, sort string) ([]HotTopic, error) {
	pipeline := mongo.Pipeline{liveMatch}
	if domain != "" {
		// Anchor on "domain/" so a domain never over-matches a sibling whose
		// name shares its prefix (e.g. "ml" must not match "mlops/…").
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
			"categoryPath": bson.M{"$regex": "^" + regexp.QuoteMeta(domain) + "/"},
		}}})
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$unwind", Value: "$sourceData.topicNames"}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":        "$sourceData.topicNames",
			"repoCount":  bson.M{"$sum": 1},
			"totalStars": bson.M{"$sum": "$metrics.stars"},
			"growth":     bson.M{"$sum": bson.M{"$ifNull": bson.A{"$dailyIncrease", 0}}},
			"top": bson.M{"$top": bson.M{
				"output": bson.M{"ext": "$externalId", "stars": "$metrics.stars"},
				"sortBy": bson.M{"metrics.stars": -1},
			}},
		}}},
		bson.D{{Key: "$match", Value: bson.M{"repoCount": bson.M{"$gte": topicRepoFloor}}}},
	)

	sortKey := "growth"
	if sort == "popular" {
		sortKey = "repoCount"
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: sortKey, Value: -1}}}},
		bson.D{{Key: "$limit", Value: topicTopN}},
	)

	cur, err := s.store.Items().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("topic ranking aggregate: %w", err)
	}
	var raw []topicRow
	if err := cur.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("topic ranking decode: %w", err)
	}

	rows := make([]HotTopic, 0, len(raw))
	for _, r := range raw {
		score := r.Growth
		if sort == "popular" {
			score = float64(r.RepoCount)
		}
		rows = append(rows, HotTopic{
			Topic:        r.Topic,
			RepoCount:    r.RepoCount,
			TotalStars:   int(r.TotalStars),
			Growth:       int(r.Growth),
			TopRepo:      r.Top.Ext,
			TopRepoStars: int(r.Top.Stars),
			Score:        score,
		})
	}
	return rows, nil
}

type topicRow struct {
	Topic      string  `bson:"_id"`
	RepoCount  int     `bson:"repoCount"`
	TotalStars float64 `bson:"totalStars"`
	Growth     float64 `bson:"growth"`
	Top        struct {
		Ext   string  `bson:"ext"`
		Stars float64 `bson:"stars"`
	} `bson:"top"`
}
