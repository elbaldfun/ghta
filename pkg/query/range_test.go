package query

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		expr    string
		want    bson.M
		wantErr bool
	}{
		{"", nil, false},
		{"1000..2000", bson.M{"$gte": 1000.0, "$lte": 2000.0}, false},
		{"0..100", bson.M{"$gte": 0.0, "$lte": 100.0}, false}, // 0 is a valid boundary
		{">1000", bson.M{"$gt": 1000.0}, false},
		{"<1000", bson.M{"$lt": 1000.0}, false},
		{"1000", bson.M{"$eq": 1000.0}, false},
		{"0", bson.M{"$eq": 0.0}, false},
		{"abc", nil, true},
		{"1000..x", nil, true},
	}
	for _, tt := range tests {
		got, err := ParseRange(tt.expr)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRange(%q) expected error, got %v", tt.expr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRange(%q) unexpected error: %v", tt.expr, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("ParseRange(%q) = %v, want %v", tt.expr, got, tt.want)
			continue
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("ParseRange(%q)[%s] = %v, want %v", tt.expr, k, got[k], v)
			}
		}
	}
}
