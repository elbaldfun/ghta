package job

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/service"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

const (
	maxItemsPerRun  = 1000
	maxAnalysisFail = 3
)

// Categorizer assigns categories to unanalyzed items via a three-tier pipeline
// ordered by cost: (1) topic-map rules, (2) embedding similarity, (3) LLM batch.
// Each tier only sees what the previous tiers left unresolved.
type Categorizer struct {
	store     *repository.Store
	rules     *taxonomy.Rules
	embed     *service.EmbedClassifier
	ai        *service.AIService
	batchSize int
	log       *slog.Logger
}

func NewCategorizer(store *repository.Store, rules *taxonomy.Rules, embed *service.EmbedClassifier, ai *service.AIService, batchSize int, log *slog.Logger) *Categorizer {
	if batchSize < 1 {
		batchSize = 15
	}
	return &Categorizer{store: store, rules: rules, embed: embed, ai: ai, batchSize: batchSize, log: log}
}

// Run categorizes up to maxItemsPerRun pending items.
func (c *Categorizer) Run(ctx context.Context) {
	cats, idByPath, err := c.loadTaxonomy(ctx)
	if err != nil {
		c.log.Error("categorizer: load taxonomy failed", "err", err)
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

	// Tier 1: rule mapping (free, deterministic, multi-label).
	var unresolved []domain.TrackedItem
	ruleDone := 0
	for _, item := range items {
		paths := c.rules.Classify(topicNames(item), item.Language)
		if len(paths) == 0 {
			unresolved = append(unresolved, item)
			continue
		}
		ids := make([]string, 0, len(paths))
		for _, p := range paths {
			if id, ok := idByPath[p]; ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 { // mapped paths missing from the tree: treat as unresolved
			unresolved = append(unresolved, item)
			continue
		}
		if err := c.markDone(ctx, item.ID, ids, paths[0], "rule"); err == nil {
			ruleDone++
		}
	}

	// Tier 2: embedding similarity (skipped when no backend is configured).
	embedDone := 0
	if c.embed.Enabled() && len(unresolved) > 0 {
		next := unresolved[:0]
		results, err := c.embed.Classify(ctx, cats, unresolved)
		if err != nil {
			c.log.Warn("categorizer: embedding tier failed, falling through to LLM", "err", err)
		} else {
			for _, item := range unresolved {
				r, ok := results[item.ExternalID]
				if !ok {
					next = append(next, item)
					continue
				}
				if err := c.markDone(ctx, item.ID, []string{r.CategoryID}, r.Path, "embedding"); err == nil {
					embedDone++
				}
			}
			unresolved = next
		}
	}

	// Tier 3: LLM batches for the long tail.
	llmDone := c.runLLMTier(ctx, cats, idByPath, unresolved)

	c.log.Info("categorizer done",
		"items", len(items), "rule", ruleDone, "embedding", embedDone, "llm", llmDone)
}

func (c *Categorizer) runLLMTier(ctx context.Context, cats []domain.Category, idByPath map[string]string, items []domain.TrackedItem) int {
	done := 0
	for start := 0; start < len(items); start += c.batchSize {
		if ctx.Err() != nil {
			return done
		}
		end := start + c.batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[start:end]

		results, err := c.ai.AnalyzeBatch(ctx, cats, batch)
		if err != nil {
			c.log.Warn("categorize batch failed", "err", err, "size", len(batch))
			for _, item := range batch {
				c.markFailed(ctx, item)
			}
			continue
		}
		for _, item := range batch {
			r, ok := results[item.ExternalID]
			if !ok {
				c.markFailed(ctx, item)
				continue
			}
			id := r.CategoryID
			if id == "" { // model returned only a path
				id = idByPath[r.Path]
			}
			if id == "" {
				c.markFailed(ctx, item)
				continue
			}
			if err := c.markDone(ctx, item.ID, []string{id}, r.Path, "llm"); err != nil {
				c.log.Error("categorize update failed", "item", item.ExternalID, "err", err)
				continue
			}
			done++
		}
	}
	return done
}

// loadTaxonomy returns the frozen tree (createdBy=taxonomy only — legacy
// AI-created categories are not assignment targets) plus a path→id index.
func (c *Categorizer) loadTaxonomy(ctx context.Context) ([]domain.Category, map[string]string, error) {
	cur, err := c.store.Categories().Find(ctx, bson.M{"createdBy": "taxonomy"})
	if err != nil {
		return nil, nil, err
	}
	var cats []domain.Category
	if err := cur.All(ctx, &cats); err != nil {
		return nil, nil, err
	}
	idByPath := make(map[string]string, len(cats))
	for _, cat := range cats {
		idByPath[cat.Path] = cat.ID.Hex()
	}
	return cats, idByPath, nil
}

func topicNames(item domain.TrackedItem) []string {
	raw, ok := item.SourceData["topicNames"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case primitive.A:
		return anyToStrings(v)
	case []interface{}:
		return anyToStrings(v)
	}
	return nil
}

func anyToStrings(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, x := range in {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (c *Categorizer) markDone(ctx context.Context, id interface{}, catIDs []string, path, by string) error {
	_, err := c.store.Items().UpdateByID(ctx, id, bson.M{"$set": bson.M{
		"categoryId":     catIDs,
		"categoryPath":   path,
		"analysisStatus": domain.AnalysisDone,
		"classifiedBy":   by,
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
