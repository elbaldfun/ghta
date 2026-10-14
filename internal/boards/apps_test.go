package boards

import (
	"testing"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
)

func appSource(id string, stars float64, l item.Labels, platforms ...string) AppSource {
	return AppSource{ExternalID: id, Name: id, Metrics: map[string]float64{"stars": stars}, Labels: l,
		Gh: &item.GitHub{Platforms: platforms}}
}

func TestNewApp(t *testing.T) {
	no, yes := false, true
	alt := []item.Alternative{{Name: "Photoshop", Slug: "photoshop"}}
	for _, tc := range []struct {
		name        string
		src         AppSource
		ok, listed  bool
		kind, group string
	}{
		{"cli form", appSource("a", 1, item.Labels{Type: "cli", Shelf: "system/files"}), true, true, "cli", "system"},
		{"gui verdict wins", appSource("a", 1, item.Labels{Type: "cli", HasGui: &yes, Shelf: "devtools/terminal"}), true, true, "app", "dev"},
		{"no gui", appSource("a", 1, item.Labels{Type: "app", HasGui: &no}), true, true, "cli", ""},
		{"platforms only", appSource("a", 1, item.Labels{Type: "software"}, "macos"), true, true, "app", ""},
		{"excluded keeps alternatives", appSource("a", 1, item.Labels{Type: "app", Shelf: item.ShelfExcluded, Alternatives: alt}), true, false, "app", ""},
		{"library with alternatives", appSource("a", 1, item.Labels{Type: "library", Alternatives: alt}, "linux"), true, false, "app", ""},
		{"library", appSource("a", 1, item.Labels{Type: "library"}, "linux"), false, false, "", ""},
		{"excluded", appSource("a", 1, item.Labels{Type: "app", Shelf: item.ShelfExcluded}), false, false, "", ""},
	} {
		a, ok := NewApp(tc.src)
		if ok != tc.ok || a.Listed != tc.listed || (ok && (a.Kind != tc.kind || a.Group != tc.group)) {
			t.Errorf("%s: ok=%v listed=%v kind=%q group=%q", tc.name, ok, a.Listed, a.Kind, a.Group)
		}
		if ok && a.Shelf == item.ShelfExcluded {
			t.Errorf("%s: the excluded verdict must not reach the row", tc.name)
		}
	}
}

func TestNewAppDownloads(t *testing.T) {
	src := appSource("o/a", 1, item.Labels{Type: "app"}, "macos", "windows")
	src.Gh.ReleaseAssets = []item.ReleaseAsset{
		{Name: "a.zip", Platform: "macos", URL: "u1"}, {Name: "a.dmg", Platform: "macos", URL: "u2"},
		{Name: "a.exe", Platform: "windows", URL: "u3"}, {Name: "a.deb", Platform: "linux"},
	}
	src.Assets.ScreenshotURL = "https://raw.githubusercontent.com/o/a/HEAD/docs/shot.png"
	src.Labels.Domain = []string{"apps/system/files", "devtools/terminal", "apps/media/player"}
	a, _ := NewApp(src)
	if len(a.Downloads) != 2 || a.Downloads[0].URL != "u2" || a.Downloads[1].OS != "windows" || !a.HasDownloads {
		t.Fatalf("downloads = %+v", a.Downloads)
	}
	if a.ScreenshotURL != "https://cdn.jsdelivr.net/gh/o/a@HEAD/docs/shot.png" {
		t.Fatalf("screenshot = %s", a.ScreenshotURL)
	}
	if len(a.Domains) != 2 || a.Domains[0] != "apps" || a.Domains[1] != "devtools" {
		t.Fatalf("domains = %v", a.Domains)
	}
}

func TestBestOfAndAltTargets(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour)
	ps := item.Alternative{Name: "Photoshop", Slug: "photoshop"}
	apps := []App{
		{ExternalID: "big", Stars: 50000, Shelf: "media/photo", Listed: true},
		{ExternalID: "fresh", Stars: 20000, Growth: 400, LatestReleaseAt: &recent, HasDownloads: true, Shelf: "media/photo", Listed: true,
			Alternatives: []item.Alternative{{Name: "Adobe Photoshop", Slug: "photoshop"}}},
		{ExternalID: "hidden", Stars: 90000, Shelf: "media/photo", Alternatives: []item.Alternative{ps}},
		{ExternalID: "other", Stars: 1, Shelf: "media/player", Listed: true, Alternatives: []item.Alternative{{Name: "VLC", Slug: "vlc"}}},
	}
	best := BestOf(apps, "media/photo", now)
	if len(best) != 2 || best[0].ExternalID != "fresh" {
		t.Fatalf("best = %+v; growth and a fresh release outweigh 2.5x the stars, unlisted rows stay out", best)
	}
	if got := BestOf(apps, "games/games", now); got == nil || len(got) != 0 {
		t.Fatalf("empty shelf = %#v; want an empty list", got)
	}
	targets := AltTargets(apps)
	if len(targets) != 2 || targets[0].Slug != "photoshop" || targets[0].Count != 2 || targets[0].Name != "Photoshop" {
		t.Fatalf("targets = %+v; the most-starred app names the product", targets)
	}
}
