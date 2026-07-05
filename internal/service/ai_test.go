package service

import "testing"

func TestParseAIResponse(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantPath string
		wantNew  bool
		wantErr  bool
	}{
		{
			name:     "bare json",
			raw:      `{"categoryId":"c1","path":"ai/llm","isNewCategory":false}`,
			wantPath: "ai/llm",
		},
		{
			name:     "fenced json",
			raw:      "here you go:\n```json\n{\"categoryId\":\"\",\"path\":\"blockchain\",\"isNewCategory\":true}\n```",
			wantPath: "blockchain",
			wantNew:  true,
		},
		{
			name:     "prose with braces",
			raw:      `The best fit is: {"categoryId":"c2","path":"web/frameworks"} — hope this helps`,
			wantPath: "web/frameworks",
		},
		{
			name:    "no json",
			raw:     "I cannot categorize this repository.",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseAIResponse(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", res.Path, tt.wantPath)
			}
			if res.IsNewCategory != tt.wantNew {
				t.Errorf("isNewCategory = %v, want %v", res.IsNewCategory, tt.wantNew)
			}
		})
	}
}
