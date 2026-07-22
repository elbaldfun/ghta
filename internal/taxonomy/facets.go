package taxonomy

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FacetType is one type-facet enum entry from facets.yaml. Order in the file is
// priority: the first entry whose topic signals match wins (Q1: awesome first).
type FacetType struct {
	Key    string   `yaml:"key"`
	Name   string   `yaml:"name"`
	Topics []string `yaml:"topics"`
}

// Facets is the parsed facets.yaml: the priority-ordered type list plus the
// fallback used when nothing matches (software).
type Facets struct {
	Type         []FacetType `yaml:"type"`
	Fallback     string      `yaml:"fallback"`
	FallbackName string      `yaml:"fallbackName"`

	// topicIndex maps a lowercased topic -> type key, built once at load.
	topicIndex map[string]string
}

// namingRules recover the high-confidence doc types from the repo name when the
// author left no matching topic (change 12 eval: naming lifts type coverage).
// Each rule maps a compiled name pattern to a type key; first match wins, and
// they are checked before the topic table because a name like "awesome-x" or
// "x-tutorial" is a stronger signal than an incidental topic.
var namingRules = []struct {
	re  *regexp.Regexp
	key string
}{
	{regexp.MustCompile(`(?i)^awesome($|-)|(-awesome$)`), "awesome"},
	{regexp.MustCompile(`(?i)free-programming|programming-books`), "awesome"},
	{regexp.MustCompile(`(?i)interview|leetcode`), "interview"},
	{regexp.MustCompile(`(?i)tutorial|-course$|roadmap|100-days|the-hard-way|for-beginners|cookbook|^hello-`), "tutorial"},
	{regexp.MustCompile(`(?i)-skill$|^skills?$|-skills$`), "skill"},
	{regexp.MustCompile(`(?i)-cli$|^cli-`), "cli"},
}

// LoadFacets parses facets.yaml and builds the topic index.
func LoadFacets(path string) (*Facets, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read facets: %w", err)
	}
	var f Facets
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse facets: %w", err)
	}
	if f.Fallback == "" {
		f.Fallback = "software"
	}
	f.topicIndex = make(map[string]string)
	for _, t := range f.Type {
		for _, topic := range t.Topics {
			f.topicIndex[strings.ToLower(strings.TrimSpace(topic))] = t.Key
		}
	}
	return &f, nil
}

// ClassifyType returns the single type-facet key for an item. Resolution order:
// (1) naming rules on the repo name (high-confidence doc forms), (2) the topic
// table in priority order, (3) the fallback. externalID is "owner/name"; only
// the trailing name segment is matched by naming rules.
//
// The software sub-forms (cli/app/library) are weakly signaled by topics — the
// LLM tier refines those; this returns the best cheap guess (often the fallback).
func (f *Facets) ClassifyType(externalID string, topics []string) string {
	name := externalID
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	for _, r := range namingRules {
		if r.re.MatchString(name) {
			return r.key
		}
	}

	// Topic table, in the file's priority order: the first type whose signals
	// the item carries wins.
	set := make(map[string]struct{}, len(topics))
	for _, t := range topics {
		set[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}
	for _, ft := range f.Type {
		for _, sig := range ft.Topics {
			if _, ok := set[strings.ToLower(sig)]; ok {
				return ft.Key
			}
		}
	}
	return f.Fallback
}
