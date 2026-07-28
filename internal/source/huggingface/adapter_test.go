package huggingface

import (
	"reflect"
	"testing"

	"github.com/elbaldfun/ghta/internal/domain"
)

func TestMapModel(t *testing.T) {
	m := Model{
		ID:          "deepseek-ai/DeepSeek-R1",
		Likes:       12000,
		Downloads:   4500000,
		PipelineTag: "text-generation",
		LibraryName: "transformers",
		Tags:        []string{"safetensors", "license:mit", "en", "gguf"},
		Gated:       false,
		CreatedAt:   "2025-01-20T09:00:00.000Z",
	}
	it := mapModel(m)

	if it.Source != domain.SourceHuggingFace || it.ExternalID != "deepseek-ai/DeepSeek-R1" {
		t.Fatalf("identity: %+v", it)
	}
	if it.PrimaryMetric != "likes" {
		t.Errorf("primary metric = %q, want likes (velocity axis rides the shared metrics job)", it.PrimaryMetric)
	}
	if it.Metrics["likes"] != 12000 || it.Metrics["downloads30d"] != 4500000 {
		t.Errorf("metrics: %v", it.Metrics)
	}
	if !reflect.DeepEqual([]string(it.CategoryPath), []string{"hf/text-gen"}) {
		t.Errorf("categoryPath = %v", it.CategoryPath)
	}
	if it.SourceData["license"] != "mit" || it.SourceData["author"] != "deepseek-ai" {
		t.Errorf("sourceData: license=%v author=%v", it.SourceData["license"], it.SourceData["author"])
	}
	if !reflect.DeepEqual(it.SourceData["quantFormats"], []string{"gguf", "safetensors"}) {
		t.Errorf("quantFormats = %v", it.SourceData["quantFormats"])
	}

	// gated arrives as false | "auto" | "manual"
	m.Gated = "manual"
	if g := mapModel(m).SourceData["gated"]; g != true {
		t.Errorf("gated 'manual' -> %v, want true", g)
	}
}

func TestTaskGroup(t *testing.T) {
	cases := map[string]string{
		"text-generation":     "text-gen",
		"image-text-to-text":  "multimodal",
		"text-to-image":       "image-gen",
		"sentence-similarity": "embedding",
		"tabular-regression":  "other", // unmapped sinks to other
		"":                    "other",
	}
	for tag, want := range cases {
		if got := TaskGroup(tag); got != want {
			t.Errorf("TaskGroup(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestParseNextLink(t *testing.T) {
	link := `<https://huggingface.co/api/models?cursor=abc>; rel="next"`
	if got := parseNextLink(link); got != "https://huggingface.co/api/models?cursor=abc" {
		t.Errorf("parseNextLink = %q", got)
	}
	if got := parseNextLink(""); got != "" {
		t.Errorf("empty link -> %q", got)
	}
}
