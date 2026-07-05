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

const aiSystemPrompt = "You are a technical expert who categorizes items (GitHub repositories, apps, browser extensions) by their content and purpose. Respond with JSON only."

// AIResult is the parsed categorization decision.
type AIResult struct {
	CategoryID    string `json:"categoryId"`
	Path          string `json:"path"`
	IsNewCategory bool   `json:"isNewCategory"`
	SuggestedName string `json:"suggestedName"`
}

type AIService struct {
	store    *repository.Store
	provider provider.Provider
}

func NewAIService(store *repository.Store, p provider.Provider) *AIService {
	return &AIService{store: store, provider: p}
}

// AnalyzeItem categorizes one item, creating a new category if the model asks
// for one. It returns the resolved category id and path.
func (s *AIService) AnalyzeItem(ctx context.Context, cats []domain.Category, item domain.TrackedItem) (string, string, error) {
	prompt := buildPrompt(cats, item)
	raw, err := s.provider.Analyze(ctx, aiSystemPrompt, prompt)
	if err != nil {
		return "", "", err
	}
	res, err := parseAIResponse(raw)
	if err != nil {
		return "", "", err
	}
	if res.IsNewCategory {
		cat, err := s.ensureCategory(ctx, res.Path)
		if err != nil {
			return "", "", err
		}
		return cat.ID.Hex(), cat.Path, nil
	}
	return res.CategoryID, res.Path, nil
}

func buildPrompt(cats []domain.Category, item domain.TrackedItem) string {
	var topics string
	if names, ok := item.SourceData["topicNames"].([]string); ok {
		topics = strings.Join(names, ", ")
	}
	return fmt.Sprintf(`Categorize the item using the existing category tree.

Rules:
1. Use the item's name, description, language and topics to find the most related category.
2. Reuse an existing category whenever possible.
3. If nothing fits, propose a new category whose path extends the existing tree.

Item:
- Name: %s
- Description: %s
- Language: %s
- Topics: %s

Existing categories:
%s

Respond with JSON only, no other text:
{"categoryId": "existing-id or empty", "path": "existing or new path", "isNewCategory": false, "suggestedName": ""}`,
		item.Name, item.Description, item.Language, topics, renderCategoryTree(cats))
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

// parseAIResponse extracts the decision JSON: whole body first, then a fenced
// block, then the outermost braces. Anything else is an error.
func parseAIResponse(raw string) (AIResult, error) {
	candidates := []string{
		strings.TrimSpace(raw),
		extractFenced(raw),
		extractBraces(raw),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		var r AIResult
		if err := json.Unmarshal([]byte(c), &r); err == nil && (r.CategoryID != "" || r.Path != "") {
			return r, nil
		}
	}
	return AIResult{}, errors.New("failed to parse AI response")
}

func extractFenced(s string) string {
	i := strings.Index(s, "```")
	if i < 0 {
		return ""
	}
	rest := s[i+3:]
	rest = strings.TrimPrefix(rest, "json")
	rest = strings.TrimPrefix(rest, "\n")
	j := strings.Index(rest, "```")
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}

func extractBraces(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j <= i {
		return ""
	}
	return s[i : j+1]
}

// ensureCategory returns the category for path, creating it (and honoring root
// categories with a nil parent) if absent. Existing paths are reused.
func (s *AIService) ensureCategory(ctx context.Context, path string) (*domain.Category, error) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return nil, errors.New("empty category path")
	}

	// Reuse if the path already exists.
	var existing domain.Category
	err := s.store.Categories().FindOne(ctx, bson.M{"path": path}).Decode(&existing)
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
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
		return nil, err
	}
	cat.ID = res.InsertedID.(primitive.ObjectID)
	return &cat, nil
}
