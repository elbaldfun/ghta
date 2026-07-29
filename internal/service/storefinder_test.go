package service

import (
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/domain"
)

func TestValidShelfSlug(t *testing.T) {
	for _, ok := range []string{"system/screenshot", "network/download", "excluded"} {
		if !ValidShelfSlug(ok) {
			t.Errorf("ValidShelfSlug(%q) = false, want true", ok)
		}
	}
	// The exact failure modes the review predicted: plural drift, bare major,
	// invented slugs.
	for _, bad := range []string{"system/screenshots", "system", "notes", "utilities/misc", ""} {
		if ValidShelfSlug(bad) {
			t.Errorf("ValidShelfSlug(%q) = true, want false", bad)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("跨平台截图与标注工具,功能强大又好用,支持全平台运行哦", 24); len([]rune(got)) != 24 {
		t.Errorf("truncateRunes rune len = %d, want 24", len([]rune(got)))
	}
	if got := truncateRunes("短句", 24); got != "短句" {
		t.Errorf("short string altered: %q", got)
	}
}

func TestReadmeExcerpt(t *testing.T) {
	it := domain.TrackedItem{SourceData: map[string]any{"readme": strings.Join([]string{
		"# Title",
		"[![badge](https://img.shields.io/x)](y)",
		"<p align=center><img src=logo.png></p>",
		"| a | b |",
		"Flameshot is a powerful yet simple to use screenshot software.",
		"---",
		"More prose here.",
	}, "\n")}}
	got := readmeExcerpt(it, 300)
	if !strings.Contains(got, "screenshot software") {
		t.Errorf("prose line missing: %q", got)
	}
	for _, noise := range []string{"badge", "img src", "# Title", "| a |"} {
		if strings.Contains(got, noise) {
			t.Errorf("noise %q leaked into excerpt: %q", noise, got)
		}
	}
}
