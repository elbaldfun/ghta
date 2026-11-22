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

func TestParseTagRefs(t *testing.T) {
	refs := ParseTagRefs([]string{
		"transformers", "tf", "en", "zh", "gguf",
		"base_model:meta-llama/Llama-3.1-8B",
		"base_model:finetune:Qwen/Qwen3-32B",
		"dataset:ms_marco", "arxiv:2402.12345",
	})
	if !reflect.DeepEqual(refs.BaseModels, []string{"meta-llama/Llama-3.1-8B", "Qwen/Qwen3-32B"}) {
		t.Errorf("baseModels = %v", refs.BaseModels)
	}
	if !reflect.DeepEqual(refs.Languages, []string{"en", "zh"}) { // "tf" must not read as a language
		t.Errorf("languages = %v", refs.Languages)
	}
	if !reflect.DeepEqual(refs.Datasets, []string{"ms_marco"}) || !reflect.DeepEqual(refs.Arxiv, []string{"2402.12345"}) {
		t.Errorf("datasets=%v arxiv=%v", refs.Datasets, refs.Arxiv)
	}
	if refs.BaseRelation != "finetune" {
		t.Errorf("relation = %q", refs.BaseRelation)
	}
}

// The Hub tags a base twice, bare and qualified; one base results, with the
// qualifier as the relation. An unknown qualifier is not a relation.
func TestParseTagRefsRelation(t *testing.T) {
	for _, c := range []struct {
		tags     []string
		bases    []string
		relation string
	}{
		{[]string{"base_model:Qwen/Qwen2.5-7B", "base_model:quantized:Qwen/Qwen2.5-7B"}, []string{"Qwen/Qwen2.5-7B"}, "quantized"},
		{[]string{"base_model:a/x", "base_model:merge:a/x", "base_model:merge:b/y"}, []string{"a/x", "b/y"}, "merge"},
		{[]string{"base_model:adapter:a/x"}, []string{"a/x"}, "adapter"},
		{[]string{"base_model:a/x"}, []string{"a/x"}, ""},
		{[]string{"base_model:weird:a/x"}, []string{"a/x"}, ""},
		{[]string{"gguf"}, nil, ""},
	} {
		refs := ParseTagRefs(c.tags)
		if !reflect.DeepEqual(refs.BaseModels, c.bases) || refs.BaseRelation != c.relation {
			t.Errorf("%v: bases %v relation %q", c.tags, refs.BaseModels, refs.BaseRelation)
		}
	}
}
