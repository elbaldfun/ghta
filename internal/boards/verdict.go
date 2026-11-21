package boards

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// The verdict is the one-line pick at the top of a shelf's best-of page and
// a domain board: which project to start with, and the evidence for it.
//
// The rule is deliberately plain, so the page can state it in a sentence and
// a reader can check it against the list below: of the projects the page
// shows first, take the most-starred one that is a project rather than
// reading matter (an awesome list, a tutorial, interview prep), is still
// maintained (a commit within VerdictFreshDays) and, on a shelf, has an
// installer to download. Stars and not the best-of score decide, so a single
// day's spike cannot make the pick. The evidence is raw signals only, never
// a score: stars, the 30-day star gain, the last commit, the latest release,
// the installers, and the move in the domain's star ranking. Projects with
// more stars are named with the reason they were passed over, and the
// fastest grower among the qualifying candidates is named next to the pick
// when that is someone else.
const (
	VerdictDomain = "domain"
	VerdictShelf  = "shelf"

	// Basis is the order of the page the pick's position refers to: a domain
	// board by stars, or a best-of collection (BestOf).
	VerdictBasisStars = "stars"
	VerdictBasisBest  = "best"

	// VerdictPool is how far down a domain board the rule looks; a shelf's
	// pool is its whole best-of collection (BestOfN).
	VerdictPool = 10
	// VerdictFreshDays is how recent the last commit must be.
	VerdictFreshDays = 90

	SkipReading = "reading"  // a list, tutorial or interview prep, not a project
	SkipStale   = "stale"    // no commit within VerdictFreshDays, or unknown
	SkipNoBuild = "no-build" // a shelf app with nothing to download
)

// readingTypes are the forms that are reading matter, not something to use.
// Skills stay in: on the agents/skills board a skill is the thing itself.
var readingTypes = []string{"awesome", "tutorial", "interview"}

func DomainVerdictKey(path string) string { return "verdict|domain|" + path }

func ShelfVerdictKey(shelf string) string { return "verdict|shelf|" + shelf }

// Verdict is the pick of one page. A board holds zero or one: zero when no
// candidate qualifies, which the page shows as no pick at all.
type Verdict struct {
	Scope string `bson:"scope" json:"scope"`
	Key   string `bson:"key" json:"key"`
	Basis string `bson:"basis" json:"basis"`
	// Pool is how many candidates the rule looked at.
	Pool int `bson:"pool" json:"pool"`
	// FreshDays is the commit window the pick had to meet, so the page states
	// the rule that was applied.
	FreshDays int      `bson:"freshDays" json:"freshDays"`
	Pick      Evidence `bson:"pick" json:"pick"`
	// Skipped are the candidates with more stars than the pick, most first,
	// and why they were passed over.
	Skipped []Skip `bson:"skipped" json:"skipped"`
	// Riser is the maintained candidate with the largest 30-day gain, when
	// that is not the pick and it gained more.
	Riser *Evidence `bson:"riser,omitempty" json:"riser,omitempty"`
}

// Evidence is what the sentence about one project cites. Days are whole days
// before the board was computed; -1 means unknown.
type Evidence struct {
	ExternalID string `bson:"externalId" json:"externalId"`
	Name       string `bson:"name" json:"name"`
	// Position is 1-based, in the page's order.
	Position int `bson:"position" json:"position"`
	Stars    int `bson:"stars" json:"stars"`
	// D30 is the 30-day star gain; nil for a project younger than the window.
	D30           *int `bson:"d30,omitempty" json:"d30,omitempty"`
	PushedDaysAgo int  `bson:"pushedDaysAgo" json:"pushedDaysAgo"`
	// ReleaseDaysAgo is the latest release's age; -1 for none known.
	ReleaseDaysAgo int `bson:"releaseDaysAgo" json:"releaseDaysAgo"`
	// Downloads are the platforms with an installer (shelves only).
	Downloads []string `bson:"downloads,omitempty" json:"downloads,omitempty"`
	// RankNow and RankPrev are the star ranking in the page's domain leaf, now
	// and at the previous ranking day; 0 when unknown.
	RankNow  int `bson:"rankNow,omitempty" json:"rankNow,omitempty"`
	RankPrev int `bson:"rankPrev,omitempty" json:"rankPrev,omitempty"`
}

// Skip is a candidate above the pick that was passed over.
type Skip struct {
	ExternalID    string `bson:"externalId" json:"externalId"`
	Name          string `bson:"name" json:"name"`
	Position      int    `bson:"position" json:"position"`
	Reason        string `bson:"reason" json:"reason"`
	PushedDaysAgo int    `bson:"pushedDaysAgo" json:"pushedDaysAgo"`
}

// candidate is one row of a pool, whatever board it came from.
type candidate struct {
	ev       Evidence
	reading  bool
	hasBuild bool
}

// daysAgo is the whole days from t to now; -1 for a zero t.
func daysAgo(t time.Time, now time.Time) int {
	if t.IsZero() {
		return -1
	}
	d := int(now.Sub(t).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

func intPtr(v *float64) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

// decide applies the rule to a pool; it walks the pool by stars, whatever
// order the page shows it in.
func decide(scope, key, basis string, pool []candidate, needBuild bool) []Verdict {
	v := Verdict{Scope: scope, Key: key, Basis: basis, Pool: len(pool), FreshDays: VerdictFreshDays, Skipped: []Skip{}}
	pool = slices.Clone(pool)
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].ev.Stars > pool[j].ev.Stars })
	why := func(c candidate) string {
		switch {
		case c.reading:
			return SkipReading
		case c.ev.PushedDaysAgo < 0 || c.ev.PushedDaysAgo > VerdictFreshDays:
			return SkipStale
		case needBuild && !c.hasBuild:
			return SkipNoBuild
		}
		return ""
	}
	pick := -1
	for i, c := range pool {
		reason := why(c)
		if reason == "" {
			pick = i
			break
		}
		v.Skipped = append(v.Skipped, Skip{
			ExternalID: c.ev.ExternalID, Name: c.ev.Name, Position: c.ev.Position,
			Reason: reason, PushedDaysAgo: c.ev.PushedDaysAgo,
		})
	}
	if pick < 0 {
		return []Verdict{}
	}
	v.Pick = pool[pick].ev
	gain := func(e Evidence) int {
		if e.D30 == nil {
			return 0
		}
		return *e.D30
	}
	for i, c := range pool {
		if i == pick || why(c) != "" {
			continue
		}
		if g := gain(c.ev); g > gain(v.Pick) && (v.Riser == nil || g > gain(*v.Riser)) {
			ev := c.ev
			v.Riser = &ev
		}
	}
	return []Verdict{v}
}

// ---- domains -------------------------------------------------------------------

// DomainVerdicts computes the verdict of every node of the tree, over the
// rows its board lists by default: a path with a slash matches the leaf
// exactly, a top-level domain matches every leaf under it (store.ListQuery),
// most stars first.
func (c *Corpus) DomainVerdicts(tree taxonomy.Tree, now time.Time) map[string][]Verdict {
	pools := map[string][]*Row{}
	for _, n := range tree.Nodes {
		pools[n.Path] = nil
	}
	better := func(a, b *Row) bool {
		if a.stars() != b.stars() {
			return a.stars() > b.stars()
		}
		return a.ExternalID < b.ExternalID
	}
	offer := func(path string, r *Row) {
		p, ok := pools[path]
		if !ok {
			return
		}
		if len(p) == VerdictPool && !better(r, p[len(p)-1]) {
			return
		}
		i := sort.Search(len(p), func(i int) bool { return better(r, p[i]) })
		p = append(p, nil)
		copy(p[i+1:], p[i:])
		p[i] = r
		if len(p) > VerdictPool {
			p = p[:VerdictPool]
		}
		pools[path] = p
	}
	for i := range c.rows {
		r := &c.rows[i]
		var tops []string
		for _, p := range r.Labels.Domain {
			offer(p, r)
			t, _, ok := strings.Cut(p, "/")
			if ok && !slices.Contains(tops, t) {
				tops = append(tops, t)
				offer(t, r)
			}
		}
	}
	out := make(map[string][]Verdict, len(pools))
	for path, rows := range pools {
		pool := make([]candidate, len(rows))
		for i, r := range rows {
			ev := Evidence{
				ExternalID: r.ExternalID, Name: r.Name, Position: i + 1, Stars: int(r.stars()),
				D30: intPtr(r.Growth.D30), PushedDaysAgo: -1, ReleaseDaysAgo: -1,
			}
			if r.Gh != nil {
				ev.PushedDaysAgo = daysAgo(r.Gh.PushedAt, now)
				if r.Gh.LatestRelease != nil {
					ev.ReleaseDaysAgo = daysAgo(r.Gh.LatestRelease.PublishedAt, now)
				}
			}
			ev.RankNow, ev.RankPrev = r.leafRank(path)
			pool[i] = candidate{ev: ev, reading: slices.Contains(readingTypes, r.Labels.Type)}
		}
		out[path] = decide(VerdictDomain, path, VerdictBasisStars, pool, false)
	}
	return out
}

// leafRank is the row's star ranking in a leaf, now and at the previous
// ranking day. A top-level domain has no stored ranking.
func (r *Row) leafRank(path string) (now, prev int) {
	if r.Ranks == nil {
		return 0, 0
	}
	now = r.Ranks.Domain[path]
	if r.Ranks.Prev != nil {
		prev = r.Ranks.Prev.Domain[path]
	}
	return now, prev
}

// ---- shelves -------------------------------------------------------------------

// ShelfVerdict computes a shelf's verdict over its best-of collection; a
// position is the app's place in that collection. A shelf app must also have
// an installer.
func ShelfVerdict(best []App, shelf string, now time.Time) []Verdict {
	leaf := ShelfLeaf(shelf)
	pool := make([]candidate, len(best))
	for i, a := range best {
		ev := Evidence{
			ExternalID: a.ExternalID, Name: a.Name, Position: i + 1, Stars: a.Stars,
			D30: a.d30, PushedDaysAgo: daysAgo(a.pushedAt, now), ReleaseDaysAgo: -1,
		}
		if a.LatestReleaseAt != nil {
			ev.ReleaseDaysAgo = daysAgo(*a.LatestReleaseAt, now)
		}
		for _, d := range a.Downloads {
			ev.Downloads = append(ev.Downloads, d.OS)
		}
		if a.ranks != nil {
			ev.RankNow = a.ranks.Domain[leaf]
			if a.ranks.Prev != nil {
				ev.RankPrev = a.ranks.Prev.Domain[leaf]
			}
		}
		pool[i] = candidate{ev: ev, reading: slices.Contains(readingTypes, a.Type), hasBuild: a.HasDownloads}
	}
	return decide(VerdictShelf, shelf, VerdictBasisBest, pool, true)
}

// ShelfLeaf is the domain leaf a shelf slug files its apps under: the apps
// branch's leaf, or the developer shelf's own path.
func ShelfLeaf(shelf string) string {
	if ShelfGroup(shelf) == DevGroup {
		return shelf
	}
	return taxonomy.ShelfPath(shelf)
}
