package huggingface

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/importer"
	"github.com/elbaldfun/ghta/internal/item"
)

// The typed output must be what the importer produces from the old output
// after a round trip through the old store.
func TestMapModelItemMatchesImportedShape(t *testing.T) {
	full := Model{
		ID: "deepseek-ai/DeepSeek-R1", Likes: 12000, Downloads: 4500000, DownloadsAll: 9000000,
		PipelineTag: "text-generation", LibraryName: "transformers", TrendingScore: 41.5,
		Tags: []string{"safetensors", "license:mit", "en", "gguf", "base_model:deepseek-ai/base",
			"dataset:wiki", "arxiv:2501.12948"},
		Gated: "manual", CreatedAt: "2025-01-20T09:00:00.000Z", LastModified: "2026-10-01T10:00:00.000Z",
		Safetensors: &struct {
			Total int64 `json:"total"`
		}{Total: 685000000000},
	}
	bare := Model{ID: "nameless-model", Likes: 3}

	for name, m := range map[string]Model{"full": full, "bare": bare} {
		t.Run(name, func(t *testing.T) {
			raw, err := bson.Marshal(mapModel(m))
			if err != nil {
				t.Fatal(err)
			}
			var stored domain.TrackedItem
			if err := bson.Unmarshal(raw, &stored); err != nil {
				t.Fatal(err)
			}
			want, blob, notes := importer.Convert(stored, importer.Rules{
				ValidPath: func(string) bool { return true }, ShelfPath: func(s string) string { return s },
			})
			if len(notes.UnknownKeys) > 0 || blob != nil {
				t.Fatalf("unknown keys %v, blob %v", notes.UnknownKeys, blob)
			}
			got := mapModelItem(m)
			if got.Blob != nil {
				t.Fatal("a model has no blob")
			}
			owned := func(it item.Item) item.Item {
				return item.Item{
					Source: it.Source, ExternalID: it.ExternalID, Name: it.Name, Description: it.Description,
					Language: it.Language, Homepage: it.Homepage, License: it.License, Topics: it.Topics,
					PrimaryMetric: it.PrimaryMetric, MetricDirection: it.MetricDirection, Metrics: it.Metrics,
					Labels: item.Labels{Domain: it.Labels.Domain, By: it.Labels.By}, Hf: it.Hf,
				}
			}
			if w, g := owned(want), owned(got.Item); !reflect.DeepEqual(w, g) {
				t.Errorf("item differs\n imported: %+v hf=%+v\n fetched:  %+v hf=%+v", w, w.Hf, g, g.Hf)
			}
		})
	}
	if hf := mapModelItem(full).Item.Hf; hf.Params == 0 || !hf.Gated || len(hf.BaseModels) == 0 || hf.CreatedAt.IsZero() {
		t.Errorf("the sample does not exercise the hf block: %+v", hf)
	}
}
