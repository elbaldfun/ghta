package api

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func hfModel(id string, likes float64, bases ...string) item.Item {
	author, name := id, id
	for i := range id {
		if id[i] == '/' {
			author, name = id[:i], id[i+1:]
			break
		}
	}
	return item.Item{
		Source: item.SourceHuggingFace, ExternalID: id, Name: name, Topics: []string{},
		PrimaryMetric: "likes", Metrics: map[string]float64{"likes": likes, "downloads30d": likes * 10},
		History: []item.Point{}, Labels: item.Labels{Domain: []string{"hf/text-gen"}, By: item.BySource},
		Hf: &item.HuggingFace{Author: author, Task: "text-gen", BaseModels: bases},
	}
}

func TestModelDetail(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	base := hfModel("q/base", 900)
	base.Hf.Datasets, base.Hf.Languages = []string{"wiki"}, []string{"en", "zh"}
	quant := hfModel("q/base-gguf", 40, "q/base")
	quant.Hf.BaseRelation = "quantized"
	merge := hfModel("x/merge", 3, "q/base", "o/other")
	merge.Hf.BaseRelation = "merge"
	dead := hfModel("x/dead", 1, "q/base")
	dead.Stale = true
	// A GitHub repository of the same name is no model.
	seed(t, st, base, quant, merge, dead, ghRepo("q/repo", "Go", 1, 0))
	r := router(st)

	type detail struct {
		Data ModelDetail `json:"data"`
	}
	var d detail
	if w := get(t, r, "/v1/models/q/base", nil, &d); w.Code != http.StatusOK {
		t.Fatalf("base = %d %s", w.Code, w.Body)
	}
	if d.Data.Derivatives == nil || *d.Data.Derivatives != 2 || d.Data.BaseModel != nil ||
		!reflect.DeepEqual(d.Data.Languages, []string{"en", "zh"}) || d.Data.Likes != 900 ||
		d.Data.URL != "https://huggingface.co/q/base" {
		t.Fatalf("base = %+v", d.Data)
	}

	d = detail{}
	get(t, r, "/v1/models/x/merge", nil, &d)
	if !reflect.DeepEqual(d.Data.BaseModel, []string{"q/base", "o/other"}) || d.Data.BaseRelation != "merge" ||
		d.Data.Derivatives == nil || *d.Data.Derivatives != 0 || d.Data.Arxiv == nil {
		t.Fatalf("merge = %+v", d.Data)
	}

	for _, url := range []string{"/v1/models/x/dead", "/v1/models/q/repo", "/v1/models/no/such"} {
		if w := get(t, r, url, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s = %d", url, w.Code)
		}
	}

	// Without the index the count is left out rather than scanning.
	if _, err := st.Items().Indexes().DropOne(ctx, store.DerivativesIndex); err != nil {
		t.Fatal(err)
	}
	d = detail{}
	if w := get(t, r, "/v1/models/q/base", nil, &d); w.Code != http.StatusOK || d.Data.Derivatives != nil {
		t.Fatalf("no index = %d %+v", w.Code, d.Data.Derivatives)
	}
}
