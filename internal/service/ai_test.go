package service

import "testing"

func TestParseBatchResponse(t *testing.T) {
	t.Run("well-formed array", func(t *testing.T) {
		raw := `{"results":[
			{"id":"a/one","categoryId":"c1","path":"ai/llm","isNewCategory":false},
			{"id":"b/two","categoryId":"","path":"web","isNewCategory":true}
		]}`
		got, err := parseBatchResponse(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d elements, want 2", len(got))
		}
		if got["a/one"].Path != "ai/llm" || got["b/two"].IsNewCategory != true {
			t.Errorf("bad mapping: %+v", got)
		}
	})

	t.Run("wrapped in prose still parses", func(t *testing.T) {
		raw := "Sure! {\"results\":[{\"id\":\"x/y\",\"path\":\"tools\"}]} done"
		got, err := parseBatchResponse(raw)
		if err != nil || got["x/y"].Path != "tools" {
			t.Fatalf("got %+v err %v", got, err)
		}
	})

	t.Run("missing element is simply absent", func(t *testing.T) {
		raw := `{"results":[{"id":"only/one","path":"a"}]}`
		got, _ := parseBatchResponse(raw)
		if _, ok := got["missing/item"]; ok {
			t.Errorf("did not expect missing/item in results")
		}
	})

	t.Run("no results errors", func(t *testing.T) {
		if _, err := parseBatchResponse("no json here"); err == nil {
			t.Errorf("expected error")
		}
		if _, err := parseBatchResponse(`{"results":[]}`); err == nil {
			t.Errorf("expected error for empty results")
		}
	})
}
