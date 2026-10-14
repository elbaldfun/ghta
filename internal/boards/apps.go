package boards

import (
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// The app store. Its filters (platform × form × domain × shelf × sort) are
// too many to materialise as boards, but its corpus is small, so the boards
// job writes one row per app to a collection of its own (store.CollApps) and
// the api filters that. Best-of collections and the alternatives index are
// boards.
const (
	AppHot     = "hot"
	AppPopular = "popular"
	AppNew     = "new"

	// AppKindApp and AppKindCLI are the form filter: a window or a terminal.
	AppKindApp = "app"
	AppKindCLI = "cli"

	// DevGroup is the shelf group of the developer apps (judge.DevShelves).
	DevGroup = "dev"

	AltTargetsKey = "alternatives"
	AltTargetsN   = 200
	// BestOfN caps a best-of collection: past a dozen it stops being a pick.
	BestOfN = 12
	// bestOfPool is how deep into a shelf's most-starred apps the best-of
	// score looks.
	bestOfPool = 60
)

var (
	AppSorts = []string{AppHot, AppPopular, AppNew}
	AppOSes  = []string{"macos", "windows", "linux", "android", "ios", "web"}
)

func BestOfKey(shelf string) string { return "apps-best|" + shelf }

// AppSource is what the boards job reads of a candidate app.
type AppSource struct {
	ExternalID  string             `bson:"externalId"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	Language    string             `bson:"language"`
	Homepage    string             `bson:"homepage"`
	License     string             `bson:"license"`
	Metrics     map[string]float64 `bson:"metrics"`
	Growth      item.Growth        `bson:"growth"`
	Labels      item.Labels        `bson:"labels"`
	Assets      item.Assets        `bson:"assets"`
	Gh          *item.GitHub       `bson:"gh"`
}

// AppFilter selects the candidates: anything the store could list, and
// anything that names a product it replaces.
func AppFilter() bson.M {
	return bson.M{
		"source": item.SourceGitHub, "stale": bson.M{"$ne": true},
		"$or": bson.A{
			bson.M{"labels.type": bson.M{"$in": bson.A{"app", "cli"}}},
			bson.M{"gh.platforms.0": bson.M{"$exists": true}},
			bson.M{"labels.alternatives.0": bson.M{"$exists": true}},
		},
	}
}

func AppProjection() bson.M {
	return bson.M{
		"externalId": 1, "name": 1, "description": 1, "language": 1, "homepage": 1, "license": 1,
		"metrics.stars": 1, "growth.d1": 1, "assets": 1,
		"labels.domain": 1, "labels.type": 1, "labels.shelf": 1, "labels.hasGui": 1,
		"labels.taglineZh": 1, "labels.taglineEn": 1, "labels.alternatives": 1,
		"gh.platforms": 1, "gh.releaseAssets": 1, "gh.latestRelease": 1,
	}
}

// Download is the one installer a platform's button points at.
type Download struct {
	OS   string `bson:"os" json:"os"`
	URL  string `bson:"url" json:"url"`
	Name string `bson:"name" json:"name"`
}

// App is one row of the store, as the api serves it. The fields marked
// json:"-" exist for the filters.
type App struct {
	ExternalID      string             `bson:"_id" json:"externalId"`
	Name            string             `bson:"name" json:"name"`
	Description     string             `bson:"description,omitempty" json:"description,omitempty"`
	Language        string             `bson:"language,omitempty" json:"language,omitempty"`
	IconURL         string             `bson:"iconUrl,omitempty" json:"iconUrl,omitempty"`
	ScreenshotURL   string             `bson:"screenshotUrl,omitempty" json:"screenshotUrl,omitempty"`
	Homepage        string             `bson:"homepage,omitempty" json:"homepage,omitempty"`
	License         string             `bson:"license,omitempty" json:"license,omitempty"`
	Stars           int                `bson:"stars" json:"stars"`
	Growth          int                `bson:"growth" json:"growth"`
	Type            string             `bson:"type,omitempty" json:"type,omitempty"`
	Kind            string             `bson:"kind" json:"kind"`
	Shelf           string             `bson:"shelf,omitempty" json:"shelf,omitempty"`
	TaglineZh       string             `bson:"taglineZh,omitempty" json:"taglineZh,omitempty"`
	TaglineEn       string             `bson:"taglineEn,omitempty" json:"taglineEn,omitempty"`
	Platforms       []string           `bson:"platforms" json:"platforms"`
	LatestReleaseAt *time.Time         `bson:"latestReleaseAt,omitempty" json:"latestReleaseAt,omitempty"`
	Downloads       []Download         `bson:"downloads,omitempty" json:"downloads,omitempty"`
	HasDownloads    bool               `bson:"hasDownloads" json:"hasDownloads"`
	Alternatives    []item.Alternative `bson:"alternatives,omitempty" json:"alternatives,omitempty"`

	// Listed is false for a row kept only for the alternatives pages: a
	// library, or an app the judge kept off the shelves.
	Listed bool `bson:"listed" json:"-"`
	// Group is the shelf's group: its first segment, or DevGroup.
	Group   string    `bson:"group,omitempty" json:"-"`
	Domains []string  `bson:"domains,omitempty" json:"-"` // top-level domains
	At      time.Time `bson:"at" json:"-"`
}

// Key is the row's id in the apps collection.
func (a App) Key() string { return a.ExternalID }

// NewApp shapes a candidate into a store row; ok is false for one that is
// neither listed nor anyone's alternative.
func NewApp(s AppSource) (a App, ok bool) {
	l := s.Labels
	var platforms []string
	var assets []item.ReleaseAsset
	var latest *time.Time
	if s.Gh != nil {
		platforms, assets = s.Gh.Platforms, s.Gh.ReleaseAssets
		if r := s.Gh.LatestRelease; r != nil && !r.PublishedAt.IsZero() {
			t := r.PublishedAt.UTC()
			latest = &t
		}
	}
	listed := l.Type != "library" && l.Shelf != item.ShelfExcluded &&
		(l.Type == "app" || l.Type == "cli" || len(platforms) > 0)
	if !listed && len(l.Alternatives) == 0 {
		return App{}, false
	}
	// The judge's GUI verdict wins over the form: Windows Terminal is a cli
	// by form and very much a window.
	kind := AppKindApp
	if l.Type == "cli" {
		kind = AppKindCLI
	}
	if l.HasGui != nil && !*l.HasGui {
		kind = AppKindCLI
	} else if l.HasGui != nil {
		kind = AppKindApp
	}
	if platforms == nil {
		platforms = []string{}
	}
	a = App{
		ExternalID: s.ExternalID, Name: s.Name, Description: clipRunes(s.Description, 300), Language: s.Language,
		IconURL: s.Assets.IconURL, ScreenshotURL: mirrorRaw(s.Assets.ScreenshotURL),
		Homepage: s.Homepage, License: s.License, Stars: int(s.Metrics["stars"]),
		Type: l.Type, Kind: kind, TaglineZh: l.TaglineZh, TaglineEn: l.TaglineEn,
		Platforms: platforms, LatestReleaseAt: latest, Downloads: primaryDownloads(assets), HasDownloads: len(assets) > 0,
		Alternatives: l.Alternatives, Listed: listed, Domains: topDomains(l.Domain),
	}
	if s.Growth.D1 != nil {
		a.Growth = int(*s.Growth.D1)
	}
	if l.Shelf != item.ShelfExcluded {
		a.Shelf = l.Shelf
		a.Group = ShelfGroup(l.Shelf)
	}
	return a, true
}

// ShelfGroup is the group a shelf filter of one segment selects.
func ShelfGroup(shelf string) string {
	if shelf == "" {
		return ""
	}
	if slices.Contains(judge.DevShelves, shelf) {
		return DevGroup
	}
	group, _, _ := strings.Cut(shelf, "/")
	return group
}

// Shelves are every shelf a best-of collection can be asked for: the apps
// branch's leaves and the developer shelves the tree has.
func Shelves(tree taxonomy.Tree) []string {
	var out []string
	leaves := map[string]bool{}
	for _, n := range tree.Leaves() {
		leaves[n.Path] = true
		if taxonomy.IsShelfPath(n.Path) {
			out = append(out, taxonomy.ShelfSlug(n.Path))
		}
	}
	for _, s := range judge.DevShelves {
		if leaves[s] {
			out = append(out, s)
		}
	}
	return out
}

// BestOf is a shelf's collection: its most-starred listed apps, reordered by
// a score that also weighs the day's growth, a release in the last quarter,
// and something to download.
func BestOf(apps []App, shelf string, now time.Time) []App {
	var rows []App
	for _, a := range apps {
		if a.Listed && a.Shelf == shelf {
			rows = append(rows, a)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Stars != rows[j].Stars {
			return rows[i].Stars > rows[j].Stars
		}
		return rows[i].ExternalID < rows[j].ExternalID
	})
	rows = rows[:min(len(rows), bestOfPool)]
	score := func(a App) float64 {
		s := lnPlus(float64(a.Stars)) + 2*lnPlus(float64(a.Growth))
		if a.LatestReleaseAt != nil && now.Sub(*a.LatestReleaseAt) < 90*24*time.Hour {
			s += 2
		}
		if a.HasDownloads {
			s++
		}
		return s
	}
	sort.SliceStable(rows, func(i, j int) bool { return score(rows[i]) > score(rows[j]) })
	rows = rows[:min(len(rows), BestOfN)]
	if rows == nil {
		rows = []App{}
	}
	return rows
}

func lnPlus(v float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Log(v + 1)
}

// AltTarget is a product with the count of open-source apps that replace it.
type AltTarget struct {
	Name  string `bson:"name" json:"name"`
	Slug  string `bson:"slug" json:"slug"`
	Kind  string `bson:"kind,omitempty" json:"kind,omitempty"`
	Count int    `bson:"count" json:"count"`
}

// AltTargets are the products with the most alternatives, most first. A
// product's name is as its most-starred alternative gives it.
func AltTargets(apps []App) []AltTarget {
	byStars := slices.Clone(apps)
	sort.Slice(byStars, func(i, j int) bool { return byStars[i].Stars > byStars[j].Stars })
	idx := map[string]int{}
	var out []AltTarget
	for _, a := range byStars {
		for _, alt := range a.Alternatives {
			if alt.Slug == "" {
				continue
			}
			i, ok := idx[alt.Slug]
			if !ok {
				i = len(out)
				idx[alt.Slug] = i
				out = append(out, AltTarget{Name: alt.Name, Slug: alt.Slug, Kind: alt.Kind})
			}
			out[i].Count++
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Slug < out[j].Slug
	})
	out = out[:min(len(out), AltTargetsN)]
	if out == nil {
		out = []AltTarget{}
	}
	return out
}

// downloadPriority ranks installer extensions per platform, so the button
// points at a .dmg over a raw .zip and an installer .exe over an archive.
var downloadPriority = map[string][]string{
	"macos":   {".dmg", ".pkg", ".app.zip", ".app.tar.gz"},
	"windows": {".exe", ".msi", ".appx", ".msix"},
	"linux":   {".appimage", ".deb", ".rpm", ".flatpak", ".snap"},
	"android": {".apk", ".aab"},
	"ios":     {".ipa"},
}

// primaryDownloads picks the best asset per platform, platforms in a fixed order.
func primaryDownloads(assets []item.ReleaseAsset) []Download {
	rank := func(os, name string) int {
		lower := strings.ToLower(name)
		for i, ext := range downloadPriority[os] {
			if strings.HasSuffix(lower, ext) {
				return i
			}
		}
		return len(downloadPriority[os]) + 1
	}
	best := map[string]item.ReleaseAsset{}
	for _, a := range assets {
		if a.URL == "" {
			continue
		}
		if cur, ok := best[a.Platform]; !ok || rank(a.Platform, a.Name) < rank(a.Platform, cur.Name) {
			best[a.Platform] = a
		}
	}
	var out []Download
	for _, os := range []string{"macos", "windows", "linux", "android", "ios"} {
		if a, ok := best[os]; ok {
			out = append(out, Download{OS: os, URL: a.URL, Name: a.Name})
		}
	}
	return out
}

// mirrorRaw serves a raw.githubusercontent.com screenshot through jsDelivr,
// which mainland China can reach.
func mirrorRaw(u string) string {
	const raw = "https://raw.githubusercontent.com/"
	rest, ok := strings.CutPrefix(u, raw)
	if !ok {
		return u
	}
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) != 4 {
		return u
	}
	return "https://cdn.jsdelivr.net/gh/" + parts[0] + "/" + parts[1] + "@" + parts[2] + "/" + parts[3]
}

func topDomains(paths []string) []string {
	var out []string
	for _, p := range paths {
		top, _, _ := strings.Cut(p, "/")
		if !slices.Contains(out, top) {
			out = append(out, top)
		}
	}
	return out
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
