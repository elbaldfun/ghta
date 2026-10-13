// Package judge is the one LLM pass of the rebuilt system (docs/REBUILD.md M3):
// a single call decides an item's domain, form, store shelf, taglines,
// alternatives and tags, replacing the three separate passes of the old one.
//
// Every item is asked more than once, in different batches, and the answers
// are compared (Decide): agreement is the confidence signal a single answer
// cannot give.
package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// PromptVersion is the generation of the prompt and its enums. Bump it to send
// the whole corpus back through the judge.
const PromptVersion = 1

// Types is the form facet, in the order the prompt explains it.
var Types = []string{"library", "app", "cli", "software", "tutorial", "awesome", "interview", "skill"}

// DevShelves are the developer-facing apps: plain domain leaves that the store
// shows under one "dev" view. Every other shelf is a leaf of the apps branch.
var DevShelves = []string{
	"devtools/terminal", "devtools/editors", "devtools/api-tools", "devtools/vcs", "devtools/package-manager", "devtools/code-quality",
	"data/database", "infra/containers", "infra/automation", "infra/monitoring",
	"gamedev/engines", "security/offensive", "agents/coding",
}

const (
	maxDomain       = 3
	maxAlternatives = 3
	maxTags         = 6
	taglineZhRunes  = 24
	taglineEnRunes  = 90
)

// Input is what the model sees of one item.
type Input struct {
	ID          string
	Name        string
	Description string
	Language    string
	Topics      []string
	Platforms   []string
	// Readme is an excerpt (Excerpt), not the document.
	Readme string
}

// Verdict is one validated answer for one item.
type Verdict struct {
	// Paths holds the domain leaves the model named, most relevant first.
	// Leaves of the apps branch are never here: Shelf carries that answer.
	Paths []string
	Type  string
	// Shelf is a shelf slug or item.ShelfExcluded.
	Shelf        string
	HasGui       bool
	TaglineZh    string
	TaglineEn    string
	Alternatives []item.Alternative
	Tags         []string
}

// Primary is the domain leaf the votes are compared on, "" for none.
func (v Verdict) Primary() string {
	if len(v.Paths) == 0 {
		return ""
	}
	return v.Paths[0]
}

// Domain is what the item is filed under: the leaf of its shelf, when it is
// on one, then the domain leaves. The shelf and the domain are two answers
// stored as one fact, so the shelf's leaf is always among them.
func (v Verdict) Domain() []string {
	out := []string{}
	if v.Shelf != item.ShelfExcluded && v.Shelf != "" {
		out = append(out, ShelfLeaf(v.Shelf))
	}
	for _, p := range v.Paths {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	if len(out) > maxDomain {
		out = out[:maxDomain]
	}
	return out
}

// Completer is the model call; *Client implements it.
type Completer interface {
	Complete(ctx context.Context, system, user string, schema json.RawMessage, maxTokens int) (string, error)
}

// Judge asks the model about batches of items.
type Judge struct {
	llm     Completer
	head    string
	leaves  []string // domain leaves: every leaf outside the apps branch
	shelves []string
	isLeaf  map[string]bool
	isShelf map[string]bool
}

// New builds a judge for one taxonomy tree.
func New(llm Completer, tree taxonomy.Tree) *Judge {
	j := &Judge{llm: llm, isLeaf: map[string]bool{}, isShelf: map[string]bool{item.ShelfExcluded: true}}
	var nodes, shelves []taxonomy.Node
	for _, n := range tree.Leaves() {
		if taxonomy.IsShelfPath(n.Path) {
			j.shelves = append(j.shelves, taxonomy.ShelfSlug(n.Path))
			shelves = append(shelves, n)
			continue
		}
		nodes = append(nodes, n)
		j.leaves = append(j.leaves, n.Path)
		j.isLeaf[n.Path] = true
	}
	for _, s := range DevShelves {
		if j.isLeaf[s] {
			j.shelves = append(j.shelves, s)
		}
	}
	for _, s := range j.shelves {
		j.isShelf[s] = true
	}
	j.shelves = append(j.shelves, item.ShelfExcluded)
	j.head = promptHead(nodes, shelves)
	return j
}

// Ask judges one batch. full asks for everything; a verification pass sets it
// false and gets only what the votes are compared on, at a fraction of the
// output. Items the model's answer is unusable for are absent from the result;
// an error means the whole call failed and says nothing about any item.
func (j *Judge) Ask(ctx context.Context, batch []Input, full bool) (map[string]Verdict, error) {
	if len(batch) == 0 {
		return map[string]Verdict{}, nil
	}
	ids := make([]string, len(batch))
	for i, in := range batch {
		ids[i] = in.ID
	}
	perItem := 90
	if full {
		perItem = 260
	}
	raw, err := j.llm.Complete(ctx, systemPrompt, j.head+fields(full)+lines(batch), j.schema(ids, full), 200+perItem*len(batch))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []answer `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse verdicts: %w", err)
	}
	out := make(map[string]Verdict, len(batch))
	for _, a := range parsed.Results {
		if !slices.Contains(ids, a.ID) {
			continue
		}
		if v, ok := j.verdict(a); ok {
			out[a.ID] = v
		}
	}
	return out, nil
}

// answer is one element of the model's reply.
type answer struct {
	ID           string   `json:"id"`
	Paths        []string `json:"paths"`
	Type         string   `json:"type"`
	Shelf        string   `json:"shelf"`
	HasGui       bool     `json:"hasGui"`
	TaglineZh    string   `json:"taglineZh"`
	TaglineEn    string   `json:"taglineEn"`
	Alternatives []struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	} `json:"alternatives"`
	Tags []string `json:"tags"`
}

// verdict validates one answer. The schema already rules most of this out;
// the checks are for a server that does not enforce it.
func (j *Judge) verdict(a answer) (Verdict, bool) {
	if !slices.Contains(Types, a.Type) || !j.isShelf[a.Shelf] {
		return Verdict{}, false
	}
	v := Verdict{
		Type: a.Type, Shelf: a.Shelf, HasGui: a.HasGui,
		TaglineZh: clip(strings.TrimSpace(a.TaglineZh), taglineZhRunes),
		TaglineEn: clip(strings.TrimSpace(a.TaglineEn), taglineEnRunes),
	}
	for _, p := range a.Paths {
		if j.isLeaf[p] && !slices.Contains(v.Paths, p) {
			v.Paths = append(v.Paths, p)
		}
	}
	seen := map[string]bool{}
	for _, alt := range a.Alternatives {
		name := strings.TrimSpace(alt.Name)
		slug := Slug(name)
		if slug == "" || seen[slug] || len(v.Alternatives) == maxAlternatives {
			continue
		}
		seen[slug] = true
		v.Alternatives = append(v.Alternatives, item.Alternative{Name: name, Slug: slug, Kind: strings.TrimSpace(alt.Kind)})
	}
	for _, t := range a.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !slices.Contains(v.Tags, t) && len(v.Tags) < maxTags {
			v.Tags = append(v.Tags, t)
		}
	}
	return v, true
}

// ShelfLeaf is the taxonomy leaf of a shelf slug: dev shelves are leaves
// themselves, consumer shelves live under apps/.
func ShelfLeaf(slug string) string {
	if slices.Contains(DevShelves, slug) {
		return slug
	}
	return taxonomy.ShelfPath(slug)
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

// Slug makes a URL-safe key of a product name: "Adobe Photoshop" -> "adobe-photoshop".
func Slug(name string) string {
	return strings.Trim(slugStrip.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
}
