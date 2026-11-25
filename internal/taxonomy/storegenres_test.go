package taxonomy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func realStoreGenres(t *testing.T) *StoreGenres {
	t.Helper()
	tree, err := LoadTree(filepath.Join(repoRoot(), "taxonomy", "taxonomy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sg, err := LoadStoreGenres(filepath.Join(repoRoot(), "taxonomy", "store-genres.yaml"), tree)
	if err != nil {
		t.Fatal(err)
	}
	return sg
}

// Every listing in store-golden.yaml (real App Store apps, labelled by hand)
// must land on its shelf: a rule edit that moves one fails here.
func TestStoreGolden(t *testing.T) {
	sg := realStoreGenres(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot(), "taxonomy", "store-golden.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Apps []struct {
			ID          string `yaml:"id"`
			Genre       string `yaml:"genre"`
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
			Shelf       string `yaml:"shelf"`
		} `yaml:"apps"`
	}
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse store golden: %v", err)
	}
	if len(g.Apps) < 50 {
		t.Fatalf("store golden has %d apps, want at least 50", len(g.Apps))
	}
	seen := map[string]bool{}
	for _, a := range g.Apps {
		if seen[a.ID] {
			t.Errorf("%s listed twice", a.ID)
		}
		seen[a.ID] = true
		if !sg.Has(a.Genre) {
			t.Errorf("%s (%s): genre %s not in store-genres.yaml", a.ID, a.Name, a.Genre)
			continue
		}
		if got := sg.Shelf(a.Genre, a.Name, a.Description); got != a.Shelf {
			t.Errorf("%s %q (genre %s) → %q, want %q", a.ID, a.Name, a.Genre, got, a.Shelf)
		}
	}
}

func TestStoreGenresRules(t *testing.T) {
	tree := Tree{Version: 4, Nodes: []Node{
		{Path: "apps"}, {Path: "apps/finance"}, {Path: "apps/finance/budget"}, {Path: "apps/finance/investing"},
		{Path: "apps/ai"}, {Path: "apps/ai/assistant"},
	}}
	load := func(body string) (*StoreGenres, error) {
		p := filepath.Join(t.TempDir(), "sg.yaml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return LoadStoreGenres(p, tree)
	}
	sg, err := load(`genres:
  - id: "1"
    shelf: finance/budget
    rules:
      - match: '\bstocks?\b'
        shelf: finance/investing
        description: true
      - match: '\bai\b'
        shelf: ai/assistant
  - id: "2"
    shelf: ""
    why: not ours
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ genre, name, desc, want string }{
		{"1", "Stock Tracker", "", "finance/investing"},
		{"1", "Penny", "Watch your stocks", "finance/investing"},               // description, when the name says nothing
		{"1", "Penny AI", "Watch your stocks", "ai/assistant"},                 // the name wins over the description
		{"1", "Penny", "An AI budget planner", "finance/budget"},               // a rule not marked description ignores it
		{"1", "Penny", strings.Repeat("x", 300) + " stocks", "finance/budget"}, // past the description head
		{"1", "Restocking", "", "finance/budget"},                              // word boundary
		{"2", "Anything", "", ""},
		{"3", "Unlisted genre", "", ""},
	} {
		if got := sg.Shelf(c.genre, c.name, c.desc); got != c.want {
			t.Errorf("Shelf(%s, %q, %q) = %q, want %q", c.genre, c.name, c.desc, got, c.want)
		}
	}

	for name, body := range map[string]string{
		"unknown shelf":   "genres:\n  - id: \"1\"\n    shelf: finance/nope\n",
		"branch not leaf": "genres:\n  - id: \"1\"\n    shelf: finance\n",
		"no why":          "genres:\n  - id: \"1\"\n    shelf: \"\"\n",
		"duplicate":       "genres:\n  - id: \"1\"\n    shelf: finance/budget\n  - id: \"1\"\n    shelf: finance/budget\n",
		"bad regexp":      "genres:\n  - id: \"1\"\n    shelf: finance/budget\n    rules:\n      - match: '('\n        shelf: finance/budget\n",
		"rule no shelf":   "genres:\n  - id: \"1\"\n    shelf: finance/budget\n    rules:\n      - match: x\n",
	} {
		if _, err := load(body); err == nil {
			t.Errorf("%s: loaded without error", name)
		}
	}
}
