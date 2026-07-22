package service

import "testing"

func TestParseBatchResponse(t *testing.T) {
	t.Run("well-formed object", func(t *testing.T) {
		raw := `{"results":[
			{"id":"a/one","paths":["ai/llm"],"type":"library","isNewCategory":false},
			{"id":"b/two","paths":["web"],"isNewCategory":true}
		]}`
		got, err := parseBatchResponse(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d elements, want 2", len(got))
		}
		if len(got["a/one"].Paths) != 1 || got["a/one"].Paths[0] != "ai/llm" || !got["b/two"].IsNewCategory {
			t.Errorf("bad mapping: %+v", got)
		}
	})

	t.Run("bare top-level array (grok) parses", func(t *testing.T) {
		raw := `[{"id":"x/y","paths":["devtools/cli"],"type":"cli"}]`
		got, err := parseBatchResponse(raw)
		if err != nil || len(got["x/y"].Paths) != 1 || got["x/y"].Paths[0] != "devtools/cli" {
			t.Fatalf("got %+v err %v", got, err)
		}
	})

	t.Run("wrapped in prose still parses", func(t *testing.T) {
		raw := "Sure! {\"results\":[{\"id\":\"x/y\",\"paths\":[\"tools\"]}]} done"
		got, err := parseBatchResponse(raw)
		if err != nil || len(got["x/y"].Paths) != 1 || got["x/y"].Paths[0] != "tools" {
			t.Fatalf("got %+v err %v", got, err)
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
