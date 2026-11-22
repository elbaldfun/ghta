package boards

import (
	"fmt"
	"testing"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

var verdictNow = time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)

func ago(days int) time.Time { return verdictNow.Add(-time.Duration(days) * 24 * time.Hour) }

// live is a domain row with a last commit and a 30-day gain.
func live(id string, stars, d30 float64, pushed int, domain ...string) Row {
	r := repo(id, "o", stars, 0, domain)
	r.Growth.D30 = f(d30)
	r.Gh.PushedAt = ago(pushed)
	return r
}

func verdictTree(paths ...string) taxonomy.Tree {
	var t taxonomy.Tree
	for _, p := range paths {
		t.Nodes = append(t.Nodes, taxonomy.Node{Path: p})
	}
	return t
}

func TestDomainVerdictSkipsStaleAndNamesRiser(t *testing.T) {
	old := live("a/old", 9000, 10, 400, "web/frontend")
	pick := live("b/pick", 8000, 50, 3, "web/frontend")
	pick.Gh.LatestRelease = &item.ReleaseRef{PublishedAt: ago(12)}
	pick.Ranks = &item.Ranks{Domain: map[string]int{"web/frontend": 2}, Prev: &item.RankPositions{Domain: map[string]int{"web/frontend": 3}}}
	riser := live("c/riser", 5000, 900, 1, "web/frontend")
	staleRiser := live("d/stale-riser", 4000, 5000, 200, "web/frontend")
	c := corpus(old, pick, riser, staleRiser)

	got := c.DomainVerdicts(verdictTree("web", "web/frontend"), verdictNow)["web/frontend"]
	if len(got) != 1 {
		t.Fatalf("want one verdict, got %d", len(got))
	}
	v := got[0]
	if v.Pick.ExternalID != "b/pick" || v.Pick.Position != 2 || v.Pick.PushedDaysAgo != 3 || v.Pick.ReleaseDaysAgo != 12 {
		t.Errorf("pick = %+v", v.Pick)
	}
	if v.Pick.RankNow != 2 || v.Pick.RankPrev != 3 {
		t.Errorf("rank = %d (was %d), want 2 (was 3)", v.Pick.RankNow, v.Pick.RankPrev)
	}
	if len(v.Skipped) != 1 || v.Skipped[0].ExternalID != "a/old" || v.Skipped[0].Reason != SkipStale || v.Skipped[0].PushedDaysAgo != 400 {
		t.Errorf("skipped = %+v", v.Skipped)
	}
	// The stale project grew fastest but is not maintained: not a riser.
	if v.Riser == nil || v.Riser.ExternalID != "c/riser" {
		t.Errorf("riser = %+v, want c/riser", v.Riser)
	}
	if v.Pool != 4 || v.Basis != VerdictBasisStars || v.Scope != VerdictDomain || v.FreshDays != VerdictFreshDays {
		t.Errorf("verdict = %+v", v)
	}
}

func TestDomainVerdictTopLevelCountsEachRowOnce(t *testing.T) {
	// One repo in two leaves of a domain is one candidate of the domain.
	c := corpus(
		live("a/both", 9000, 1, 1, "web/frontend", "web/backend"),
		live("b/other", 8000, 1, 1, "web/backend"),
	)
	v := c.DomainVerdicts(verdictTree("web", "web/frontend", "web/backend"), verdictNow)
	if top := v["web"]; len(top) != 1 || top[0].Pool != 2 || top[0].Pick.ExternalID != "a/both" {
		t.Errorf("web = %+v", top)
	}
	if top := v["web"][0]; top.Pick.RankNow != 0 {
		t.Errorf("a top-level domain has no stored ranking, got %d", top.Pick.RankNow)
	}
}

func TestDomainVerdictPoolAndEmpty(t *testing.T) {
	var rows []Row
	for i := 0; i < VerdictPool+5; i++ {
		// All stale but the one past the pool.
		pushed := 365
		if i == VerdictPool {
			pushed = 1
		}
		rows = append(rows, live(fmt.Sprintf("o/r%02d", i), float64(1000-i), 0, pushed, "data/database"))
	}
	v := corpus(rows...).DomainVerdicts(verdictTree("data", "data/database", "data/pipeline"), verdictNow)
	// Nothing in the first ten is maintained: no pick, rather than reach down.
	if got := v["data/database"]; len(got) != 0 {
		t.Errorf("want no pick past the pool, got %+v", got)
	}
	// A node with nothing in it still gets a board, so yesterday's pick goes.
	if got, ok := v["data/pipeline"]; !ok || got == nil || len(got) != 0 {
		t.Errorf("empty node = %v (present %v), want an empty board", got, ok)
	}
}

func TestDomainVerdictUnknownPushIsStale(t *testing.T) {
	r := live("a/x", 10, 0, 0, "lang/utils")
	r.Gh.PushedAt = time.Time{}
	v := corpus(r).DomainVerdicts(verdictTree("lang/utils"), verdictNow)["lang/utils"]
	if len(v) != 0 {
		t.Errorf("an unknown last commit must not qualify, got %+v", v)
	}
}

func TestShelfVerdictNeedsInstaller(t *testing.T) {
	gain := func(n int) *int { return &n }
	rel := ago(20)
	best := []App{
		{ExternalID: "a/cli-only", Name: "cli-only", Stars: 9000, pushedAt: ago(2), d30: gain(10)},
		{ExternalID: "b/app", Name: "app", Stars: 8000, pushedAt: ago(5), d30: gain(40), HasDownloads: true,
			LatestReleaseAt: &rel, Downloads: []Download{{OS: "macos"}, {OS: "windows"}},
			ranks: &item.Ranks{Domain: map[string]int{"apps/productivity/notes": 4}}},
		{ExternalID: "c/fast", Name: "fast", Stars: 100, pushedAt: ago(1), d30: gain(300), HasDownloads: true},
		{ExternalID: "d/fast-no-build", Name: "fast-no-build", Stars: 90, pushedAt: ago(1), d30: gain(900)},
	}
	got := ShelfVerdict(best, "productivity/notes", verdictNow)
	if len(got) != 1 {
		t.Fatalf("want one verdict, got %d", len(got))
	}
	v := got[0]
	if v.Pick.ExternalID != "b/app" || v.Pick.ReleaseDaysAgo != 20 || len(v.Pick.Downloads) != 2 || v.Pick.RankNow != 4 {
		t.Errorf("pick = %+v", v.Pick)
	}
	if len(v.Skipped) != 1 || v.Skipped[0].Reason != SkipNoBuild {
		t.Errorf("skipped = %+v", v.Skipped)
	}
	if v.Riser == nil || v.Riser.ExternalID != "c/fast" {
		t.Errorf("riser = %+v, want c/fast (the faster one has nothing to install)", v.Riser)
	}
	if v.Basis != VerdictBasisBest || v.Pool != 4 {
		t.Errorf("verdict = %+v", v)
	}
}

func TestShelfLeaf(t *testing.T) {
	if got := ShelfLeaf("productivity/notes"); got != "apps/productivity/notes" {
		t.Errorf("apps shelf leaf = %q", got)
	}
	if got := ShelfLeaf("devtools/terminal"); got != "devtools/terminal" {
		t.Errorf("dev shelf leaf = %q", got)
	}
}

func TestDomainVerdictSkipsReadingMatter(t *testing.T) {
	list := live("a/awesome-hacking", 9000, 0, 1, "security/offensive")
	list.Labels.Type = "awesome"
	tool := live("b/tool", 8000, 0, 1, "security/offensive")
	v := corpus(list, tool).DomainVerdicts(verdictTree("security/offensive"), verdictNow)["security/offensive"]
	if len(v) != 1 || v[0].Pick.ExternalID != "b/tool" || len(v[0].Skipped) != 1 || v[0].Skipped[0].Reason != SkipReading {
		t.Errorf("verdict = %+v; an awesome list is not a pick", v)
	}
}

func TestShelfVerdictGoesByStarsNotPageOrder(t *testing.T) {
	// The best-of score put a day's spike first; the pick is still the
	// most-starred qualifying app, and keeps its place on the page.
	gain := func(n int) *int { return &n }
	best := []App{
		{ExternalID: "a/spike", Name: "spike", Stars: 13000, pushedAt: ago(0), d30: gain(3600), HasDownloads: true},
		{ExternalID: "b/big", Name: "big", Stars: 200000, pushedAt: ago(0), d30: gain(9300), HasDownloads: true},
	}
	v := ShelfVerdict(best, "network/download", verdictNow)
	if len(v) != 1 || v[0].Pick.ExternalID != "b/big" || v[0].Pick.Position != 2 || len(v[0].Skipped) != 0 || v[0].Riser != nil {
		t.Errorf("verdict = %+v", v)
	}
}
