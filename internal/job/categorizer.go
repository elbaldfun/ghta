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
	maxItemsPerRun   = 1000
	maxAnalysisFail  = 3
	defaultMaxLabels = 3
)

// Categorizer assigns each pending item a form type (facet) plus one or more
// domain category paths (multi-label). Type comes from the deterministic facet
// rules; domain runs a two-tier pipeline ordered by cost: (1) topic-map rules,
// (2) LLM batch for the long tail (the embedding tier was dropped — see
// change 12 eval-baseline: it resolved at ~31% accuracy, below the LLM).
type Categorizer struct {
	store     *repository.Store
	rules     *taxonomy.Rules
	facets    *taxonomy.Facets
	ai        *service.AIService
	batchSize int
	maxLabels int
	log       *slog.Logger
}

func NewCategorizer(store *repository.Store, rules *taxonomy.Rules, facets *taxonomy.Facets, ai *service.AIService, batchSize, maxLabels int, log *slog.Logger) *Categorizer {
	if batchSize < 1 {
		batchSize = 15
	}
	if maxLabels < 1 {
		maxLabels = defaultMaxLabels
	}
	return &Categorizer{store: store, rules: rules, facets: facets, ai: ai, batchSize: batchSize, maxLabels: maxLabels, log: log}
}

// pending pairs an item with its deterministically-derived form type, carried
// into the LLM tier so a resource-class item that comes up empty on domain is
// still marked done (not failed) with its type.
type pending struct {
	item  domain.TrackedItem
	ftype string
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

	// Tier 1: topic-map rules (free, deterministic, multi-label). Type is the
	// deterministic facet in all cases.
	var unresolved []pending
	ruleDone := 0
	for _, item := range items {
		ftype := c.facets.ClassifyType(item.ExternalID, topicNames(item))
		paths := c.rules.Classify(topicNames(item), item.Language)
		ids := c.resolveIDs(paths, idByPath)
		if len(ids.paths) == 0 { // no rule hit, or hits missing from tree
			unresolved = append(unresolved, pending{item: item, ftype: ftype})
			continue
		}
		if err := c.markDone(ctx, item.ID, ids.ids, ids.paths, ftype, nil, "rule"); err == nil {
			ruleDone++
		}
	}

	// Tier 2: LLM batches for the long tail (domain + refined type + tags).
	llmDone := c.runLLMTier(ctx, cats, idByPath, unresolved)

	c.log.Info("categorizer done", "items", len(items), "rule", ruleDone, "llm", llmDone)
}

// resolved holds capped domain paths and their category ids, index-aligned.
type resolved struct {
	paths []string
	ids   []string
}

// resolveIDs keeps only paths present in the tree, dedups, and caps at maxLabels.
func (c *Categorizer) resolveIDs(paths []string, idByPath map[string]string) resolved {
	var out resolved
	seen := map[string]struct{}{}
	for _, p := range paths {
		id, ok := idByPath[p]
		if !ok {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out.paths = append(out.paths, p)
		out.ids = append(out.ids, id)
		if len(out.paths) >= c.maxLabels {
			break
		}
	}
	return out
}

func (c *Categorizer) runLLMTier(ctx context.Context, cats []domain.Category, idByPath map[string]string, items []pending) int {
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

		trackedBatch := make([]domain.TrackedItem, len(batch))
		for i, p := range batch {
			trackedBatch[i] = p.item
		}
		results, err := c.ai.AnalyzeBatch(ctx, cats, trackedBatch)
		if err != nil {
			c.log.Warn("categorize batch failed", "err", err, "size", len(batch))
			for _, p := range batch {
				c.failOrTypeOnly(ctx, p)
			}
			continue
		}
		for _, p := range batch {
			r, ok := results[p.item.ExternalID]
			if !ok {
				c.failOrTypeOnly(ctx, p) // omitted by the model
				continue
			}
			ids := c.resolveIDs(r.Paths, idByPath)
			ftype := r.Type // LLM's type is preferred for the software sub-form
			if ftype == "" {
				ftype = p.ftype
			}
			// Resource-class items may legitimately have no domain; that is a
			// success (marked done with type), not a failure.
			if len(ids.paths) == 0 && !isResourceType(ftype) {
				c.failOrTypeOnly(ctx, p)
				continue
			}
			if err := c.markDone(ctx, p.item.ID, ids.ids, ids.paths, ftype, r.Tags, "llm"); err != nil {
				c.log.Error("categorize update failed", "item", p.item.ExternalID, "err", err)
				continue
			}
			done++
		}
	}
	return done
}

// failOrTypeOnly resolves an item the LLM could not place. Resource-class items
// (awesome/tutorial/interview/skill) are marked done with their known type and
// an empty domain rather than accruing a failure — their type alone is useful.
func (c *Categorizer) failOrTypeOnly(ctx context.Context, p pending) {
	if isResourceType(p.ftype) {
		_ = c.markDone(ctx, p.item.ID, nil, nil, p.ftype, nil, "rule")
		return
	}
	c.markFailed(ctx, p.item)
}

func isResourceType(t string) bool {
	switch t {
	case "awesome", "interview", "tutorial", "skill":
		return true
	}
	return false
}

// loadTaxonomy returns the frozen tree (createdBy=taxonomy only). The path→id
// index contains ONLY leaf categories: parents (e.g. "lang", "ai") are never
// assignment targets, so an LLM that returns a parent path is dropped.
func (c *Categorizer) loadTaxonomy(ctx context.Context) ([]domain.Category, map[string]string, error) {
	cur, err := c.store.Categories().Find(ctx, bson.M{"createdBy": "taxonomy"})
	if err != nil {
		return nil, nil, err
	}
	var cats []domain.Category
	if err := cur.All(ctx, &cats); err != nil {
		return nil, nil, err
	}
	isParent := make(map[string]bool, len(cats))
	for _, cat := range cats {
		if cat.ParentID != nil {
			isParent[cat.ParentID.Hex()] = true
		}
	}
	idByPath := make(map[string]string, len(cats))
	for _, cat := range cats {
		if !isParent[cat.ID.Hex()] { // leaf only
			idByPath[cat.Path] = cat.ID.Hex()
		}
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

// markDone writes the multi-label domain, form type and (optionally) generated
// tags. Tags land under sourceData.generatedTopics, kept apart from the author's
// topicNames so the rule tier never trusts synthetic topics (change 12 guardrail).
func (c *Categorizer) markDone(ctx context.Context, id interface{}, catIDs, paths []string, ftype string, tags []string, by string) error {
	if catIDs == nil {
		catIDs = []string{}
	}
	if paths == nil {
		paths = []string{}
	}
	set := bson.M{
		"categoryId":     catIDs,
		"categoryPath":   paths,
		"type":           ftype,
		"analysisStatus": domain.AnalysisDone,
		"classifiedBy":   by,
	}
	if len(tags) > 0 {
		set["generatedTopics"] = tags // top-level: fetcher's sourceData replace can't wipe it
	}
	_, err := c.store.Items().UpdateByID(ctx, id, bson.M{"$set": set})
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
