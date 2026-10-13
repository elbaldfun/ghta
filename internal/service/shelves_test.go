package service

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// AppShelves must be exactly the apps branch of taxonomy.yaml (minus the
// prefix), and every DevShelves entry must be a non-apps leaf — the store
// enum and the tree are one thing in v3.
func TestShelvesMatchTree(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	tree, err := taxonomy.LoadTree(filepath.Join(filepath.Dir(file), "..", "..", "taxonomy", "taxonomy.yaml"))
	if err != nil {
		t.Fatalf("load tree: %v", err)
	}
	leaves := map[string]bool{}
	fromTree := map[string]bool{}
	for _, l := range tree.Leaves() {
		leaves[l.Path] = true
		if taxonomy.IsShelfPath(l.Path) {
			fromTree[taxonomy.ShelfSlug(l.Path)] = true
		}
	}
	for _, s := range AppShelves {
		if !fromTree[s] {
			t.Errorf("AppShelves has %q, not an apps/ leaf in the tree", s)
		}
		delete(fromTree, s)
	}
	for s := range fromTree {
		t.Errorf("tree shelf %q missing from AppShelves", s)
	}
	for _, d := range DevShelves {
		if !leaves[d] || taxonomy.IsShelfPath(d) {
			t.Errorf("DevShelves %q must be a non-apps leaf", d)
		}
		if ShelfDomainPath(d) != d {
			t.Errorf("ShelfDomainPath(%q) = %q", d, ShelfDomainPath(d))
		}
	}
	if ShelfDomainPath("productivity/notes") != "apps/productivity/notes" {
		t.Error("consumer shelf path")
	}
	if !validShelfFilter(DevShelfGroup) || !validShelfFilter("system") || validShelfFilter("devtools") || validShelfFilter("excluded") {
		t.Error("validShelfFilter groups")
	}
}
