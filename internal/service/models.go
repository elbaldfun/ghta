package service

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/source/huggingface"
)

const (
	modelRankTTL = time.Hour
	modelTopN    = 300
)

// validTask whitelists the task-group filter (bounds the cache key space).
var validTask = func() map[string]bool {
	m := make(map[string]bool, len(huggingface.TaskGroups))
	for _, g := range huggingface.TaskGroups {
		m[g] = true
	}
	return m
}()

// ModelItem is one row of the HF model boards.
type ModelItem struct {
	ExternalID   string     `json:"externalId"` // "author/model"
	Author       string     `json:"author,omitempty"`
	Name         string     `json:"name"`
	Task         string     `json:"task,omitempty"` // task group (hf/<task>)
	PipelineTag  string     `json:"pipelineTag,omitempty"`
	Library      string     `json:"library,omitempty"`
	License      string     `json:"license,omitempty"`
	Likes        int        `json:"likes"`
	Downloads30d int64      `json:"downloads30d"`
	Growth       int        `json:"growth"` // daily likes gain (velocity axis)
	Gated        bool       `json:"gated"`
	QuantFormats []string   `json:"quantFormats,omitempty"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
	LastModified *time.Time `json:"lastModified,omitempty"`
	URL          string     `json:"url"`
}

// ModelService ranks the HuggingFace model corpus. Cached per task+sort.
type ModelService struct {
	store *repository.Store
	cache *ttlCache[[]ModelItem]
}

func NewModelService(store *repository.Store) *ModelService {
	return &ModelService{store: store, cache: newTTLCache[[]ModelItem](modelRankTTL)}
}

// Ranking returns one page of a model board. task filters to a group; sort is
// hot (likes velocity, default) | downloads (30d adoption) | likes | new.
func (s *ModelService) Ranking(ctx context.Context, task, sort string, limit, page int) ([]ModelItem, int, error) {
	if !validTask[task] {
		task = ""
	}
	switch sort {
	case "downloads", "likes", "new":
	default:
		sort = "hot"
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if page < 1 {
		page = 1
	}

	rows, err := s.board(ctx, task, sort)
	if err != nil {
		return nil, 0, err
	}
	total := len(rows)
	start := (page - 1) * limit
	if start >= total {
		return []ModelItem{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
}

func (s *ModelService) board(ctx context.Context, task, sort string) ([]ModelItem, error) {
	return s.cache.get(ctx, task+"|"+sort, func(ctx context.Context) ([]ModelItem, error) {
		return s.compute(ctx, task, sort)
	})
}

func (s *ModelService) compute(ctx context.Context, task, sort string) ([]ModelItem, error) {
	filter := bson.M{"source": domain.SourceHuggingFace}
	if task != "" {
		filter["categoryPath"] = "hf/" + task
	}

	var sortKey bson.D
	switch sort {
	case "downloads":
		sortKey = bson.D{{Key: "metrics.downloads30d", Value: -1}}
	case "likes":
		sortKey = bson.D{{Key: "metrics.likes", Value: -1}}
	case "new":
		sortKey = bson.D{{Key: "sourceData.createdAt", Value: -1}}
	default: // hot: daily likes velocity
		filter["dailyIncrease"] = bson.M{"$gt": 0}
		sortKey = bson.D{{Key: "dailyIncrease", Value: -1}}
	}

	items, err := s.query(ctx, filter, sortKey)
	if err != nil {
		return nil, err
	}
	// Bootstrap: likes velocity needs two days of snapshots. Until increments
	// exist, rank "hot" by HF's own trendingScore so the default board is never
	// empty; the real velocity axis takes over automatically once data accrues.
	if sort == "hot" && len(items) == 0 {
		delete(filter, "dailyIncrease")
		items, err = s.query(ctx, filter, bson.D{{Key: "sourceData.trendingScore", Value: -1}})
		if err != nil {
			return nil, err
		}
	}

	rows := make([]ModelItem, 0, len(items))
	for _, it := range items {
		rows = append(rows, mapModelItem(it))
	}
	return rows, nil
}

func (s *ModelService) query(ctx context.Context, filter bson.M, sortKey bson.D) ([]domain.TrackedItem, error) {
	cur, err := s.store.Items().Find(ctx, filter,
		options.Find().
			SetSort(sortKey).
			SetLimit(modelTopN).
			SetProjection(bson.M{
				"externalId": 1, "name": 1, "metrics": 1, "dailyIncrease": 1, "sourceData": 1,
			}))
	if err != nil {
		return nil, fmt.Errorf("model board query: %w", err)
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		return nil, fmt.Errorf("model board decode: %w", err)
	}
	return items, nil
}

func mapModelItem(it domain.TrackedItem) ModelItem {
	sd := it.SourceData
	str := func(k string) string { v, _ := sd[k].(string); return v }
	boolean := func(k string) bool { v, _ := sd[k].(bool); return v }
	t := func(k string) *time.Time {
		switch v := sd[k].(type) {
		case time.Time:
			return &v
		case primitiveDateTimeLike:
			tt := v.Time()
			return &tt
		}
		return nil
	}
	var quant []string
	if raw, ok := sd["quantFormats"].(bson.A); ok {
		for _, q := range raw {
			if s, ok := q.(string); ok {
				quant = append(quant, s)
			}
		}
	}
	growth := 0
	if it.DailyIncrease != nil {
		growth = int(*it.DailyIncrease)
	}
	url := str("url")
	if url == "" {
		url = "https://huggingface.co/" + it.ExternalID
	}
	return ModelItem{
		ExternalID:   it.ExternalID,
		Author:       str("author"),
		Name:         it.Name,
		Task:         str("task"),
		PipelineTag:  str("pipelineTag"),
		Library:      str("library"),
		License:      str("license"),
		Likes:        int(it.Metrics["likes"]),
		Downloads30d: int64(it.Metrics["downloads30d"]),
		Growth:       growth,
		Gated:        boolean("gated"),
		QuantFormats: quant,
		CreatedAt:    t("createdAt"),
		LastModified: t("lastModified"),
		URL:          url,
	}
}

// primitiveDateTimeLike matches bson primitive.DateTime without importing the
// package at every call site.
type primitiveDateTimeLike interface{ Time() time.Time }
