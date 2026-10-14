package boards

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// ---- ecosystem -----------------------------------------------------------------

// The agent ecosystem has three pillars: skills (by form), MCP (by topic) and
// agents (by domain). Each board is sorted "hot" by the day's star gain or
// "popular" by stars.
const (
	PillarAll   = "all"
	PillarSkill = "skill"
	PillarMcp   = "mcp"
	PillarAgent = "agent"

	EcoHot     = "hot"
	EcoPopular = "popular"

	EcoTopN = 300
)

var Pillars = []string{PillarAll, PillarSkill, PillarMcp, PillarAgent}

var mcpTopics = []string{"mcp", "mcp-server", "model-context-protocol"}

// agentLeaf reports a v3 agents leaf that is about agents themselves. Skills
// and MCP have their own pillars, judged by form and topic as before.
func agentLeaf(p string) bool {
	return strings.HasPrefix(p, "agents/") && p != "agents/skills" && p != "agents/mcp"
}

type EcoItem struct {
	ExternalID  string `bson:"externalId" json:"externalId"`
	Name        string `bson:"name" json:"name"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	Language    string `bson:"language,omitempty" json:"language,omitempty"`
	Stars       int    `bson:"stars" json:"stars"`
	Forks       int    `bson:"forks" json:"forks"`
	Growth      int    `bson:"growth" json:"growth"`
	Type        string `bson:"type,omitempty" json:"type,omitempty"`
	IsSkill     bool   `bson:"isSkill" json:"isSkill"`
	IsMcp       bool   `bson:"isMcp" json:"isMcp"`
	IsAgent     bool   `bson:"isAgent" json:"isAgent"`
}

// Ecosystem returns the hot and popular boards of one pillar.
func (c *Corpus) Ecosystem(pillar string) (hot, popular []EcoItem) {
	var rows []EcoItem
	for i := range c.rows {
		r := &c.rows[i]
		e := EcoItem{
			ExternalID: r.ExternalID, Name: r.Name, Description: r.Description, Language: r.Language,
			Stars: int(r.stars()), Forks: int(r.Metrics["forks"]), Growth: int(r.d1()), Type: r.Labels.Type,
			IsSkill: r.Labels.Type == "skill",
			IsMcp:   slices.ContainsFunc(r.Topics, func(t string) bool { return slices.Contains(mcpTopics, t) }),
			IsAgent: slices.ContainsFunc(r.Labels.Domain, agentLeaf),
		}
		in := e.IsSkill || e.IsMcp || e.IsAgent
		switch pillar {
		case PillarSkill:
			in = e.IsSkill
		case PillarMcp:
			in = e.IsMcp
		case PillarAgent:
			in = e.IsAgent
		}
		if in {
			rows = append(rows, e)
		}
	}
	hot = headEco(slices.Clone(rows), func(e EcoItem) int { return e.Growth })
	popular = headEco(rows, func(e EcoItem) int { return e.Stars })
	return hot, popular
}

func headEco(rows []EcoItem, key func(EcoItem) int) []EcoItem {
	sort.Slice(rows, func(i, j int) bool {
		if a, b := key(rows[i]), key(rows[j]); a != b {
			return a > b
		}
		return rows[i].ExternalID < rows[j].ExternalID
	})
	if len(rows) > EcoTopN {
		rows = rows[:EcoTopN]
	}
	if rows == nil {
		rows = []EcoItem{}
	}
	return rows
}

// ---- heat map ------------------------------------------------------------------

// HeatCell is one domain or leaf of the heat map. Intensity, the day's gain
// per repo, is the heat; the old map summed a domain from its leaves, so an
// item in two leaves of one domain counts in both, and so does this one.
type HeatCell struct {
	Path      string     `bson:"path" json:"path"`
	Name      string     `bson:"name" json:"name"`
	NameEn    string     `bson:"nameEn,omitempty" json:"nameEn,omitempty"`
	Repos     int        `bson:"repos" json:"repos"`
	Stars     int        `bson:"stars" json:"stars"`
	Growth    int        `bson:"growth" json:"growth"`
	Intensity float64    `bson:"intensity" json:"intensity"`
	Children  []HeatCell `bson:"children,omitempty" json:"children,omitempty"`
}

func (c *Corpus) Heat(tree taxonomy.Tree) []HeatCell {
	names := nodeNames(tree)
	leaves := map[string]*HeatCell{}
	for i := range c.rows {
		r := &c.rows[i]
		for _, p := range r.Labels.Domain {
			if !strings.Contains(p, "/") {
				continue
			}
			l := leaves[p]
			if l == nil {
				l = &HeatCell{Path: p}
				leaves[p] = l
			}
			l.Repos++
			l.Stars += int(r.stars())
			l.Growth += int(r.d1())
		}
	}
	tops := map[string]*HeatCell{}
	for p, l := range leaves {
		t, _, _ := strings.Cut(p, "/")
		top := tops[t]
		if top == nil {
			top = &HeatCell{Path: t}
			tops[t] = top
		}
		top.Repos += l.Repos
		top.Stars += l.Stars
		top.Growth += l.Growth
		l.Intensity = perRepo(l.Growth, l.Repos)
		names.label(l)
		top.Children = append(top.Children, *l)
	}
	out := make([]HeatCell, 0, len(tops))
	for _, t := range tops {
		t.Intensity = perRepo(t.Growth, t.Repos)
		names.label(t)
		sortHeat(t.Children)
		out = append(out, *t)
	}
	sortHeat(out)
	return out
}

func sortHeat(cells []HeatCell) {
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].Growth != cells[j].Growth {
			return cells[i].Growth > cells[j].Growth
		}
		return cells[i].Path < cells[j].Path
	})
}

func perRepo(growth, repos int) float64 {
	if repos == 0 {
		return 0
	}
	return float64(int(float64(growth)/float64(repos)*100+0.5)) / 100
}

type nameMap map[string][2]string

func nodeNames(tree taxonomy.Tree) nameMap {
	m := nameMap{}
	for _, n := range tree.Nodes {
		m[n.Path] = [2]string{n.Name, n.NameEn}
	}
	return m
}

// label names a cell after its tree node, or after its path when the tree has
// no such node.
func (m nameMap) label(c *HeatCell) {
	n, ok := m[c.Path]
	if !ok || n[0] == "" {
		c.Name = c.Path
		return
	}
	c.Name, c.NameEn = n[0], n[1]
}

// ---- tree counts and facets ----------------------------------------------------

// CategoryNode is a node of the domain tree with the number of live items
// filed anywhere under it, each item counted once per node.
type CategoryNode struct {
	Path     string         `bson:"path" json:"path"`
	Name     string         `bson:"name" json:"name"`
	NameEn   string         `bson:"nameEn,omitempty" json:"nameEn,omitempty"`
	Count    int            `bson:"count" json:"count"`
	Children []CategoryNode `bson:"children,omitempty" json:"children,omitempty"`
}

func (c *Corpus) Categories(tree taxonomy.Tree) []CategoryNode {
	counts := map[string]int{}
	for i := range c.rows {
		seen := map[string]bool{}
		for _, p := range c.rows[i].Labels.Domain {
			// The leaf and every node above it.
			for {
				if !seen[p] {
					seen[p] = true
					counts[p]++
				}
				j := strings.LastIndexByte(p, '/')
				if j < 0 {
					break
				}
				p = p[:j]
			}
		}
	}
	// The tree comes flat, parents first; nest each node under its parent.
	children := map[string][]taxonomy.Node{}
	for _, n := range tree.Nodes {
		parent := ""
		if j := strings.LastIndexByte(n.Path, '/'); j > 0 {
			parent = n.Path[:j]
		}
		children[parent] = append(children[parent], n)
	}
	var build func(parent string) []CategoryNode
	build = func(parent string) []CategoryNode {
		nodes := children[parent]
		out := make([]CategoryNode, 0, len(nodes))
		for _, n := range nodes {
			out = append(out, CategoryNode{Path: n.Path, Name: n.Name, NameEn: n.NameEn,
				Count: counts[n.Path], Children: build(n.Path)})
		}
		return out
	}
	return build("")
}

// Tops are the tree's top-level domains, in tree order.
func Tops(tree taxonomy.Tree) []string {
	var out []string
	for _, n := range tree.Nodes {
		if !strings.Contains(n.Path, "/") {
			out = append(out, n.Path)
		}
	}
	return out
}

// Facet is one form with its live item count, for the filter chips.
type Facet struct {
	Key   string `bson:"key" json:"key"`
	Name  string `bson:"name" json:"name"`
	Count int    `bson:"count" json:"count"`
}

// TypeFacets counts the live items of each form, in the order given.
func (c *Corpus) TypeFacets(order []Facet) []Facet {
	counts := map[string]int{}
	for i := range c.rows {
		counts[c.rows[i].Labels.Type]++
	}
	out := make([]Facet, len(order))
	for i, f := range order {
		f.Count = counts[f.Key]
		out[i] = f
	}
	return out
}

// ---- corpus statistics -----------------------------------------------------------

// LanguageTopN is how many languages the language board keeps, most repos
// first.
const LanguageTopN = 100

type LanguageStat struct {
	Language    string  `bson:"language" json:"language"`
	Repos       int     `bson:"repos" json:"repos"`
	TotalStars  int64   `bson:"totalStars" json:"totalStars"`
	MedianStars float64 `bson:"medianStars" json:"medianStars"`
	TopRepo     string  `bson:"topRepo" json:"topRepo"`
	TopStars    int64   `bson:"topStars" json:"topStars"`
}

// Languages is the per-language totals. The median, not the mean, because a
// few giant repos would otherwise speak for the whole language.
func (c *Corpus) Languages() []LanguageStat {
	type acc struct {
		stars []float64
		total float64
		top   top
	}
	by := map[string]*acc{}
	for i := range c.rows {
		r := &c.rows[i]
		if r.Language == "" {
			continue
		}
		a := by[r.Language]
		if a == nil {
			a = &acc{}
			by[r.Language] = a
		}
		a.stars = append(a.stars, r.stars())
		a.total += r.stars()
		a.top.offer(r)
	}
	out := make([]LanguageStat, 0, len(by))
	for lang, a := range by {
		out = append(out, LanguageStat{Language: lang, Repos: len(a.stars), TotalStars: int64(a.total),
			MedianStars: median(a.stars), TopRepo: a.top.ID, TopStars: int64(a.top.Stars)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repos != out[j].Repos {
			return out[i].Repos > out[j].Repos
		}
		return out[i].Language < out[j].Language
	})
	if len(out) > LanguageTopN {
		out = out[:LanguageTopN]
	}
	return out
}

// Staleness buckets repos by how long since their last push. The median open
// issues rides along: "not pushed lately" alone cannot tell a finished
// project from an abandoned one, a growing backlog can.
type Staleness struct {
	Total    int               `bson:"total" json:"total"`
	Buckets  []StalenessBucket `bson:"buckets" json:"buckets"`
	Examples []StaleRepo       `bson:"examples" json:"examples"`
}

type StalenessBucket struct {
	Bucket         string  `bson:"bucket" json:"bucket"`
	Repos          int     `bson:"repos" json:"repos"`
	MedianStars    float64 `bson:"medianStars" json:"medianStars"`
	MedianIssues   float64 `bson:"medianIssues" json:"medianIssues"`
	IssuesPerKStar float64 `bson:"issuesPerKStar" json:"issuesPerKStar"`
}

type StaleRepo struct {
	ExternalID string    `bson:"externalId" json:"externalId"`
	Language   string    `bson:"language" json:"language"`
	Stars      int64     `bson:"stars" json:"stars"`
	OpenIssues int64     `bson:"openIssues" json:"openIssues"`
	PushedAt   time.Time `bson:"pushedAt" json:"pushedAt"`
}

// StaleExamples is how many long-dormant repos the staleness board names.
const StaleExamples = 50

// stalenessCutoffs: a repo lands in the first bucket its last push is newer
// than; "stale" takes the rest.
var stalenessCutoffs = []struct {
	id   string
	days int
}{{"active", 90}, {"slowing", 365}, {"dormant", 730}, {"stale", 0}}

func (c *Corpus) Staleness(now time.Time) Staleness {
	stars := make([][]float64, len(stalenessCutoffs))
	issues := make([][]float64, len(stalenessCutoffs))
	var dormant []StaleRepo
	longAgo := now.AddDate(0, 0, -730)
	for i := range c.rows {
		r := &c.rows[i]
		if r.Gh == nil || r.Gh.PushedAt.IsZero() {
			continue
		}
		b := len(stalenessCutoffs) - 1
		for k, cut := range stalenessCutoffs[:b] {
			if !r.Gh.PushedAt.Before(now.AddDate(0, 0, -cut.days)) {
				b = k
				break
			}
		}
		stars[b] = append(stars[b], r.stars())
		issues[b] = append(issues[b], r.Metrics["openIssues"])
		if r.Gh.PushedAt.Before(longAgo) {
			dormant = append(dormant, StaleRepo{ExternalID: r.ExternalID, Language: r.Language,
				Stars: int64(r.stars()), OpenIssues: int64(r.Metrics["openIssues"]), PushedAt: r.Gh.PushedAt.UTC()})
		}
	}
	var s Staleness
	for k, cut := range stalenessCutoffs {
		b := StalenessBucket{Bucket: cut.id, Repos: len(stars[k]),
			MedianStars: median(stars[k]), MedianIssues: median(issues[k])}
		if b.MedianStars > 0 {
			b.IssuesPerKStar = b.MedianIssues / b.MedianStars * 1000
		}
		s.Total += b.Repos
		s.Buckets = append(s.Buckets, b)
	}
	sort.Slice(dormant, func(i, j int) bool {
		if dormant[i].Stars != dormant[j].Stars {
			return dormant[i].Stars > dormant[j].Stars
		}
		return dormant[i].ExternalID < dormant[j].ExternalID
	})
	if len(dormant) > StaleExamples {
		dormant = dormant[:StaleExamples]
	}
	s.Examples = dormant
	if s.Examples == nil {
		s.Examples = []StaleRepo{}
	}
	return s
}
