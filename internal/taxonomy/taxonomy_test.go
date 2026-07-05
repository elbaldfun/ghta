package taxonomy

import (
	"path/filepath"
	"runtime"
	"testing"
)

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// The real taxonomy assets must parse, and every topic-map target must exist in
// the tree — this guards human edits to the YAML files in CI.
func TestRealAssetsConsistent(t *testing.T) {
	nodes, err := Load(filepath.Join(repoRoot(), "taxonomy", "taxonomy.yaml"))
	if err != nil {
		t.Fatalf("load taxonomy: %v", err)
	}
	if len(nodes) < 20 {
		t.Fatalf("suspiciously small taxonomy: %d nodes", len(nodes))
	}
	paths := map[string]bool{}
	for _, n := range nodes {
		if n.Path == "" || n.Name == "" {
			t.Errorf("node with empty path/name: %+v", n)
		}
		if paths[n.Path] {
			t.Errorf("duplicate path %q", n.Path)
		}
		paths[n.Path] = true
	}

	rules, err := LoadRules(filepath.Join(repoRoot(), "taxonomy", "topic-map.yaml"))
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	for topic, target := range rules.Topics {
		if !paths[target] {
			t.Errorf("topic %q maps to unknown category %q", topic, target)
		}
	}
	for lang, target := range rules.Languages {
		if !paths[target] {
			t.Errorf("language %q maps to unknown category %q", lang, target)
		}
	}
}

func TestRulesClassify(t *testing.T) {
	r := &Rules{
		Topics:    map[string]string{"llm": "ai/llm", "react": "web/frontend"},
		Languages: map[string]string{"dart": "mobile/cross"},
	}
	// multi-label, case-insensitive, deduped
	got := r.Classify([]string{"LLM", "react", "llm"}, "Go")
	if len(got) != 2 || got[0] != "ai/llm" || got[1] != "web/frontend" {
		t.Errorf("got %v, want [ai/llm web/frontend]", got)
	}
	// language fallback only when no topic hits
	if got := r.Classify([]string{"unknown"}, "Dart"); len(got) != 1 || got[0] != "mobile/cross" {
		t.Errorf("language fallback got %v", got)
	}
	if got := r.Classify(nil, ""); got != nil {
		t.Errorf("empty input got %v", got)
	}
}
