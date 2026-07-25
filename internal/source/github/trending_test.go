package github

import (
	"os"
	"testing"

	"golang.org/x/net/html"
)

// TestParseTrending guards the scraper against silent breakage when GitHub
// changes the trending page markup: it parses a saved fixture and asserts the
// fields we depend on are extracted.
func TestParseTrending(t *testing.T) {
	f, err := os.Open("testdata/trending_weekly.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	repos := parseTrending(doc)
	if len(repos) < 10 {
		t.Fatalf("expected ~25 repos from a full trending page, got %d", len(repos))
	}

	// Every row must have the essentials; the whole point is the weekly delta.
	for i, r := range repos {
		if r.ExternalID == "" || r.Owner == "" || r.Name == "" {
			t.Errorf("repo %d missing owner/name: %+v", i, r)
		}
		if r.URL == "" {
			t.Errorf("repo %d (%s) missing URL", i, r.ExternalID)
		}
	}

	// At least most rows should carry a parsed "stars this week" and total stars;
	// a broad zero would mean the selectors drifted.
	withPeriod, withStars := 0, 0
	for _, r := range repos {
		if r.StarsThisPeriod > 0 {
			withPeriod++
		}
		if r.Stars > 0 {
			withStars++
		}
	}
	if withPeriod < len(repos)/2 {
		t.Errorf("only %d/%d repos had starsThisPeriod parsed", withPeriod, len(repos))
	}
	if withStars < len(repos)/2 {
		t.Errorf("only %d/%d repos had total stars parsed", withStars, len(repos))
	}

	// Spot-check the first row against the known fixture content.
	first := repos[0]
	if first.ExternalID != "bojieli/ai-agent-book" {
		t.Errorf("first repo = %q, want bojieli/ai-agent-book", first.ExternalID)
	}
	if first.Language != "Python" {
		t.Errorf("first repo language = %q, want Python", first.Language)
	}
	if first.Stars != 19133 {
		t.Errorf("first repo stars = %d, want 19133", first.Stars)
	}
	if first.StarsThisPeriod != 17401 {
		t.Errorf("first repo starsThisPeriod = %d, want 17401", first.StarsThisPeriod)
	}
	if first.Description == "" {
		t.Errorf("first repo missing description")
	}
}
