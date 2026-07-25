package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/repository"
)

const (
	ecosystemTTL  = time.Hour
	ecosystemTopN = 300
)

// mcpTopics are the author topics that mark an MCP server repo.
var mcpTopics = []string{"mcp", "mcp-server", "model-context-protocol"}

// EcosystemItem is one repo in the "AI stack": things you use to give an AI or
// agent capabilities. Each carries which of the three pillars it belongs to so
// the UI can tag it (a repo can be in more than one).
type EcosystemItem struct {
	ExternalID  string `json:"externalId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	Growth      int    `json:"growth"`
	Type        string `json:"type,omitempty"`
	IsSkill     bool   `json:"isSkill"`
	IsMcp       bool   `json:"isMcp"`
	IsAgent     bool   `json:"isAgent"`
}

// EcosystemService ranks the AI-stack union (skills ∪ MCP servers ∪ agent repos)
// by momentum. Results are cached per pillar+sort.
type EcosystemService struct {
	store *repository.Store
	mu    sync.Mutex
	cache map[string]cachedEco
}

type cachedEco struct {
	rows []EcosystemItem
	at   time.Time
}

func NewEcosystemService(store *repository.Store) *EcosystemService {
	return &EcosystemService{store: store, cache: map[string]cachedEco{}}
}

// pillarFilter returns the mongo filter for one pillar, or the full union for
// "all"/"".
func pillarFilter(pillar string) bson.M {
	skill := bson.M{"type": "skill"}
	mcp := bson.M{"sourceData.topicNames": bson.M{"$in": mcpTopics}}
	agent := bson.M{"categoryPath": "ai/agents"}
	switch pillar {
	case "skill":
		return skill
	case "mcp":
		return mcp
	case "agent":
		return agent
	default:
		return bson.M{"$or": bson.A{skill, mcp, agent}}
	}
}

// Ranking returns one page of a pillar. pillar is all|skill|mcp|agent; sort is
// "hot" (recent star growth — default) or "popular" (total stars).
func (s *EcosystemService) Ranking(ctx context.Context, pillar, sort string, limit, page int) ([]EcosystemItem, int, error) {
	switch pillar {
	case "skill", "mcp", "agent":
	default:
		pillar = "all"
	}
	if sort != "popular" {
		sort = "hot"
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if page < 1 {
		page = 1
	}

	rows, err := s.board(ctx, pillar, sort)
	if err != nil {
		return nil, 0, err
	}
	total := len(rows)
	start := (page - 1) * limit
	if start >= total {
		return []EcosystemItem{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
}

func (s *EcosystemService) board(ctx context.Context, pillar, sort string) ([]EcosystemItem, error) {
	key := pillar + "|" + sort

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cache[key]; ok && time.Since(c.at) < ecosystemTTL {
		return c.rows, nil
	}

	rows, err := s.compute(ctx, pillar, sort)
	if err != nil {
		if c, ok := s.cache[key]; ok {
			return c.rows, nil
		}
		return nil, err
	}
	s.cache[key] = cachedEco{rows: rows, at: time.Now()}
	return rows, nil
}

func (s *EcosystemService) compute(ctx context.Context, pillar, sort string) ([]EcosystemItem, error) {
	sortKey := "growth"
	if sort == "popular" {
		sortKey = "metrics.stars"
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: pillarFilter(pillar)}},
		bson.D{{Key: "$addFields", Value: bson.M{
			"growth":  bson.M{"$ifNull": bson.A{"$dailyIncrease", 0}},
			"isSkill": bson.M{"$eq": bson.A{"$type", "skill"}},
			"isMcp": bson.M{"$gt": bson.A{
				bson.M{"$size": bson.M{"$setIntersection": bson.A{
					bson.M{"$ifNull": bson.A{"$sourceData.topicNames", bson.A{}}}, mcpTopics,
				}}}, 0,
			}},
			"isAgent": bson.M{"$in": bson.A{"ai/agents",
				bson.M{"$cond": bson.A{bson.M{"$isArray": "$categoryPath"}, "$categoryPath", bson.A{}}}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: sortKey, Value: -1}}}},
		bson.D{{Key: "$limit", Value: ecosystemTopN}},
		bson.D{{Key: "$project", Value: bson.M{
			"externalId": 1, "name": 1, "description": 1, "language": 1,
			"stars": "$metrics.stars", "forks": "$metrics.forks",
			"growth": 1, "type": 1, "isSkill": 1, "isMcp": 1, "isAgent": 1,
		}}},
	}

	cur, err := s.store.Items().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("ecosystem aggregate: %w", err)
	}
	var raw []ecoRow
	if err := cur.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("ecosystem decode: %w", err)
	}
	rows := make([]EcosystemItem, 0, len(raw))
	for _, r := range raw {
		rows = append(rows, EcosystemItem{
			ExternalID:  r.ExternalID,
			Name:        r.Name,
			Description: r.Description,
			Language:    r.Language,
			Stars:       int(r.Stars),
			Forks:       int(r.Forks),
			Growth:      int(r.Growth),
			Type:        r.Type,
			IsSkill:     r.IsSkill,
			IsMcp:       r.IsMcp,
			IsAgent:     r.IsAgent,
		})
	}
	return rows, nil
}

type ecoRow struct {
	ExternalID  string  `bson:"externalId"`
	Name        string  `bson:"name"`
	Description string  `bson:"description"`
	Language    string  `bson:"language"`
	Stars       float64 `bson:"stars"`
	Forks       float64 `bson:"forks"`
	Growth      float64 `bson:"growth"`
	Type        string  `bson:"type"`
	IsSkill     bool    `bson:"isSkill"`
	IsMcp       bool    `bson:"isMcp"`
	IsAgent     bool    `bson:"isAgent"`
}
