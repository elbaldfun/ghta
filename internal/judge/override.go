package judge

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// Override is a person's answer for one item (taxonomy/overrides.yaml),
// given where the model's was wrong. It has the shape of a verdict, and it
// replaces the verdict's domain, form and shelf.
type Override struct {
	ID    string   `yaml:"id"`
	Type  string   `yaml:"type"`
	Paths []string `yaml:"paths"`
	Shelf string   `yaml:"shelf"`
	Why   string   `yaml:"why"`
}

// Domain is what the item is filed under, the shelf's leaf first, as for a
// verdict.
func (o Override) Domain() []string {
	return Verdict{Paths: o.Paths, Shelf: o.Shelf}.Domain()
}

// LoadOverrides reads and checks the override table against a tree. Anything
// a verdict could not hold is an error, so a bad table stops the worker at
// start rather than writing labels the site cannot file.
func LoadOverrides(path string, tree taxonomy.Tree) ([]Override, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read overrides: %w", err)
	}
	var f struct {
		Items []Override `yaml:"items"`
	}
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse overrides: %w", err)
	}
	j := New(nil, tree)
	seen := map[string]bool{}
	for _, o := range f.Items {
		if err := j.checkOverride(o); err != nil {
			return nil, fmt.Errorf("override %s: %w", o.ID, err)
		}
		if seen[o.ID] {
			return nil, fmt.Errorf("override %s: listed twice", o.ID)
		}
		seen[o.ID] = true
	}
	return f.Items, nil
}

func (j *Judge) checkOverride(o Override) error {
	if owner, name, ok := strings.Cut(o.ID, "/"); !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("id must be owner/name")
	}
	if !slices.Contains(Types, o.Type) {
		return fmt.Errorf("type %q is not a form", o.Type)
	}
	if o.Shelf == "" || !j.isShelf[o.Shelf] {
		return fmt.Errorf("shelf %q is not a shelf of the tree or %q", o.Shelf, item.ShelfExcluded)
	}
	if o.Shelf != item.ShelfExcluded && o.Type != "app" && o.Type != "cli" {
		return fmt.Errorf("only an app or a cli goes on a shelf, not a %s", o.Type)
	}
	if slices.Contains(resourceTypes, o.Type) && o.Shelf != item.ShelfExcluded {
		return fmt.Errorf("a %s is not shelved", o.Type)
	}
	for i, p := range o.Paths {
		// A second shelf's leaf is allowed beside the shelf (golden files a
		// few apps under two); it does not put the item on that shelf.
		app := taxonomy.IsShelfPath(p) && j.isShelf[taxonomy.ShelfSlug(p)]
		if !j.isLeaf[p] && !app {
			return fmt.Errorf("path %q is not a leaf of the tree", p)
		}
		if OnConsumerShelf(o.Shelf) && strings.HasPrefix(p, ClientBranch) {
			return fmt.Errorf("path %q: an app on a consumer shelf is not filed under client development", p)
		}
		if app && o.Shelf == item.ShelfExcluded {
			return fmt.Errorf("path %q is an apps leaf, but the item is on no shelf", p)
		}
		if slices.Contains(o.Paths[:i], p) {
			return fmt.Errorf("path %q listed twice", p)
		}
		if o.Shelf != item.ShelfExcluded && ShelfLeaf(o.Shelf) == p {
			return fmt.Errorf("path %q is the shelf's leaf, which is added already", p)
		}
	}
	leaves := len(o.Paths)
	if o.Shelf != item.ShelfExcluded {
		leaves++
	}
	if leaves > maxDomain {
		return fmt.Errorf("at most %d domain leaves, the shelf's included", maxDomain)
	}
	if strings.TrimSpace(o.Why) == "" {
		return fmt.Errorf("why is required")
	}
	return nil
}
