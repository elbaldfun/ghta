package boards

import (
	"reflect"
	"testing"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func f(v float64) *float64 { return &v }

// repo builds a live GitHub row.
func repo(id, owner string, stars, d1 float64, domain []string, topics ...string) Row {
	r := Row{Source: item.SourceGitHub, ExternalID: id, Name: id, Topics: topics,
		Metrics: map[string]float64{"stars": stars}, Growth: item.Growth{D1: f(d1)}}
	r.Labels.Domain = domain
	r.Gh = &struct {
		Owner    string    `bson:"owner"`
		PushedAt time.Time `bson:"pushedAt"`
	}{Owner: owner}
	return r
}

func corpus(rows ...Row) *Corpus {
	var c Corpus
	for _, r := range rows {
		c.Add(r)
	}
	return &c
}

func TestCorpusSkipsStaleAndOtherSources(t *testing.T) {
	gone := repo("o/gone", "o", 1, 0, nil)
	gone.Stale = true
	model := repo("m/x", "m", 1, 0, nil)
	model.Source = item.SourceHuggingFace
	if n := corpus(repo("o/a", "o", 1, 0, nil), gone, model).Len(); n != 1 {
		t.Fatalf("len = %d; want only the live GitHub repo", n)
	}
}

func TestGroups(t *testing.T) {
	gs := Groups([]string{"agents", "ai", "web"})
	if gs[0].Key != "all" || gs[0].Tops != nil {
		t.Fatalf("first group = %+v; want all", gs[0])
	}
	ai := gs[2]
	if ai.Key != "ai" || !reflect.DeepEqual(ai.Tops, []string{"ai", "agents"}) {
		t.Fatalf("ai group = %+v; want ai with agents", ai)
	}
	r := repo("o/a", "o", 1, 0, []string{"agents/coding"})
	if !ai.has(&r) || gs[3].has(&r) {
		t.Fatal("agents/coding should be in ai, not in web")
	}
	// The prefix is anchored on the slash: "web" never matches "web3/…".
	r = repo("o/b", "o", 1, 0, []string{"web3/defi"})
	if gs[3].has(&r) {
		t.Fatal("web matched web3/defi")
	}
}

func TestDevelopers(t *testing.T) {
	c := corpus(
		repo("al/one", "al", 1500, 300, []string{"ai/llm-serving"}),
		repo("al/two", "al", 1000, 10, []string{"web/frontend"}),
		repo("bo/big", "bo", 100000, 50, []string{"ai/llm-serving"}),
		repo("org/x", "org", 900000, 9000, []string{"ai/llm-serving"}),
		repo("cy/new", "cy", 1900, 1900, []string{"ai/llm-serving"}),
	)
	profiles := map[string]Profile{
		"al": {Login: "al", Type: "User", Name: "Al"}, "bo": {Login: "bo", Type: "User"},
		"org": {Login: "org", Type: "Organization"}, "cy": {Login: "cy", Type: "User"},
	}
	rising, merit := c.Developers(Group{Key: "all"}, profiles)
	// Organisations never rank; cy is under the 2,000-star floor for rising.
	if got := logins(rising); !reflect.DeepEqual(got, []string{"al", "bo"}) {
		t.Fatalf("rising = %v; want al (310 today) then bo (50)", got)
	}
	al := rising[0]
	if al.RepoCount != 2 || al.TotalStars != 2500 || al.TopRepo != "al/one" || al.Growth != 310 || al.Name != "Al" {
		t.Fatalf("al = %+v", al)
	}
	// Merit is log-summed: al's two repos (3.18 + 3) beat bo's one 100k hit (5).
	if got := logins(merit); !reflect.DeepEqual(got, []string{"al", "bo", "cy"}) {
		t.Fatalf("merit = %v", got)
	}
	if merit[0].Score < 6.17 || merit[0].Score > 6.18 {
		t.Fatalf("al merit = %v; want log10(1500)+log10(1000)", merit[0].Score)
	}

	// Within ai, al counts only the ai repo.
	rising, _ = c.Developers(Group{Key: "ai", Tops: []string{"ai"}}, profiles)
	if rising[0].Login != "bo" || len(rising) != 1 {
		t.Fatalf("ai rising = %v; al's ai work alone is under the floor", logins(rising))
	}
}

func logins(rows []Developer) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Login)
	}
	return out
}

func TestTopics(t *testing.T) {
	var rows []Row
	for i, id := range []string{"a", "b", "c", "d", "e"} {
		rows = append(rows, repo("o/"+id, "o", float64(100*(i+1)), 1, nil, "go", "cli"))
	}
	rows = append(rows, repo("o/f", "o", 1, 50, nil, "go", "rare"))
	hot, popular := corpus(rows...).Topics(Group{Key: "all"})
	// "rare" is on one repo: under the floor.
	if len(hot) != 2 || hot[0].Topic != "go" || hot[0].Growth != 55 || hot[0].TopRepo != "o/e" {
		t.Fatalf("hot = %+v", hot)
	}
	if popular[0].Topic != "go" || popular[0].RepoCount != 6 || popular[1].Score != 5 {
		t.Fatalf("popular = %+v", popular)
	}
}

func TestEcosystem(t *testing.T) {
	skill := repo("o/skill", "o", 10, 5, nil)
	skill.Labels.Type = "skill"
	c := corpus(skill,
		repo("o/mcp", "o", 30, 1, nil, "mcp-server"),
		repo("o/agent", "o", 20, 9, []string{"agents/frameworks"}),
		// agents/mcp alone is the MCP pillar's by topic, not an agent.
		repo("o/tool", "o", 40, 0, []string{"agents/mcp"}),
		repo("o/other", "o", 50, 50, []string{"web/frontend"}),
	)
	hot, popular := c.Ecosystem(PillarAll)
	if got := ecoIDs(hot); !reflect.DeepEqual(got, []string{"o/agent", "o/skill", "o/mcp"}) {
		t.Fatalf("all hot = %v", got)
	}
	if got := ecoIDs(popular); !reflect.DeepEqual(got, []string{"o/mcp", "o/agent", "o/skill"}) {
		t.Fatalf("all popular = %v", got)
	}
	if hot, _ := c.Ecosystem(PillarAgent); len(hot) != 1 || !hot[0].IsAgent || hot[0].IsMcp {
		t.Fatalf("agent pillar = %+v", hot)
	}
}

func ecoIDs(rows []EcoItem) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ExternalID)
	}
	return out
}

var tree = taxonomy.Tree{Nodes: []taxonomy.Node{
	{Path: "ai", Name: "人工智能", NameEn: "AI", Children: []taxonomy.Node{
		{Path: "ai/llm-serving", Name: "推理", NameEn: "Serving"}, {Path: "ai/vision", Name: "视觉"},
	}},
	{Path: "apps", Name: "应用", Children: []taxonomy.Node{
		{Path: "apps/productivity", Name: "效率", Children: []taxonomy.Node{{Path: "apps/productivity/notes", Name: "笔记"}}},
	}},
}}

func TestHeat(t *testing.T) {
	cells := corpus(
		repo("o/a", "o", 100, 10, []string{"ai/llm-serving", "ai/vision"}),
		repo("o/b", "o", 100, 30, []string{"ai/vision"}),
		repo("o/c", "o", 100, 1, []string{"lost/leaf"}),
	).Heat(tree)
	if len(cells) != 2 || cells[0].Path != "ai" || cells[0].Name != "人工智能" || cells[0].NameEn != "AI" {
		t.Fatalf("tops = %+v", cells)
	}
	ai := cells[0]
	// o/a sits in two ai leaves and counts in both, as on the old map.
	if ai.Repos != 3 || ai.Growth != 50 || ai.Intensity != 16.67 {
		t.Fatalf("ai = %+v", ai)
	}
	if ai.Children[0].Path != "ai/vision" || ai.Children[0].Growth != 40 || ai.Children[1].Name != "推理" {
		t.Fatalf("ai leaves = %+v", ai.Children)
	}
	if cells[1].Name != "lost" {
		t.Fatalf("a domain outside the tree is named by its path: %+v", cells[1])
	}
}

func TestCategories(t *testing.T) {
	nodes := corpus(
		repo("o/a", "o", 1, 0, []string{"ai/llm-serving", "ai/vision"}),
		repo("o/b", "o", 1, 0, []string{"apps/productivity/notes", "ai/vision"}),
	).Categories(tree)
	ai, apps := nodes[0], nodes[1]
	// Each item once per node: o/a is in two ai leaves but ai counts it once.
	if ai.Count != 2 || ai.Children[0].Count != 1 || ai.Children[1].Count != 2 {
		t.Fatalf("ai = %+v", ai)
	}
	if apps.Count != 1 || apps.Children[0].Count != 1 || apps.Children[0].Children[0].Count != 1 {
		t.Fatalf("apps = %+v", apps)
	}
}

func TestTypeFacets(t *testing.T) {
	a, b := repo("o/a", "o", 1, 0, nil), repo("o/b", "o", 1, 0, nil)
	a.Labels.Type, b.Labels.Type = "cli", "cli"
	got := corpus(a, b).TypeFacets([]Facet{{Key: "app", Name: "应用"}, {Key: "cli", Name: "命令行"}})
	if !reflect.DeepEqual(got, []Facet{{Key: "app", Name: "应用"}, {Key: "cli", Name: "命令行", Count: 2}}) {
		t.Fatalf("facets = %+v", got)
	}
}

func TestLanguages(t *testing.T) {
	var rows []Row
	for i, s := range []float64{10, 20, 30, 40} {
		r := repo("o/go"+string(rune('a'+i)), "o", s, 0, nil)
		r.Language = "Go"
		rows = append(rows, r)
	}
	r := repo("o/rs", "o", 99, 0, nil)
	r.Language = "Rust"
	rows = append(rows, r, repo("o/none", "o", 1, 0, nil))
	got := corpus(rows...).Languages()
	want := []LanguageStat{
		{Language: "Go", Repos: 4, TotalStars: 100, MedianStars: 20, TopRepo: "o/god", TopStars: 40},
		{Language: "Rust", Repos: 1, TotalStars: 99, MedianStars: 99, TopRepo: "o/rs", TopStars: 99},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("languages = %+v", got)
	}
}

func TestStaleness(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	pushed := func(id string, stars, issues float64, daysAgo int) Row {
		r := repo(id, "o", stars, 0, nil)
		r.Metrics["openIssues"] = issues
		r.Gh.PushedAt = now.AddDate(0, 0, -daysAgo)
		return r
	}
	s := corpus(
		pushed("o/fresh", 1000, 10, 10),
		pushed("o/edge", 3000, 30, 90), // exactly 90 days: still active
		pushed("o/slow", 500, 5, 200),
		pushed("o/old", 800, 80, 1000),
		pushed("o/older", 900, 9, 2000),
		repo("o/unknown", "o", 1, 0, nil), // no push date: not counted
	).Staleness(now)
	if s.Total != 5 {
		t.Fatalf("total = %d", s.Total)
	}
	active := s.Buckets[0]
	if active.Bucket != "active" || active.Repos != 2 || active.MedianStars != 1000 || active.IssuesPerKStar != 10 {
		t.Fatalf("active = %+v", active)
	}
	if s.Buckets[2].Repos != 0 || s.Buckets[3].Bucket != "stale" || s.Buckets[3].Repos != 2 {
		t.Fatalf("buckets = %+v", s.Buckets)
	}
	if len(s.Examples) != 2 || s.Examples[0].ExternalID != "o/older" {
		t.Fatalf("examples = %+v; want the dormant repos, most stars first", s.Examples)
	}
}
