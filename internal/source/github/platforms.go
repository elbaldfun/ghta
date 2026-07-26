package github

import "strings"

// Platform is a normalized target OS/runtime for a shipped, downloadable app.
type Platform string

const (
	PlatformMacOS   Platform = "macos"
	PlatformWindows Platform = "windows"
	PlatformLinux   Platform = "linux"
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformWeb     Platform = "web"
)

// platformOrder fixes the output ordering so the stored/serialized set is stable.
var platformOrder = []Platform{
	PlatformMacOS, PlatformWindows, PlatformLinux, PlatformAndroid, PlatformIOS, PlatformWeb,
}

// ReleaseAsset is one downloadable file attached to a GitHub release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// AssetMatch is a release asset that resolved to a platform — the raw material
// for the detail-page "download for your OS" section.
type AssetMatch struct {
	Name     string   `json:"name" bson:"name"`
	Platform Platform `json:"platform" bson:"platform"`
	URL      string   `json:"url,omitempty" bson:"url,omitempty"`
	Size     int64    `json:"size,omitempty" bson:"size,omitempty"`
}

// PlatformResult is the outcome of platform detection for one repo.
type PlatformResult struct {
	Platforms []Platform   `json:"platforms"`
	Source    string       `json:"platformSource"` // asset | topic | heuristic | ""
	Assets    []AssetMatch `json:"releaseAssets,omitempty"`
}

// isSidecarAsset reports whether a filename is a signature, checksum, update
// manifest, or other non-runnable companion file that must not be mistaken for a
// downloadable build. `name` must be lowercased.
func isSidecarAsset(name string) bool {
	if hasAnySuffix(name,
		".sig", ".asc", ".blockmap", ".sha1", ".sha256", ".sha512", ".md5",
		".pem", ".txt", ".yml", ".yaml", ".json", ".metalink", ".cat", ".sbom",
	) {
		return true
	}
	return strings.Contains(name, "sha256sums") || strings.Contains(name, "sha512sums") ||
		strings.Contains(name, "checksums")
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// platformOfAsset resolves one asset filename to a platform, or "" when it is a
// sidecar, source archive, or otherwise not a platform-specific build. Matching
// is case-insensitive and deliberately conservative: an archive (.zip/.tar.*)
// only counts when it carries an explicit OS keyword, so source tarballs and
// ambiguous zips are dropped rather than guessed.
func platformOfAsset(name string) Platform {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || isSidecarAsset(n) {
		return ""
	}

	// macOS app bundles are archives but unambiguous, so check before the generic
	// archive-needs-a-keyword rule below.
	if hasAnySuffix(n, ".app.zip", ".app.tar.gz") {
		return PlatformMacOS
	}
	switch {
	case hasAnySuffix(n, ".dmg", ".pkg"):
		return PlatformMacOS
	case hasAnySuffix(n, ".exe", ".msi", ".appx", ".msix"):
		return PlatformWindows
	case hasAnySuffix(n, ".appimage", ".deb", ".rpm", ".snap", ".flatpak", ".pacman"):
		return PlatformLinux
	case hasAnySuffix(n, ".apk", ".aab"):
		return PlatformAndroid
	case hasAnySuffix(n, ".ipa"):
		return PlatformIOS
	}

	// Archives are only assigned when they name an OS explicitly.
	if hasAnySuffix(n, ".zip", ".tar.gz", ".tgz", ".tar.xz", ".tar.bz2", ".7z") {
		switch {
		case containsAny(n, "windows", "win32", "win64", "-win-", "-win.", "-win_"):
			return PlatformWindows
		case containsAny(n, "darwin", "macos", "osx", "-mac.", "-mac-", "-mac_"):
			return PlatformMacOS
		case containsAny(n, "linux"):
			return PlatformLinux
		}
	}
	return ""
}

// DetectFromAssets returns the deduped platform set and the per-asset matches
// from a release's assets.
func DetectFromAssets(assets []ReleaseAsset) ([]Platform, []AssetMatch) {
	seen := map[Platform]bool{}
	var matches []AssetMatch
	for _, a := range assets {
		p := platformOfAsset(a.Name)
		if p == "" {
			continue
		}
		seen[p] = true
		matches = append(matches, AssetMatch{Name: a.Name, Platform: p, URL: a.DownloadURL, Size: a.Size})
	}
	return ordered(seen), matches
}

// topicPlatforms maps a topic to the platforms it implies. Single-OS topics map
// directly; cross-platform frameworks imply their usual target set.
var topicPlatforms = map[string][]Platform{
	"macos": {PlatformMacOS}, "osx": {PlatformMacOS}, "macos-app": {PlatformMacOS},
	"windows": {PlatformWindows}, "win32": {PlatformWindows},
	"linux":   {PlatformLinux},
	"android": {PlatformAndroid}, "ios": {PlatformIOS}, "ipados": {PlatformIOS},
	// desktop frameworks → the three desktop OSes
	"electron": {PlatformMacOS, PlatformWindows, PlatformLinux},
	"tauri":    {PlatformMacOS, PlatformWindows, PlatformLinux},
	"wails":    {PlatformMacOS, PlatformWindows, PlatformLinux},
	// mobile frameworks → the two mobile OSes
	"react-native": {PlatformAndroid, PlatformIOS},
	"ionic":        {PlatformAndroid, PlatformIOS},
	"capacitor":    {PlatformAndroid, PlatformIOS},
	"cordova":      {PlatformAndroid, PlatformIOS},
	"expo":         {PlatformAndroid, PlatformIOS},
	"flutter":      {PlatformAndroid, PlatformIOS},
	// web
	"pwa": {PlatformWeb}, "progressive-web-app": {PlatformWeb},
	"webapp": {PlatformWeb}, "web-app": {PlatformWeb}, "spa": {PlatformWeb},
}

// DetectFromTopics infers platforms from repo topics — the fallback for apps
// that ship via stores or their own site instead of GitHub release binaries.
func DetectFromTopics(topics []string) []Platform {
	seen := map[Platform]bool{}
	for _, tp := range topics {
		for _, p := range topicPlatforms[strings.ToLower(strings.TrimSpace(tp))] {
			seen[p] = true
		}
	}
	return ordered(seen)
}

var webLanguages = map[string]bool{
	"javascript": true, "typescript": true, "vue": true, "svelte": true, "html": true,
}

// looksLikeWebApp is the heuristic layer for web apps that ship no binaries: an
// application on a web stack, with a homepage/demo, and no native build. It is
// the fuzziest signal (a homepage may be only docs), so callers tag the result
// as heuristic and the UI flags it as inferred. A "-desktop" repo name is a
// strong native-app signal that vetoes the guess — otherwise an Electron desktop
// app that ships off-GitHub with no topics (e.g. Signal-Desktop) reads as web.
func looksLikeWebApp(itemType, name, homepage, language string, hasNativeAssets bool) bool {
	if strings.Contains(strings.ToLower(name), "desktop") {
		return false
	}
	return itemType == "app" && strings.TrimSpace(homepage) != "" &&
		webLanguages[strings.ToLower(language)] && !hasNativeAssets
}

// DetectPlatforms combines the layers: authoritative asset parsing first, topic
// inference as fallback, and the web heuristic last. Source records the strongest
// signal that contributed, so the UI can distinguish verified from inferred.
func DetectPlatforms(assets []ReleaseAsset, topics []string, itemType, name, homepage, language string) PlatformResult {
	assetSet, matches := DetectFromAssets(assets)
	seen := map[Platform]bool{}
	for _, p := range assetSet {
		seen[p] = true
	}
	source := ""
	if len(assetSet) > 0 {
		source = "asset"
	}

	for _, p := range DetectFromTopics(topics) {
		if !seen[p] {
			if source == "" {
				source = "topic"
			}
			seen[p] = true
		}
	}

	// The web heuristic is a last resort: only when no asset or topic signal has
	// placed the repo on any platform. Otherwise an Electron/Tauri desktop app
	// written in TypeScript (with a homepage, no GitHub binaries) would be
	// mislabelled web on top of its real desktop platforms.
	if len(seen) == 0 && looksLikeWebApp(itemType, name, homepage, language, len(assetSet) > 0) {
		seen[PlatformWeb] = true
		source = "heuristic"
	}

	return PlatformResult{Platforms: ordered(seen), Source: source, Assets: matches}
}

// ordered returns the set in the fixed platformOrder for stable output.
func ordered(seen map[Platform]bool) []Platform {
	out := make([]Platform, 0, len(seen))
	for _, p := range platformOrder {
		if seen[p] {
			out = append(out, p)
		}
	}
	return out
}
