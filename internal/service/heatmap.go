package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/repository"
)

const heatmapTTL = time.Hour

// HeatCell is one domain on the ecosystem map. Scale (repos) and heat
// (intensity) are deliberately separate: the biggest field is often not the one
// moving, and a map that only encoded size would restate the taxonomy instead of
// saying anything about right now.
type HeatCell struct {
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	NameEn    string     `json:"nameEn,omitempty"`
	Repos     int        `json:"repos"`
	Stars     int        `json:"stars"`
	Growth    int        `json:"growth"`    // Σ daily star gain across the subtree
	Intensity float64    `json:"intensity"` // growth per repo — the heat signal
	Children  []HeatCell `json:"children,omitempty"`
}

// HeatmapService computes the domain heat map: every taxonomy node with its
// scale and current velocity.
type HeatmapService struct {
	store *repository.Store
	cats  *CategoryService
	cache *ttlCache[[]HeatCell]
}

func NewHeatmapService(store *repository.Store, cats *CategoryService) *HeatmapService {
	return &HeatmapService{store: store, cats: cats, cache: newTTLCache[[]HeatCell](heatmapTTL)}
}

// Map returns the top-level domains, each with its children, sorted by growth.
// The single cached value recomputes outside the lock (see ttlCache), so a slow
// rebuild doesn't block concurrent callers.
func (s *HeatmapService) Map(ctx context.Context) ([]HeatCell, error) {
	return s.cache.get(ctx, "", func(ctx context.Context) ([]HeatCell, error) {
		return s.compute(ctx)
	})
}

func (s *HeatmapService) compute(ctx context.Context) ([]HeatCell, error) {
	// One pass over the leaf paths; parents are rolled up in Go rather than with
	// a second aggregation.
	cur, err := s.store.Items().Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$unwind", Value: "$categoryPath"}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":    "$categoryPath",
			"repos":  bson.M{"$sum": 1},
			"stars":  bson.M{"$sum": "$metrics.stars"},
			"growth": bson.M{"$sum": bson.M{"$ifNull": bson.A{"$dailyIncrease", 0}}},
		}}},
	})
	if err != nil {
		return nil, fmt.Errorf("heatmap aggregate: %w", err)
	}
	var rows []struct {
		Path   string  `bson:"_id"`
		Repos  int     `bson:"repos"`
		Stars  float64 `bson:"stars"`
		Growth float64 `bson:"growth"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("heatmap decode: %w", err)
	}

	// Names come from the taxonomy so the map speaks the site's language.
	tree, err := s.cats.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	type label struct{ name, nameEn string }
	labels := map[string]label{}
	for _, top := range tree {
		labels[top.Path] = label{top.Name, top.NameEn}
		for _, kid := range top.Children {
			labels[kid.Path] = label{kid.Name, kid.NameEn}
		}
	}

	tops := map[string]*HeatCell{}
	for _, r := range rows {
		if r.Path == "" {
			continue
		}
		topPath := r.Path
		if i := strings.IndexByte(r.Path, '/'); i > 0 {
			topPath = r.Path[:i]
		}
		top, ok := tops[topPath]
		if !ok {
			l := labels[topPath]
			name := l.name
			if name == "" {
				name = topPath
			}
			top = &HeatCell{Path: topPath, Name: name, NameEn: l.nameEn}
			tops[topPath] = top
		}
		top.Repos += r.Repos
		top.Stars += int(r.Stars)
		top.Growth += int(r.Growth)

		if topPath != r.Path { // a leaf under this domain
			l := labels[r.Path]
			name := l.name
			if name == "" {
				name = r.Path
			}
			top.Children = append(top.Children, HeatCell{
				Path: r.Path, Name: name, NameEn: l.nameEn,
				Repos: r.Repos, Stars: int(r.Stars), Growth: int(r.Growth),
				Intensity: perRepo(int(r.Growth), r.Repos),
			})
		}
	}

	out := make([]HeatCell, 0, len(tops))
	for _, t := range tops {
		t.Intensity = perRepo(t.Growth, t.Repos)
		sort.Slice(t.Children, func(i, j int) bool { return t.Children[i].Growth > t.Children[j].Growth })
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Growth > out[j].Growth })
	return out, nil
}

func perRepo(growth, repos int) float64 {
	if repos == 0 {
		return 0
	}
	v := float64(growth) / float64(repos)
	return float64(int(v*100+0.5)) / 100 // 2dp, no fmt round-trip
}
