// Package taxonomy loads the git-controlled category tree (taxonomy.yaml) and
// topic mapping rules (topic-map.yaml), and syncs the tree into MongoDB.
// The YAML files are the single source of truth: the AI never creates
// categories, it only files suggestions.
package taxonomy

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v3"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// Node is one category in taxonomy.yaml.
type Node struct {
	Path     string `yaml:"path"`
	Name     string `yaml:"name"`
	NameEn   string `yaml:"nameEn"`
	Desc     string `yaml:"desc"`
	Children []Node `yaml:"children"`
}

// Load parses taxonomy.yaml and returns the flattened node list (parents first).
func Load(path string) ([]Node, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read taxonomy: %w", err)
	}
	var roots []Node
	if err := yaml.Unmarshal(raw, &roots); err != nil {
		return nil, fmt.Errorf("parse taxonomy: %w", err)
	}
	var flat []Node
	var walk func(nodes []Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			children := n.Children
			n.Children = nil
			flat = append(flat, n)
			walk(children)
		}
	}
	walk(roots)
	return flat, nil
}

// Sync upserts the taxonomy into the categories collection (matched by path,
// createdBy=taxonomy). It is idempotent and safe to run at every startup.
func Sync(ctx context.Context, store *repository.Store, nodes []Node) error {
	now := time.Now().UTC()
	idByPath := map[string]interface{}{}

	for _, n := range nodes { // parents come first, so parent ids resolve
		level := len(strings.Split(n.Path, "/"))
		var parentID interface{}
		if i := strings.LastIndex(n.Path, "/"); i > 0 {
			parentID = idByPath[n.Path[:i]]
		}
		res := store.Categories().FindOneAndUpdate(ctx,
			bson.M{"path": n.Path},
			bson.M{
				"$set": bson.M{
					"name": n.Name, "nameEn": n.NameEn, "description": n.Desc, "level": level,
					"parentId": parentID, "createdBy": "taxonomy", "updatedAt": now,
				},
				"$setOnInsert": bson.M{"createdAt": now},
			},
			options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
		)
		var cat domain.Category
		if err := res.Decode(&cat); err != nil {
			return fmt.Errorf("sync category %s: %w", n.Path, err)
		}
		idByPath[n.Path] = cat.ID
	}

	// Prune taxonomy categories no longer in the tree (e.g. change 12 removed
	// learning/*, lang/stdlib-utils, *-framework). Without this they linger in
	// the categories collection and still render in GET /category. Only prunes
	// createdBy=taxonomy — human/legacy-ai categories are never touched.
	current := make([]string, 0, len(nodes))
	for _, n := range nodes {
		current = append(current, n.Path)
	}
	if _, err := store.Categories().DeleteMany(ctx, bson.M{
		"createdBy": "taxonomy",
		"path":      bson.M{"$nin": current},
	}); err != nil {
		return fmt.Errorf("prune stale categories: %w", err)
	}
	return nil
}

// Rules is the topic/language → category-path mapping from topic-map.yaml.
type Rules struct {
	Topics    map[string]string `yaml:"topics"`
	Languages map[string]string `yaml:"languages"`
}

func LoadRules(path string) (*Rules, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read topic map: %w", err)
	}
	var r Rules
	if err := yaml.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parse topic map: %w", err)
	}
	return &r, nil
}

// Classify returns the category paths an item maps to by rule: every distinct
// topic hit (multi-label); if no topic hits, the primary-language fallback.
func (r *Rules) Classify(topics []string, language string) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, t := range topics {
		if p, ok := r.Topics[strings.ToLower(strings.TrimSpace(t))]; ok {
			if _, dup := seen[p]; !dup {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}
	if len(paths) == 0 && language != "" {
		if p, ok := r.Languages[strings.ToLower(language)]; ok {
			paths = append(paths, p)
		}
	}
	return paths
}
