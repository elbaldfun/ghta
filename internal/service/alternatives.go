package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/provider"
)

const altSystemPrompt = `You label open-source projects with the commercial or paid software they are a direct alternative to — the way AlternativeTo.net does.

Rules:
- List a product ONLY if the project is a genuine like-for-like replacement someone would use INSTEAD of paying for that product. A rough overlap is not enough.
- Use the canonical product name: "Notion", "Figma", "Postman", "1Password", "Adobe Photoshop", "Jira", "Slack", "Zoom", "ChatGPT", "Zapier", "Airtable".
- Most projects are NOT alternatives to any well-known paid product (libraries, frameworks, niche dev tools, servers). For those return an empty array. Empty is the common, correct answer — do not force a match.
- Never invent products or list open-source projects as the target. Only real, well-known commercial/paid products.
- At most 3 per project, most relevant first.

Return STRICT JSON only, no prose:
{"results":{"<owner/repo>":[{"name":"<product>","kind":"<short category e.g. note-taking, design, api-client, password-manager>"}], ...}}
Every input id MUST appear as a key, with an array (possibly empty).`

// altInput is one project line in the batch prompt.
func altPromptLine(it domain.TrackedItem) string {
	desc := it.Description
	if len(desc) > 240 {
		desc = truncate(desc, 240)
	}
	topics := ""
	if sd := it.SourceData; sd != nil {
		if tn, ok := sd["topicNames"].([]any); ok && len(tn) > 0 {
			parts := make([]string, 0, len(tn))
			for _, t := range tn {
				if s, ok := t.(string); ok {
					parts = append(parts, s)
				}
			}
			topics = strings.Join(parts, ", ")
		}
	}
	return fmt.Sprintf("id=%s | %s | %s | lang=%s | topics=%s | domains=%s",
		it.ExternalID, it.Name, desc, it.Language, topics, strings.Join(it.CategoryPath, ","))
}

func buildAltPrompt(items []domain.TrackedItem) string {
	var b strings.Builder
	b.WriteString("Projects:\n")
	for _, it := range items {
		b.WriteString(altPromptLine(it))
		b.WriteByte('\n')
	}
	return b.String()
}

type altResponse struct {
	Results map[string][]struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	} `json:"results"`
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

// slugify makes a url-safe key: "Adobe Photoshop" -> "adobe-photoshop".
func slugify(name string) string {
	s := slugStrip.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	return strings.Trim(s, "-")
}

// InferAlternatives asks the LLM which paid products each item is an open-source
// alternative to. Returns a result per input id (empty slice when none). A
// non-nil error means the whole call/parse failed — callers must treat that as
// "retry later", never as "this batch has no alternatives".
func InferAlternatives(ctx context.Context, p provider.Provider, items []domain.TrackedItem) (map[string][]domain.Alternative, error) {
	if len(items) == 0 {
		return map[string][]domain.Alternative{}, nil
	}
	raw, err := p.AnalyzeJSON(ctx, altSystemPrompt, buildAltPrompt(items))
	if err != nil {
		return nil, err
	}
	var parsed altResponse
	if err := json.Unmarshal([]byte(extractJSON(raw)), &parsed); err != nil {
		return nil, fmt.Errorf("parse alternatives: %w", err)
	}

	out := make(map[string][]domain.Alternative, len(items))
	for _, it := range items {
		out[it.ExternalID] = []domain.Alternative{} // default: processed, none
	}
	for id, alts := range parsed.Results {
		if _, ok := out[id]; !ok {
			continue // ignore ids we didn't ask about
		}
		seen := map[string]bool{}
		list := []domain.Alternative{}
		for _, a := range alts {
			name := strings.TrimSpace(a.Name)
			slug := slugify(name)
			if name == "" || slug == "" || seen[slug] {
				continue
			}
			seen[slug] = true
			list = append(list, domain.Alternative{Name: name, Slug: slug, Kind: strings.TrimSpace(a.Kind)})
			if len(list) >= 3 {
				break
			}
		}
		out[id] = list
	}
	return out, nil
}

// extractJSON pulls the first {...} object out of a reply, tolerating code fences
// or stray prose from models that ignore the JSON-only instruction.
func extractJSON(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
