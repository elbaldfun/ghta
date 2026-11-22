package judge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The shipped table loads against the shipped tree, and where the golden set
// has an answer for the same repo, the two agree: a correction is never a
// second opinion on what a person already decided.
func TestShippedOverridesAgreeWithGolden(t *testing.T) {
	overrides, err := LoadOverrides("../../taxonomy/overrides.yaml", tree(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) == 0 {
		t.Fatal("no overrides loaded")
	}
	raw, err := os.ReadFile("../../taxonomy/golden.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Items []struct {
			ID     string   `yaml:"id"`
			Domain []string `yaml:"domain"`
			Type   string   `yaml:"type"`
		} `yaml:"items"`
	}
	if err := yaml.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	// The golden set lists a few repos twice (Q29); an override must agree
	// with one of the entries.
	for _, o := range overrides {
		got := slices.Sorted(slices.Values(o.Domain()))
		var disagree []string
		agrees := false
		for _, gi := range g.Items {
			if !strings.EqualFold(gi.ID, o.ID) {
				continue
			}
			want := slices.Sorted(slices.Values(gi.Domain))
			if o.Type == gi.Type && slices.Equal(got, want) {
				agrees = true
			} else {
				disagree = append(disagree, gi.Type+" "+strings.Join(want, ","))
			}
		}
		if !agrees && len(disagree) > 0 {
			t.Errorf("%s: override says %s %v, golden says %v", o.ID, o.Type, got, disagree)
		}
	}
}

func TestLoadOverridesRejects(t *testing.T) {
	for name, entry := range map[string]string{
		"unknown path":        "{ id: a/b, type: cli, paths: [web/nope], shelf: excluded, why: x }",
		"apps leaf, no shelf": "{ id: a/b, type: app, paths: [apps/system/files], shelf: excluded, why: x }",
		"shelf leaf twice":    "{ id: a/b, type: cli, paths: [devtools/terminal], shelf: devtools/terminal, why: x }",
		"unknown shelf":       "{ id: a/b, type: app, paths: [], shelf: system/nope, why: x }",
		"no shelf":            "{ id: a/b, type: app, paths: [], why: x }",
		"library on a shelf":  "{ id: a/b, type: library, paths: [], shelf: system/files, why: x }",
		"unknown type":        "{ id: a/b, type: thing, paths: [], shelf: excluded, why: x }",
		"too many leaves":     "{ id: a/b, type: app, paths: [web/frontend, web/backend, web/build-tools], shelf: system/files, why: x }",
		"no reason":           "{ id: a/b, type: cli, paths: [], shelf: excluded }",
		"bad id":              "{ id: ab, type: cli, paths: [], shelf: excluded, why: x }",
		"app in client dev":   "{ id: a/b, type: app, paths: [client/android], shelf: system/phone, why: x }",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "o.yaml")
			if err := os.WriteFile(path, []byte("items:\n  - "+entry+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadOverrides(path, tree(t)); err == nil {
				t.Errorf("want an error for %s", entry)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "o.yaml")
	twice := "items:\n  - { id: a/b, type: cli, paths: [], shelf: excluded, why: x }\n  - { id: a/b, type: app, paths: [], shelf: excluded, why: y }\n"
	if err := os.WriteFile(path, []byte(twice), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOverrides(path, tree(t)); err == nil {
		t.Error("want an error for an id listed twice")
	}
}

func TestOverrideDomainPutsTheShelfFirst(t *testing.T) {
	o := Override{Type: "app", Paths: []string{"ai/media-gen"}, Shelf: "ai/generation"}
	if got := o.Domain(); !slices.Equal(got, []string{"apps/ai/generation", "ai/media-gen"}) {
		t.Errorf("domain = %v", got)
	}
	o = Override{Type: "tutorial", Shelf: "excluded"}
	if got := o.Domain(); len(got) != 0 {
		t.Errorf("domain = %v, want none", got)
	}
}

func TestDomainKeepsAppsOutOfClientDevelopment(t *testing.T) {
	for _, c := range []struct {
		v    Verdict
		want []string
	}{
		// An Android app on the phone shelf is not an Android library.
		{Verdict{Paths: []string{"client/android", "apps/system/phone"}, Shelf: "system/phone"}, []string{"apps/system/phone"}},
		{Verdict{Paths: []string{"client/desktop", "web/frontend"}, Shelf: "media/music"}, []string{"apps/media/music", "web/frontend"}},
		// A developer app keeps it (golden: termux), and so does a library.
		{Verdict{Paths: []string{"client/android"}, Shelf: "devtools/terminal"}, []string{"devtools/terminal", "client/android"}},
		{Verdict{Paths: []string{"client/android"}, Shelf: "excluded"}, []string{"client/android"}},
	} {
		if got := c.v.Domain(); !slices.Equal(got, c.want) {
			t.Errorf("%+v: domain = %v, want %v", c.v, got, c.want)
		}
	}
}
