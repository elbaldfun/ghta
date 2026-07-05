package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/provider"
	"github.com/elbaldfun/ghta/internal/repository"
)

const aiSystemPrompt = "You are a technical expert who categorizes items (GitHub repositories, apps, browser extensions) by their content and purpose. Respond with a single JSON object only."

// BatchResult is a resolved categorization for one item.
type BatchResult struct {
	CategoryID string
	Path       string
}

type AIService struct {
	store    *repository.Store
	provider provider.Provider
}

func NewAIService(store *repository.Store, p provider.Provider) *AIService {
	return &AIService{store: store, provider: p}
}

// AnalyzeBatch categorizes many items in one AI call. It returns a result per
// item keyed by externalId (items the model omitted are absent, i.e. failed),
// plus any categories created during the batch so the caller can refresh its
// cached tree. A non-nil error means the whole call/parse failed.
func (s *AIService) AnalyzeBatch(ctx context.Context, cats []domain.Category, items []domain.TrackedItem) (map[string]BatchResult, []domain.Category, error) {
	prompt := buildBatchPrompt(cats, items)
	raw, err := s.provider.AnalyzeJSON(ctx, aiSystemPrompt, prompt)
	if err != nil {
		return nil, nil, err
	}
	elems, err := parseBatchResponse(raw)
	if err != nil {
		return nil, nil, err
	}

	results := make(map[string]BatchResult, len(items))
	var created []domain.Category
	for _, it := range items {
		el, ok := elems[it.ExternalID]
		if !ok {
			continue // omitted by the model -> element-level failure
		}
		if el.IsNewCategory {
			cat, isNew, err := s.ensureCategory(ctx, el.Path)
			if err != nil {
				continue
			}
			if isNew {
				created = append(created, *cat)
			}
			results[it.ExternalID] = BatchResult{CategoryID: cat.ID.Hex(), Path: cat.Path}
			continue
		}
		if el.Path == "" && el.CategoryID == "" {
			continue // unusable element
		}
		results[it.ExternalID] = BatchResult{CategoryID: el.CategoryID, Path: el.Path}
	}
	return results, created, nil
}

type batchElement struct {
	ID            string `json:"id"`
	CategoryID    string `json:"categoryId"`
	Path          string `json:"path"`
	IsNewCategory bool   `json:"isNewCategory"`
	SuggestedName string `json:"suggestedName"`
}

func buildBatchPrompt(cats []domain.Category, items []domain.TrackedItem) string {
	var b strings.Builder
	b.WriteString(`Categorize each item below using the existing category tree.

Rules:
1. Use each item's name, description, language and topics to find the best category.
2. Reuse an existing category whenever possible.
3. Only when nothing fits, propose a new category whose path extends the tree.

Existing categories:
`)
	b.WriteString(renderCategoryTree(cats))
	b.WriteString("\nItems (categorize each, echo its id):\n")
	for _, it := range items {
		fmt.Fprintf(&b, "- id=%q name=%q lang=%q topics=[%s] desc=%q\n",
			it.ExternalID, it.Name, it.Language, topicsOf(it), truncate(it.Description, 240))
	}
	b.WriteString(`
Respond with a single JSON object only:
{"results":[{"id":"<echo item id>","categoryId":"existing-id or empty","path":"existing or new path","isNewCategory":false,"suggestedName":""}]}`)
	return b.String()
}

// renderCategoryTree renders the categories as an indented outline with ids/paths.
func renderCategoryTree(cats []domain.Category) string {
	var b strings.Builder
	var walk func(parentID string, depth int)
	walk = func(parentID string, depth int) {
		for _, c := range cats {
			cp := ""
			if c.ParentID != nil {
				cp = c.ParentID.Hex()
			}
			if cp != parentID {
				continue
			}
			fmt.Fprintf(&b, "%s- %s (ID: %s, Path: %s)\n", strings.Repeat("  ", depth), c.Name, c.ID.Hex(), c.Path)
			walk(c.ID.Hex(), depth+1)
		}
	}
	walk("", 0)
	if b.Len() == 0 {
		return "(none yet)"
	}
	return b.String()
}

// parseBatchResponse extracts the results array from the model's JSON object,
// falling back to the outermost braces. Returns a map keyed by item id.
func parseBatchResponse(raw string) (map[string]batchElement, error) {
	type wrap struct {
		Results []batchElement `json:"results"`
	}
	for _, candidate := range []string{strings.TrimSpace(raw), extractBraces(raw)} {
		if candidate == "" {
			continue
		}
		var w wrap
		if err := json.Unmarshal([]byte(candidate), &w); err == nil && len(w.Results) > 0 {
			out := make(map[string]batchElement, len(w.Results))
			for _, e := range w.Results {
				if e.ID != "" {
					out[e.ID] = e
				}
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	return nil, errors.New("failed to parse AI batch response")
}

func extractBraces(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j <= i {
		return ""
	}
	return s[i : j+1]
}

// topicsOf extracts topic names from sourceData, tolerating both the in-process
// []string and the []interface{} shape produced when decoding from Mongo.
func topicsOf(item domain.TrackedItem) string {
	raw, ok := item.SourceData["topicNames"]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case []string:
		return strings.Join(v, ", ")
	case primitive.A:
		return joinAny(v)
	case []interface{}:
		return joinAny(v)
	}
	return ""
}

func joinAny(items []interface{}) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ensureCategory returns the category for path, creating it (root categories get
// a nil parent) if absent. Existing paths are reused. The bool reports creation.
func (s *AIService) ensureCategory(ctx context.Context, path string) (*domain.Category, bool, error) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return nil, false, errors.New("empty category path")
	}

	var existing domain.Category
	err := s.store.Categories().FindOne(ctx, bson.M{"path": path}).Decode(&existing)
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, err
	}

	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	var parentID *primitive.ObjectID
	if len(parts) > 1 {
		parentPath := strings.Join(parts[:len(parts)-1], "/")
		var parent domain.Category
		if err := s.store.Categories().FindOne(ctx, bson.M{"path": parentPath}).Decode(&parent); err == nil {
			parentID = &parent.ID
		}
	}

	now := time.Now().UTC()
	cat := domain.Category{
		Name:      name,
		ParentID:  parentID,
		Level:     len(parts),
		Path:      path,
		CreatedBy: "ai",
		CreatedAt: now,
		UpdatedAt: now,
	}
	res, err := s.store.Categories().InsertOne(ctx, cat)
	if err != nil {
		return nil, false, err
	}
	cat.ID = res.InsertedID.(primitive.ObjectID)
	return &cat, true, nil
}
