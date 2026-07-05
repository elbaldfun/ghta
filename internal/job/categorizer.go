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
	maxItemsPerRun  = 1000
	maxAnalysisFail = 3
)

// Categorizer assigns categories to unanalyzed items via the AI service, in
// batches to amortize the per-call cost.
type Categorizer struct {
	store     *repository.Store
	ai        *service.AIService
	batchSize int
	log       *slog.Logger
}

func NewCategorizer(store *repository.Store, ai *service.AIService, batchSize int, log *slog.Logger) *Categorizer {
	if batchSize < 1 {
		batchSize = 15
	}
	return &Categorizer{store: store, ai: ai, batchSize: batchSize, log: log}
}

// Run categorizes up to maxItemsPerRun pending items. An item is pending when
// its analysisStatus is neither done nor failed (missing counts as pending), so
// pre-existing documents are still picked up.
func (c *Categorizer) Run(ctx context.Context) {
	cats, err := c.loadCategories(ctx)
	if err != nil {
		c.log.Error("categorizer: load categories failed", "err", err)
		return
	}

	filter := bson.M{"analysisStatus": bson.M{"$nin": []string{domain.AnalysisDone, domain.AnalysisFailed}}}
	cur, err := c.store.Items().Find(ctx, filter, options.Find().SetLimit(maxItemsPerRun))
	if err != nil {
		c.log.Error("categorizer: query failed", "err", err)
		return
	}
	var items []domain.TrackedItem
	if err := cur.All(ctx, &items); err != nil {
		c.log.Error("categorizer: decode failed", "err", err)
		return
	}
	c.log.Info("categorizer starting", "items", len(items), "batchSize", c.batchSize)

	done := 0
	for start := 0; start < len(items); start += c.batchSize {
		if ctx.Err() != nil {
			return
		}
		end := start + c.batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]

		results, created, err := c.ai.AnalyzeBatch(ctx, cats, batch)
		if err != nil {
			// Whole-batch failure: count an attempt against each item.
			c.log.Warn("categorize batch failed", "err", err, "size", len(batch))
			for _, item := range batch {
				c.markFailed(ctx, item)
			}
			continue
		}
		cats = append(cats, created...) // keep the tree cache fresh within the run

		for _, item := range batch {
			r, ok := results[item.ExternalID]
			if !ok {
				c.markFailed(ctx, item)
				continue
			}
			if err := c.markDone(ctx, item.ID, r.CategoryID, r.Path); err != nil {
				c.log.Error("categorize update failed", "item", item.ExternalID, "err", err)
				continue
			}
			done++
		}
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
