package service

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/provider"
)

// EmbedClassifier assigns items to the closest category by cosine similarity
// between item text and category description text. Cheap, deterministic for a
// fixed model, and reproducible — the pipeline's middle layer.
type EmbedClassifier struct {
	embedder  provider.Embedder
	threshold float64
}

func NewEmbedClassifier(e provider.Embedder, threshold float64) *EmbedClassifier {
	return &EmbedClassifier{embedder: e, threshold: threshold}
}

// Enabled reports whether an embedding backend is configured.
func (c *EmbedClassifier) Enabled() bool { return c != nil && c.embedder != nil }

// categoryVectors embeds every leaf category's "path: name — desc" text.
// Called once per run; ~100 categories fit one request.
func (c *EmbedClassifier) categoryVectors(ctx context.Context, cats []domain.Category) ([]domain.Category, [][]float32, error) {
	leaves := leavesOf(cats)
	if len(leaves) == 0 {
		return nil, nil, fmt.Errorf("no leaf categories to embed")
	}
	texts := make([]string, len(leaves))
	for i, cat := range leaves {
		texts[i] = fmt.Sprintf("%s: %s — %s", cat.Path, cat.Name, cat.Description)
	}
	vecs, err := c.embedder.Embed(ctx, texts)
	if err != nil {
		return nil, nil, err
	}
	return leaves, vecs, nil
}

// Classify returns, per item externalId, the best-matching category when its
// similarity clears the threshold. Items below threshold are simply absent.
func (c *EmbedClassifier) Classify(ctx context.Context, cats []domain.Category, items []domain.TrackedItem) (map[string]BatchResult, error) {
	leaves, catVecs, err := c.categoryVectors(ctx, cats)
	if err != nil {
		return nil, err
	}

	texts := make([]string, len(items))
	for i, it := range items {
		texts[i] = itemText(it)
	}
	itemVecs, err := c.embedder.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}

	out := make(map[string]BatchResult)
	for i, it := range items {
		bestIdx, bestSim := -1, c.threshold
		for j, cv := range catVecs {
			if sim := cosine(itemVecs[i], cv); sim >= bestSim {
				bestIdx, bestSim = j, sim
			}
		}
		if bestIdx >= 0 {
			out[it.ExternalID] = BatchResult{
				CategoryID: leaves[bestIdx].ID.Hex(),
				Path:       leaves[bestIdx].Path,
			}
		}
	}
	return out, nil
}

// leavesOf filters categories that have no children (assignment targets).
func leavesOf(cats []domain.Category) []domain.Category {
	hasChild := map[string]bool{}
	for _, c := range cats {
		if c.ParentID != nil {
			hasChild[c.ParentID.Hex()] = true
		}
	}
	var leaves []domain.Category
	for _, c := range cats {
		if !hasChild[c.ID.Hex()] {
			leaves = append(leaves, c)
		}
	}
	return leaves
}

func itemText(it domain.TrackedItem) string {
	parts := []string{it.Name}
	if it.Description != "" {
		parts = append(parts, it.Description)
	}
	if it.Language != "" {
		parts = append(parts, it.Language)
	}
	if topics := topicsOf(it); topics != "" {
		parts = append(parts, topics)
	}
	return truncate(strings.Join(parts, ". "), 800)
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return -1
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return -1
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
