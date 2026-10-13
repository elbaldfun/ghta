package service

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

// Items classified before multi-label store categoryPath as a bare string.
// A row holding one must still decode, or the whole app listing it appears in
// answers 500.
func TestAppRowDecodesLegacyStringCategoryPath(t *testing.T) {
	for name, tc := range map[string]struct {
		in   any
		want int
	}{
		"array":        {bson.A{"web/frontend", "apps/ai/assistant"}, 2},
		"string":       {"web/frontend", 1},
		"empty string": {"", 0},
	} {
		raw, err := bson.Marshal(bson.M{"categoryPath": tc.in})
		if err != nil {
			t.Fatal(err)
		}
		var r appRow
		if err := bson.Unmarshal(raw, &r); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(r.CategoryPath) != tc.want {
			t.Errorf("%s: got %v", name, r.CategoryPath)
		}
	}
}
