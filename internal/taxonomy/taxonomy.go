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

// Node is one category in taxonomy.yaml. Est/From are human-facing notes
// (estimated size, v2 origin leaves) that the loader carries but never uses.
type Node struct {
	Path     string   `yaml:"path"`
	Name     string   `yaml:"name"`
	NameEn   string   `yaml:"nameEn"`
	Desc     string   `yaml:"desc"`
	Est      int      `yaml:"est,omitempty"`
	From     []string `yaml:"from,omitempty"`
	Children []Node   `yaml:"children"`
}

// ShelfPrefix is the apps branch: the only branch with a third level. Its
// leaves are the store shelves; the shelf slug is the path minus this prefix
// (apps/productivity/notes -> productivity/notes), which keeps store.category
// values and /apps?shelf= URLs short and stable across tree versions.
const ShelfPrefix = "apps/"

// IsShelfPath reports whether p is a leaf of the apps branch (three segments).
func IsShelfPath(p string) bool {
	return strings.HasPrefix(p, ShelfPrefix) && strings.Count(p, "/") == 2
}

// ShelfSlug strips the apps/ prefix; ShelfPath adds it back.
func ShelfSlug(p string) string { return strings.TrimPrefix(p, ShelfPrefix) }
func ShelfPath(slug string) string {
	if strings.HasPrefix(slug, ShelfPrefix) {
		return slug
	}
	return ShelfPrefix + slug
}

// Tree is the parsed taxonomy.yaml: a version number plus the flattened node
// list (parents first). Version is what items record as taxonomyVersion; an
// item behind it is due for re-classification (see migrations.yaml).
type Tree struct {
	Version int
	Nodes   []Node
}

// Leaves returns the assignment targets: nodes that have no children.
func (t Tree) Leaves() []Node {
	isParent := map[string]bool{}
	for _, n := range t.Nodes {
		if i := strings.LastIndex(n.Path, "/"); i > 0 {
			isParent[n.Path[:i]] = true
		}
	}
	var out []Node
	for _, n := range t.Nodes {
		if !isParent[n.Path] {
			out = append(out, n)
		}
	}
	return out
}

// PathSet returns every node path for membership checks.
func (t Tree) PathSet() map[string]bool {
	m := make(map[string]bool, len(t.Nodes))
	for _, n := range t.Nodes {
		m[n.Path] = true
	}
	return m
}

// treeFile is the on-disk shape since v3: a mapping with version + nodes. The
// v2 file was a bare node list; LoadTree still accepts that and reports it as
// version 2 so an un-migrated checkout keeps booting.
type treeFile struct {
	Version int    `yaml:"version"`
	Nodes   []Node `yaml:"nodes"`
}

// LoadTree parses taxonomy.yaml (either shape) into a flattened Tree.
func LoadTree(path string) (Tree, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Tree{}, fmt.Errorf("read taxonomy: %w", err)
	}
	var roots []Node
	version := 2
	var tf treeFile
	if err := yaml.Unmarshal(raw, &tf); err == nil && len(tf.Nodes) > 0 {
		roots, version = tf.Nodes, tf.Version
	} else if err := yaml.Unmarshal(raw, &roots); err != nil {
		return Tree{}, fmt.Errorf("parse taxonomy: %w", err)
	}
	if version < 1 {
		return Tree{}, fmt.Errorf("parse taxonomy: version must be >= 1")
	}
	var flat []Node
	var walk func(nodes []Node, depth int) error
	walk = func(nodes []Node, depth int) error {
		for _, n := range nodes {
			children := n.Children
			n.Children = nil
			segs := strings.Count(n.Path, "/") + 1
			if segs != depth {
				return fmt.Errorf("node %q nested at depth %d but its path has %d segments", n.Path, depth, segs)
			}
			// Only the apps branch may go three deep; everything else is big-group/leaf.
			if segs > 2 && !strings.HasPrefix(n.Path, ShelfPrefix) {
				return fmt.Errorf("node %q: only the apps branch may have a third level", n.Path)
			}
			if segs > 3 {
				return fmt.Errorf("node %q: tree is at most three levels deep", n.Path)
			}
			flat = append(flat, n)
			if err := walk(children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(roots, 1); err != nil {
		return Tree{}, fmt.Errorf("parse taxonomy: %w", err)
	}
	return Tree{Version: version, Nodes: flat}, nil
}

// Load is LoadTree without the version — kept for callers that only need nodes.
func Load(path string) ([]Node, error) {
	t, err := LoadTree(path)
	if err != nil {
		return nil, err
	}
	return t.Nodes, nil
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

	// Prune taxonomy categories no longer in the tree — but only once nothing
	// references them. During a tree migration the old leaves keep serving
	// (items carry the old path until re-classified, GET /category still lists
	// it); deleting them at the first restart would leave dangling categoryIds
	// and empty navigation. A stale node with zero live items is gone for good.
	// Only prunes createdBy=taxonomy — human/legacy-ai categories are never touched.
	current := make([]string, 0, len(nodes))
	for _, n := range nodes {
		current = append(current, n.Path)
	}
	cur, err := store.Categories().Find(ctx, bson.M{
		"createdBy": "taxonomy",
		"path":      bson.M{"$nin": current},
	})
	if err != nil {
		return fmt.Errorf("list stale categories: %w", err)
	}
	var stale []domain.Category
	if err := cur.All(ctx, &stale); err != nil {
		return fmt.Errorf("decode stale categories: %w", err)
	}
	for _, c := range stale {
		// Answered from the categoryPath index; no document scan.
		n, err := store.Items().CountDocuments(ctx, bson.M{"categoryPath": c.Path}, options.Count().SetLimit(1))
		if err != nil {
			return fmt.Errorf("count refs of %s: %w", c.Path, err)
		}
		if n > 0 {
			continue // still referenced: keep until the migration drains it
		}
		if _, err := store.Categories().DeleteOne(ctx, bson.M{"_id": c.ID}); err != nil {
			return fmt.Errorf("prune %s: %w", c.Path, err)
		}
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
