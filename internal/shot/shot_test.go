package shot

import (
	"reflect"
	"strings"
	"testing"
)

func TestCandidates(t *testing.T) {
	readme := strings.Join([]string{
		"# MyApp",
		"[![Build](https://img.shields.io/badge/build-passing-green)](x)",
		"[![Coverage](https://codecov.io/gh/a/b/badge.svg)](y)",
		`<p align="center"><img src="docs/logo.png" width="120"></p>`,
		"[![Star History](https://api.star-history.com/svg?repos=a/b)](z)",
		"A powerful app.",
		"![screenshot](docs/screenshot-main.png)",
		`<img src="https://user-images.githubusercontent.com/1/demo.png">`,
		"![another](./assets/settings.jpg)",
	}, "\n")

	got := Candidates(readme, "acme/myapp")
	want := []string{
		"https://raw.githubusercontent.com/acme/myapp/HEAD/docs/screenshot-main.png",
		"https://user-images.githubusercontent.com/1/demo.png",
		"https://raw.githubusercontent.com/acme/myapp/HEAD/assets/settings.jpg",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Candidates = %v\nwant %v", got, want)
	}
}

func TestURLFilters(t *testing.T) {
	bad := []string{
		"https://img.shields.io/badge/x.png",            // badge host
		"https://example.com/logo.png",                  // logo word
		"https://example.com/app-banner.png",            // banner word
		"https://example.com/pic.svg",                   // svg
		"https://example.com/download-on-app-store.png", // store badge
		"https://api.star-history.com/chart.png",        // chart host
	}
	for _, u := range bad {
		if urlLooksLikeShot(u) {
			t.Errorf("urlLooksLikeShot(%q) = true, want false", u)
		}
	}
	if !urlLooksLikeShot("https://user-images.githubusercontent.com/1/shot.png") {
		t.Error("real screenshot URL rejected")
	}
}

func TestResolve(t *testing.T) {
	cases := map[string]string{
		"docs/a.png":              "https://raw.githubusercontent.com/o/r/HEAD/docs/a.png",
		"./docs/a.png":            "https://raw.githubusercontent.com/o/r/HEAD/docs/a.png",
		"/docs/a.png":             "https://raw.githubusercontent.com/o/r/HEAD/docs/a.png",
		"https://x.com/a.png":     "https://x.com/a.png",
		"//cdn.x.com/a.png":       "https://cdn.x.com/a.png",
		"data:image/png;base64,x": "",
	}
	for in, want := range cases {
		if got := resolve(in, "o/r"); got != want {
			t.Errorf("resolve(%q) = %q, want %q", in, got, want)
		}
	}
}
