package service

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
