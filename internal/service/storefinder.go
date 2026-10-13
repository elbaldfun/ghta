package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/provider"
)

// CurrentStoreVersion is the prompt/enum generation. Bumping it re-queues every
// item for re-judgement (the job picks up store.version < CurrentStoreVersion),
// so the enum can evolve without deleting anything. v2 = taxonomy v3 shelves:
// the migration stamps mechanically-mapped items with this version so only the
// rejudge shelves (devtools/devops) actually go back through the model.
const CurrentStoreVersion = 2

// AppShelves is the consumer-shelf enum: the leaves of the taxonomy's apps
// branch minus the "apps/" prefix (taxonomy.ShelfSlug). Slugs are frozen per
// store version; display names live in the frontend i18n files. Keep this in
// step with taxonomy.yaml — TestShelvesMatchTree enforces it.
var AppShelves = []string{
	"productivity/notes", "productivity/todo", "productivity/docs", "productivity/knowledge",
	"creative/image", "creative/video", "creative/audio", "creative/whiteboard", "creative/cad", "creative/writing",
	"ai/assistant", "ai/local-llm", "ai/generation", "ai/builder",
	"selfhosted/cloud", "selfhosted/media-server", "selfhosted/monitoring", "selfhosted/tunnel", "selfhosted/automation",
	"network/browser", "network/download", "network/proxy", "network/remote", "network/browser-ext",
	"media/player", "media/music", "media/reader",
	"social/messaging", "social/tools",
	"system/screenshot", "system/launcher", "system/files", "system/transfer", "system/phone", "system/backup",
	"security/passwords", "security/privacy",
	"games/games", "games/emulator",
}

// DevShelves are the developer-facing apps that left the consumer shelves in
// taxonomy v3. They are plain domain leaves (the item's store.category holds
// the leaf path), surfaced in the store under one "dev" view rather than as
// consumer shelves, so the storefront's first screen is for end users.
var DevShelves = []string{
	"devtools/terminal", "devtools/editors", "devtools/api-tools", "devtools/vcs", "devtools/package-manager", "devtools/code-quality",
	"data/database", "infra/containers", "infra/automation", "infra/monitoring",
	"gamedev/engines", "security/offensive", "agents/coding",
}

// DevShelfGroup is the pseudo-major that selects every DevShelves entry.
const DevShelfGroup = "dev"

// StoreExcluded marks "not a runnable app" (framework/library/list/service).
const StoreExcluded = "excluded"

// validShelf whitelists LLM output; anything else is an item-level failure.
var validShelf = func() map[string]bool {
	m := map[string]bool{StoreExcluded: true}
	for _, s := range AppShelves {
		m[s] = true
	}
	for _, s := range DevShelves {
		m[s] = true
	}
	return m
}()

// ValidShelfSlug reports whether s is a known shelf slug (consumer or dev) or "excluded".
func ValidShelfSlug(s string) bool { return validShelf[s] }

// IsDevShelf reports whether s is one of the developer-app leaves.
func IsDevShelf(s string) bool {
	for _, d := range DevShelves {
		if d == s {
			return true
		}
	}
	return false
}

// ShelfDomainPath is the taxonomy leaf an effective shelf corresponds to:
// consumer shelves live under apps/, dev shelves are the leaf itself.
func ShelfDomainPath(slug string) string {
	if IsDevShelf(slug) {
		return slug
	}
	return "apps/" + slug
}

const storeSystemPrompt = `You classify open-source GitHub projects for an app store of downloadable, runnable software.

For EACH project decide:

1. "category": which ONE shelf it belongs on, from EXACTLY this list (copy the slug verbatim).
   Consumer shelves (software a non-developer would install):
%s
   Developer shelves (software developers install for their own work; pick these for terminals, editors, API clients, DB clients, git tools, package managers, linters, containers/CI/monitoring tools, game engines, offensive-security tools, coding agents):
%s
   OR "excluded" if it is NOT a runnable application: a framework, a library/SDK, an awesome/list/tutorial/interview collection, a dataset, a spec, or a hosted-only service with nothing to download or self-host. Rule of thumb: "excluded" unless a user can download/run/self-host it. CLI tools ARE apps (yt-dlp -> network/download). Game engines with a downloadable editor ARE apps (gamedev/engines).

2. "taglineZh": ONE plain-Chinese sentence (<=20 chars) telling a normal user what this app does for them. Plain words, no hype, no "开源" prefix. Example: "跨平台截图与标注工具".

3. "taglineEn": the same in plain English (<=70 chars).

4. "hasGui": true if it has a graphical/web UI a user interacts with; false if it is command-line/daemon only.

Return STRICT JSON only:
{"results":{"<owner/repo>":{"category":"...","taglineZh":"...","taglineEn":"...","hasGui":true}, ...}}
Every input id MUST appear as a key. Do not invent slugs.`

// StoreResult is one item's verdict.
type StoreResult struct {
	Category  string `json:"category"`
	TaglineZh string `json:"taglineZh"`
	TaglineEn string `json:"taglineEn"`
	HasGui    bool   `json:"hasGui"`
}

// storePromptLine renders one project for the batch prompt. The README excerpt
// is the key addition over the alternatives prompt: a one-line dev description
// is too thin to write an honest tagline for long-tail repos.
func storePromptLine(it domain.TrackedItem) string {
	desc := truncate(it.Description, 200)
	topics := ""
	if sd := it.SourceData; sd != nil {
		if tn, ok := sd["topicNames"].([]any); ok && len(tn) > 0 {
			parts := make([]string, 0, len(tn))
			for _, t := range tn {
				if s, ok := t.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 8 {
				parts = parts[:8]
			}
			topics = strings.Join(parts, ",")
		}
	}
	platforms := ""
	if sd := it.SourceData; sd != nil {
		if ps, ok := sd["platforms"].([]any); ok && len(ps) > 0 {
			parts := make([]string, 0, len(ps))
			for _, p := range ps {
				if s, ok := p.(string); ok {
					parts = append(parts, s)
				}
			}
			platforms = strings.Join(parts, ",")
		}
	}
	readme := readmeExcerpt(it, 300)
	return fmt.Sprintf("id=%s | %s | type=%s | topics=%s | platforms=%s | readme: %s",
		it.ExternalID, desc, it.Type, topics, platforms, readme)
}

// readmeExcerpt returns the first n runes of the README with markdown noise
// (badges, links, headings, html) stripped — enough signal for a tagline
// without blowing up the prompt.
func readmeExcerpt(it domain.TrackedItem, n int) string {
	sd := it.SourceData
	if sd == nil {
		return ""
	}
	raw, _ := sd["readme"].(string)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "<") ||
			strings.HasPrefix(l, "[!") || strings.HasPrefix(l, "!") ||
			strings.HasPrefix(l, "|") || strings.HasPrefix(l, "---") {
			continue
		}
		b.WriteString(l)
		b.WriteByte(' ')
		if b.Len() > n*4 { // rough byte bound before the rune cut
			break
		}
	}
	s := b.String()
	if utf8.RuneCountInString(s) > n {
		runes := []rune(s)
		s = string(runes[:n])
	}
	return s
}

func buildStorePrompt(items []domain.TrackedItem) string {
	var b strings.Builder
	b.WriteString("Projects:\n")
	for _, it := range items {
		b.WriteString(storePromptLine(it))
		b.WriteByte('\n')
	}
	return b.String()
}

type storeResponse struct {
	Results map[string]StoreResult `json:"results"`
}

// InferStore judges one batch. A non-nil error means the WHOLE call failed
// (relay down / unparseable) — callers count it against the batch's failCount
// but must not record verdicts. Item-level problems (missing id, invalid slug)
// surface as absent entries in the returned map, so one bad item never poisons
// its batchmates.
func InferStore(ctx context.Context, p provider.Provider, items []domain.TrackedItem) (map[string]StoreResult, error) {
	if len(items) == 0 {
		return map[string]StoreResult{}, nil
	}
	system := fmt.Sprintf(storeSystemPrompt, strings.Join(AppShelves, ", "), strings.Join(DevShelves, ", "))
	raw, err := p.AnalyzeJSON(ctx, system, buildStorePrompt(items))
	if err != nil {
		return nil, err
	}
	var parsed storeResponse
	if err := json.Unmarshal([]byte(extractJSON(raw)), &parsed); err != nil {
		return nil, fmt.Errorf("parse store response: %w", err)
	}

	out := make(map[string]StoreResult, len(items))
	for _, it := range items {
		r, ok := parsed.Results[it.ExternalID]
		if !ok {
			continue // omitted by the model -> item-level failure
		}
		r.Category = strings.TrimSpace(r.Category)
		if !ValidShelfSlug(r.Category) {
			continue // invented slug -> item-level failure, don't guess
		}
		// Programmatic bounds — never trust the model to honor length limits.
		r.TaglineZh = truncateRunes(strings.TrimSpace(r.TaglineZh), 24)
		r.TaglineEn = truncate(strings.TrimSpace(r.TaglineEn), 90)
		out[it.ExternalID] = r
	}
	return out, nil
}

// truncateRunes cuts by runes (CJK-safe), unlike byte-based truncate.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
