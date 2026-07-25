package service

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// developerRankTTL bounds how often the (expensive) owner aggregation runs. The
// board only changes as the daily fetch/devsync move numbers, so hourly is fine.
const developerRankTTL = time.Hour

// rankTopN caps how deep each board is computed/cached: nobody browses past a
// few hundred, and it bounds both the aggregation and memory.
const rankTopN = 500

// risingStarFloor keeps the rising board from being topped by a brand-new repo
// with a one-week spike and no track record.
const risingStarFloor = 2000

// RankedDeveloper is one row of a "developers to follow" board: an individual
// (never an Organization) ranked by the real accomplishment of the repos they
// own, with their self-declared X handle attached purely as context.
type RankedDeveloper struct {
	Login           string  `json:"login"`
	Name            string  `json:"name,omitempty"`
	TwitterUsername string  `json:"twitterUsername,omitempty"`
	Followers       int     `json:"followers"`
	Company         string  `json:"company,omitempty"`
	AvatarURL       string  `json:"avatarUrl,omitempty"`
	Bio             string  `json:"bio,omitempty"`
	RepoCount       int     `json:"repoCount"`
	TotalStars      int     `json:"totalStars"`
	TopRepo         string  `json:"topRepo,omitempty"`
	TopRepoStars    int     `json:"topRepoStars"`
	Growth          int     `json:"growth"`         // recent star gain (rising board)
	Score           float64 `json:"score"`          // the board's sort value
}

// DeveloperService ranks repo owners by merit. Results are cached per board+domain.
type DeveloperService struct {
	store *repository.Store
	mu    sync.Mutex
	cache map[string]cachedRank
}

type cachedRank struct {
	rows []RankedDeveloper
	at   time.Time
}

func NewDeveloperService(store *repository.Store) *DeveloperService {
	return &DeveloperService{store: store, cache: map[string]cachedRank{}}
}

// Ranking returns one page of a board. board is "rising" (recent growth of owned
// repos, with a quality floor — the default, dynamic) or "merit" (log-summed
// stars across owned repos, rewarding a portfolio over one lucky hit). domain
// filters to a classification subtree (e.g. "ai" or "ai/llm"); "" or "all" spans
// every domain. Organizations are always excluded.
func (s *DeveloperService) Ranking(ctx context.Context, board, domain string, limit, page int) ([]RankedDeveloper, int, error) {
	if board != "merit" {
		board = "rising"
	}
	if domain == "all" {
		domain = ""
	}
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	if page < 1 {
		page = 1
	}

	rows, err := s.board(ctx, board, domain)
	if err != nil {
		return nil, 0, err
	}
	total := len(rows)
	start := (page - 1) * limit
	if start >= total {
		return []RankedDeveloper{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
}

// board returns the full cached top-N list for a board+domain, recomputing past
// the TTL.
func (s *DeveloperService) board(ctx context.Context, board, domain string) ([]RankedDeveloper, error) {
	key := board + "|" + domain

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cache[key]; ok && time.Since(c.at) < developerRankTTL {
		return c.rows, nil
	}

	rows, err := s.compute(ctx, board, domain)
	if err != nil {
		if c, ok := s.cache[key]; ok {
			return c.rows, nil // serve stale rather than fail
		}
		return nil, err
	}
	s.cache[key] = cachedRank{rows: rows, at: time.Now()}
	return rows, nil
}

func (s *DeveloperService) compute(ctx context.Context, board, domain string) ([]RankedDeveloper, error) {
	pipeline := mongo.Pipeline{}

	// Only count the owner's repos within the requested domain, so "AI builders"
	// ranks people by their AI work, not their unrelated projects.
	if domain != "" {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
			"categoryPath": bson.M{"$regex": "^" + regexp.QuoteMeta(domain)},
		}}})
	}

	pipeline = append(pipeline,
		bson.D{{Key: "$group", Value: bson.M{
			"_id":        "$sourceData.owner",
			"totalStars": bson.M{"$sum": "$metrics.stars"},
			"meritScore": bson.M{"$sum": bson.M{"$log10": bson.M{"$cond": bson.A{
				bson.M{"$gt": bson.A{"$metrics.stars", 1}}, "$metrics.stars", 1,
			}}}},
			"growth":    bson.M{"$sum": bson.M{"$ifNull": bson.A{"$dailyIncrease", 0}}},
			"repoCount": bson.M{"$sum": 1},
			"top": bson.M{"$top": bson.M{
				"output": bson.M{"ext": "$externalId", "stars": "$metrics.stars"},
				"sortBy": bson.M{"metrics.stars": -1},
			}},
		}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": repository.CollDevelopers, "localField": "_id", "foreignField": "login", "as": "d",
		}}},
		bson.D{{Key: "$addFields", Value: bson.M{"d": bson.M{"$arrayElemAt": bson.A{"$d", 0}}}}},
		bson.D{{Key: "$match", Value: bson.M{"d.type": domain2type()}}},
	)

	sortKey := "meritScore"
	if board == "rising" {
		sortKey = "growth"
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{
			"totalStars": bson.M{"$gte": risingStarFloor},
		}}})
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: sortKey, Value: -1}}}},
		bson.D{{Key: "$limit", Value: rankTopN}},
	)

	cur, err := s.store.Items().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("developer ranking aggregate: %w", err)
	}
	var raw []rankRow
	if err := cur.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("developer ranking decode: %w", err)
	}

	rows := make([]RankedDeveloper, 0, len(raw))
	for _, r := range raw {
		score := r.MeritScore
		if board == "rising" {
			score = r.Growth
		}
		rows = append(rows, RankedDeveloper{
			Login:           r.Login,
			Name:            r.Dev.Name,
			TwitterUsername: r.Dev.TwitterUsername,
			Followers:       r.Dev.Followers,
			Company:         r.Dev.Company,
			AvatarURL:       r.Dev.AvatarURL,
			Bio:             r.Dev.Bio,
			RepoCount:       r.RepoCount,
			TotalStars:      int(r.TotalStars),
			TopRepo:         r.Top.Ext,
			TopRepoStars:    int(r.Top.Stars),
			Growth:          int(r.Growth),
			Score:           score,
		})
	}
	return rows, nil
}

// domain2type is a tiny indirection so the intent (only individuals rank) is
// obvious at the $match site.
func domain2type() string { return domain.DeveloperUser }

type rankRow struct {
	Login      string  `bson:"_id"`
	TotalStars float64 `bson:"totalStars"`
	MeritScore float64 `bson:"meritScore"`
	Growth     float64 `bson:"growth"`
	RepoCount  int     `bson:"repoCount"`
	Top        struct {
		Ext   string  `bson:"ext"`
		Stars float64 `bson:"stars"`
	} `bson:"top"`
	Dev struct {
		Name            string `bson:"name"`
		TwitterUsername string `bson:"twitterUsername"`
		Followers       int    `bson:"followers"`
		Company         string `bson:"company"`
		AvatarURL       string `bson:"avatarUrl"`
		Bio             string `bson:"bio"`
	} `bson:"d"`
}
