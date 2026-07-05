package job

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
)

const (
	categorizeBatch = 1000
	maxAnalysisFail = 3
)

// Categorizer assigns categories to unanalyzed items via the AI service.
type Categorizer struct {
	store *repository.Store
	ai    *service.AIService
	log   *slog.Logger
}

func NewCategorizer(store *repository.Store, ai *service.AIService, log *slog.Logger) *Categorizer {
	return &Categorizer{store: store, ai: ai, log: log}
}

// Run categorizes up to categorizeBatch pending items. An item is pending when
// it has no categoryId field or an empty one — so pre-existing documents that
// never had the field are still picked up.
func (c *Categorizer) Run(ctx context.Context) {
	cats, err := c.loadCategories(ctx)
	if err != nil {
		c.log.Error("categorizer: load categories failed", "err", err)
		return
	}

	filter := bson.M{
		"analysisStatus": bson.M{"$ne": domain.AnalysisFailed},
		"$or": []bson.M{
			{"categoryId": bson.M{"$exists": false}},
			{"categoryId": bson.M{"$size": 0}},
		},
	}
	cur, err := c.store.Items().Find(ctx, filter, options.Find().SetLimit(categorizeBatch))
	if err != nil {
		c.log.Error("categorizer: query failed", "err", err)
		return
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		c.log.Error("categorizer: decode failed", "err", err)
		return
	}
	c.log.Info("categorizer starting", "items", len(items))

	done := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		catID, path, err := c.ai.AnalyzeItem(ctx, cats, item)
		if err != nil {
			c.markFailed(ctx, item)
			c.log.Warn("categorize failed", "item", item.ExternalID, "err", err)
			continue
		}
		if err := c.markDone(ctx, item.ID, catID, path); err != nil {
			c.log.Error("categorize update failed", "item", item.ExternalID, "err", err)
			continue
		}
		done++
	}
	c.log.Info("categorizer done", "analyzed", done, "of", len(items))
}

func (c *Categorizer) loadCategories(ctx context.Context) ([]domain.Category, error) {
	cur, err := c.store.Categories().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var cats []domain.Category
	if err := cur.All(ctx, &cats); err != nil {
		return nil, err
	}
	return cats, nil
}

func (c *Categorizer) markDone(ctx context.Context, id interface{}, catID, path string) error {
	catIDs := []string{}
	if catID != "" {
		catIDs = []string{catID}
	}
	_, err := c.store.Items().UpdateByID(ctx, id, bson.M{"$set": bson.M{
		"categoryId":     catIDs,
		"categoryPath":   path,
		"analysisStatus": domain.AnalysisDone,
	}})
	return err
}

func (c *Categorizer) markFailed(ctx context.Context, item domain.TrackedItem) {
	newCount := item.AnalysisFailCount + 1
	set := bson.M{"analysisFailCount": newCount}
	if newCount >= maxAnalysisFail {
		set["analysisStatus"] = domain.AnalysisFailed
	}
	_, _ = c.store.Items().UpdateByID(ctx, item.ID, bson.M{"$set": set})
}
