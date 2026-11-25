package taxonomy

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// StoreGenres maps an app store listing to a shelf (taxonomy/store-genres.yaml):
// its primary genre, then keywords in its name or the head of its
// description. No model is involved; an unmapped listing gets no shelf.
type StoreGenres struct {
	genres map[string]storeGenre
}

type storeGenre struct {
	rules []storeRule
	shelf string // when no rule matches; "" for none
}

type storeRule struct {
	match *regexp.Regexp
	shelf string
	desc  bool // also read the head of the description
}

// StoreDescriptionHead is how much of a description the keyword rules read:
// the opening says what the app is, the rest is feature lists and legalese.
const StoreDescriptionHead = 300

type storeGenresFile struct {
	Genres []struct {
		ID    string `yaml:"id"`
		Name  string `yaml:"name"`
		Shelf string `yaml:"shelf"`
		Why   string `yaml:"why"`
		Rules []struct {
			Match string `yaml:"match"`
			Shelf string `yaml:"shelf"`
			Desc  bool   `yaml:"description"`
		} `yaml:"rules"`
	} `yaml:"genres"`
}

// LoadStoreGenres reads and checks the genre table against a tree: every
// target must be a shelf of the tree, and a genre that maps to no shelf must
// say why. A bad table stops the worker at start.
func LoadStoreGenres(path string, tree Tree) (*StoreGenres, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read store genres: %w", err)
	}
	var f storeGenresFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse store genres: %w", err)
	}
	shelves := map[string]bool{}
	for _, l := range tree.Leaves() {
		if IsShelfPath(l.Path) {
			shelves[ShelfSlug(l.Path)] = true
		}
	}
	check := func(id, slug string) error {
		if slug != "" && !shelves[slug] {
			return fmt.Errorf("store genres: genre %s: %q is not a shelf of the tree", id, slug)
		}
		return nil
	}
	sg := &StoreGenres{genres: map[string]storeGenre{}}
	for _, g := range f.Genres {
		if g.ID == "" {
			return nil, fmt.Errorf("store genres: a genre has no id")
		}
		if _, dup := sg.genres[g.ID]; dup {
			return nil, fmt.Errorf("store genres: genre %s listed twice", g.ID)
		}
		if err := check(g.ID, g.Shelf); err != nil {
			return nil, err
		}
		if g.Shelf == "" && strings.TrimSpace(g.Why) == "" {
			return nil, fmt.Errorf("store genres: genre %s maps to no shelf by default and says no why", g.ID)
		}
		out := storeGenre{shelf: g.Shelf}
		for i, r := range g.Rules {
			if r.Shelf == "" || r.Match == "" {
				return nil, fmt.Errorf("store genres: genre %s rule %d needs match and shelf", g.ID, i+1)
			}
			if err := check(g.ID, r.Shelf); err != nil {
				return nil, err
			}
			re, err := regexp.Compile("(?i)" + r.Match)
			if err != nil {
				return nil, fmt.Errorf("store genres: genre %s rule %d: %w", g.ID, i+1, err)
			}
			out.rules = append(out.rules, storeRule{match: re, shelf: r.Shelf, desc: r.Desc})
		}
		sg.genres[g.ID] = out
	}
	return sg, nil
}

// Has reports whether the table lists a genre.
func (sg *StoreGenres) Has(genreID string) bool {
	_, ok := sg.genres[genreID]
	return ok
}

// Shelf is the shelf slug for a listing, or "" for none. The rules are tried
// in order against the name; then the rules marked description are tried
// against the head of the description. A name is the stronger signal, so a
// rule further down that matches the name beats one that only matches the
// description.
func (sg *StoreGenres) Shelf(genreID, name, description string) string {
	g, ok := sg.genres[genreID]
	if !ok {
		return ""
	}
	for _, r := range g.rules {
		if r.match.MatchString(name) {
			return r.shelf
		}
	}
	head := headRunes(description, StoreDescriptionHead)
	for _, r := range g.rules {
		if r.desc && r.match.MatchString(head) {
			return r.shelf
		}
	}
	return g.shelf
}

func headRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for j := range s {
		if i == n {
			return s[:j]
		}
		i++
	}
	return s
}
