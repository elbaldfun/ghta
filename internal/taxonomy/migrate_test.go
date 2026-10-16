package taxonomy

import (
	"os"
	"path/filepath"
	"testing"
)

// The shipped tree must be v3-shaped: versioned, apps three deep, nothing
// else deeper than two, and every migration target a real leaf.
func TestRealTreeAndMigrations(t *testing.T) {
	tree, err := LoadTree(filepath.Join(repoRoot(), "taxonomy", "taxonomy.yaml"))
	if err != nil {
		t.Fatalf("load tree: %v", err)
	}
	if tree.Version < 3 {
		t.Fatalf("tree version %d, want >= 3", tree.Version)
	}
	shelves := 0
	for _, l := range tree.Leaves() {
		if IsShelfPath(l.Path) {
			shelves++
		}
	}
	if shelves < 30 {
		t.Errorf("only %d shelves under apps/, want the v3 set", shelves)
	}
	ms, err := LoadMigrations(filepath.Join(repoRoot(), "taxonomy", "migrations.yaml"), tree)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var step *Migration
	for i := range ms {
		if ms[i].To == tree.Version {
			step = &ms[i]
		}
	}
	if step == nil {
		t.Fatalf("no migration step lands on v%d", tree.Version)
	}
}

func TestLoadTreeRejectsBadShapes(t *testing.T) {
	cases := map[string]string{
		"third level outside apps": "version: 3\nnodes:\n- path: web\n  name: W\n  children:\n  - path: web/a\n    name: a\n    children:\n    - path: web/a/b\n      name: b\n",
		"depth/path mismatch":      "version: 3\nnodes:\n- path: web/a\n  name: W\n",
	}
	for name, body := range cases {
		p := filepath.Join(t.TempDir(), "t.yaml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadTree(p); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// Legacy bare-list shape still loads as v2.
	p := filepath.Join(t.TempDir(), "v2.yaml")
	if err := os.WriteFile(p, []byte("- path: web\n  name: W\n  children:\n  - { path: web/a, name: a }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTree(p)
	if err != nil || tr.Version != 2 || len(tr.Nodes) != 2 {
		t.Errorf("legacy load: %v %+v", err, tr)
	}
}

func TestShelfHelpers(t *testing.T) {
	if !IsShelfPath("apps/productivity/notes") || IsShelfPath("apps/productivity") || IsShelfPath("devtools/terminal") {
		t.Error("IsShelfPath")
	}
	if ShelfSlug("apps/productivity/notes") != "productivity/notes" || ShelfPath("productivity/notes") != "apps/productivity/notes" || ShelfPath("apps/x/y") != "apps/x/y" {
		t.Error("ShelfSlug/ShelfPath")
	}
}

// ---- mongo-backed: the migration ops end to end ----

var v3Nodes = Tree{Version: 3, Nodes: []Node{
	{Path: "web", Name: "web"}, {Path: "web/ui-components", Name: "ui"}, {Path: "web/frontend", Name: "fe"}, {Path: "web/data-viz", Name: "viz"},
	{Path: "infra", Name: "infra"}, {Path: "infra/automation", Name: "auto"},
	{Path: "apps", Name: "apps"}, {Path: "apps/productivity", Name: "prod"}, {Path: "apps/productivity/notes", Name: "notes"},
	{Path: "devtools", Name: "dt"}, {Path: "devtools/editors", Name: "ed"},
}}
