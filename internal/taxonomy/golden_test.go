package taxonomy

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every golden domain path must be a leaf of the shipped tree — the golden set
// is only comparable against the tree it was labelled on, so a tree change
// that forgets to migrate golden.yaml fails here.
func TestGoldenPathsAreLeaves(t *testing.T) {
	tree, err := LoadTree(filepath.Join(repoRoot(), "taxonomy", "taxonomy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	leaves := map[string]bool{}
	for _, l := range tree.Leaves() {
		leaves[l.Path] = true
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot(), "taxonomy", "golden.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Items []struct {
			ID     string   `yaml:"id"`
			Domain []string `yaml:"domain"`
			Type   string   `yaml:"type"`
		} `yaml:"items"`
	}
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	if len(g.Items) < 200 {
		t.Fatalf("golden has %d items, expected the full set", len(g.Items))
	}
	for _, it := range g.Items {
		for _, p := range it.Domain {
			if !leaves[p] {
				t.Errorf("golden %s: %q is not a leaf of the v%d tree", it.ID, p, tree.Version)
			}
		}
	}
}
