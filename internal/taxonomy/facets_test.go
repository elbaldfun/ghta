package taxonomy

import (
	"path/filepath"
	"runtime"
	"testing"
)

func facetsPath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// internal/taxonomy -> repo root -> taxonomy/facets.yaml
	return filepath.Join(filepath.Dir(file), "..", "..", "taxonomy", "facets.yaml")
}

func TestClassifyType(t *testing.T) {
	f, err := LoadFacets(facetsPath(t))
	if err != nil {
		t.Fatalf("LoadFacets: %v", err)
	}

	cases := []struct {
		name     string
		id       string
		topics   []string
		wantType string
	}{
		// Naming rules (no matching topic) — the hard cases eval flagged.
		{"awesome by name", "vinta/awesome-python", nil, "awesome"},
		{"free-programming by name", "EbookFoundation/free-programming-books", nil, "awesome"},
		{"interview by name", "jwasham/coding-interview-university", nil, "interview"},
		{"tutorial by name", "jackfrued/Python-100-Days", nil, "tutorial"},
		{"the-hard-way by name", "kelseyhightower/kubernetes-the-hard-way", nil, "tutorial"},
		{"skill by name", "anthropics/skills", nil, "skill"},
		{"cli by name", "MoonshotAI/kimi-cli", nil, "cli"},

		// Topic table, priority order (Q1: awesome > interview > tutorial).
		{"awesome beats interview", "x/y", []string{"interview", "awesome-list"}, "awesome"},
		{"cli topic", "x/y", []string{"cli", "terminal"}, "cli"},
		{"app topic", "x/y", []string{"electron", "desktop-app"}, "app"},

		// Fallback: real software with no form signal (transformers has none).
		{"software fallback", "huggingface/transformers", []string{"nlp", "pytorch"}, "software"},
		{"empty falls back", "torvalds/linux", nil, "software"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := f.ClassifyType(c.id, c.topics); got != c.wantType {
				t.Errorf("ClassifyType(%q, %v) = %q, want %q", c.id, c.topics, got, c.wantType)
			}
		})
	}
}
