package github

import (
	"reflect"
	"strings"
	"testing"
)

func names(ns ...string) []ReleaseAsset {
	a := make([]ReleaseAsset, len(ns))
	for i, n := range ns {
		a[i] = ReleaseAsset{Name: n}
	}
	return a
}

// Fixtures are the real latest-release asset names of well-known open-source
// apps (captured from the GitHub API), so the detector is validated against how
// projects actually name their builds, not idealized examples.
func TestDetectFromAssets_RealApps(t *testing.T) {
	cases := []struct {
		repo   string
		assets []ReleaseAsset
		want   []Platform
	}{
		{
			repo: "clash-verge-rev/clash-verge-rev",
			assets: names(
				"Clash.Verge-2.5.2-1.aarch64.rpm", "Clash.Verge-2.5.2-1.x86_64.rpm.sig",
				"Clash.Verge_2.5.2_aarch64.app.tar.gz", "Clash.Verge_2.5.2_aarch64.dmg",
				"Clash.Verge_2.5.2_amd64.deb", "Clash.Verge_2.5.2_arm64-setup.exe",
				"Clash.Verge_2.5.2_arm64-setup.exe.sig",
			),
			want: []Platform{PlatformMacOS, PlatformWindows, PlatformLinux},
		},
		{
			repo: "localsend/localsend",
			assets: names(
				"LocalSend-1.17.0-android-arm64v8.apk", "LocalSend-1.17.0-linux-x86-64.AppImage",
				"LocalSend-1.17.0-linux-x86-64.deb", "LocalSend-1.17.0-windows-x86-64.exe",
				"LocalSend-1.17.0-windows-x86-64.zip", "LocalSend-1.17.0.dmg",
			),
			want: []Platform{PlatformMacOS, PlatformWindows, PlatformLinux, PlatformAndroid},
		},
		{
			repo: "laurent22/joplin",
			assets: names(
				"Joplin-3.6.15-arm64.DMG", "Joplin-3.6.15-arm64.dmg.blockmap",
				"Joplin-3.6.15-arm64.pkg", "Joplin-3.6.15-mac.zip",
				"Joplin-3.6.15.AppImage", "Joplin-3.6.15.AppImage.sha512",
				"Joplin-3.6.15.deb", "Joplin-Setup-3.6.15.exe",
			),
			want: []Platform{PlatformMacOS, PlatformWindows, PlatformLinux},
		},
		{
			repo: "Genymobile/scrcpy",
			assets: names(
				"scrcpy-linux-x86_64-v4.1.tar.gz", "scrcpy-macos-aarch64-v4.1.tar.gz",
				"scrcpy-server-v4.1", "scrcpy-win64-v4.1.zip",
				"SHA256SUMS.txt", "SHA256SUMS.txt.asc",
			),
			want: []Platform{PlatformMacOS, PlatformWindows, PlatformLinux},
		},
	}

	for _, c := range cases {
		got, matches := DetectFromAssets(c.assets)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: platforms = %v, want %v", c.repo, got, c.want)
		}
		// Sidecar files (.sig/.blockmap/.sha512/.txt/.asc, and scrcpy-server with
		// no extension) must never appear as downloadable matches.
		for _, m := range matches {
			if isSidecarAsset(strings.ToLower(m.Name)) || platformOfAsset(m.Name) == "" {
				t.Errorf("%s: sidecar/unknown asset leaked into matches: %q", c.repo, m.Name)
			}
		}
	}
}

func TestPlatformFallbacks(t *testing.T) {
	// vscode attaches no binaries to GitHub releases (ships via its own site), so
	// only the topic fallback can place it — electron → the three desktop OSes.
	res := DetectPlatforms(nil, []string{"editor", "electron", "typescript"}, "app", "microsoft/vscode", "https://code.visualstudio.com", "TypeScript")
	want := []Platform{PlatformMacOS, PlatformWindows, PlatformLinux}
	if !reflect.DeepEqual(res.Platforms, want) {
		t.Errorf("vscode topic fallback = %v, want %v", res.Platforms, want)
	}
	if res.Source != "topic" {
		t.Errorf("vscode source = %q, want topic", res.Source)
	}

	// A web-stack app with a homepage and no binaries → web via heuristic.
	res = DetectPlatforms(nil, []string{"dashboard"}, "app", "acme/dashboard", "https://demo.example.com", "TypeScript")
	if len(res.Platforms) != 1 || res.Platforms[0] != PlatformWeb || res.Source != "heuristic" {
		t.Errorf("web heuristic = %v (src %q), want [web] heuristic", res.Platforms, res.Source)
	}

	// Assets present → source is authoritative "asset", topics don't downgrade it.
	res = DetectPlatforms(names("App-1.0.0.dmg"), []string{"electron"}, "app", "acme/app", "", "")
	if res.Source != "asset" {
		t.Errorf("source with assets = %q, want asset", res.Source)
	}
}
