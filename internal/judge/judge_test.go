package judge

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func tree(t *testing.T) taxonomy.Tree {
	t.Helper()
	tr, err := taxonomy.LoadTree("../../taxonomy/taxonomy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// fake answers with a canned reply and keeps what it was asked.
type fake struct {
	reply  string
	err    error
	user   string
	schema json.RawMessage
}

func (f *fake) Complete(_ context.Context, _, user string, schema json.RawMessage, _ int) (string, error) {
	f.user, f.schema = user, schema
	return f.reply, f.err
}

func TestDevShelvesAreLeaves(t *testing.T) {
	leaves := map[string]bool{}
	for _, n := range tree(t).Leaves() {
		leaves[n.Path] = true
	}
	for _, s := range DevShelves {
		if !leaves[s] {
			t.Errorf("dev shelf %q is not a leaf of the tree", s)
		}
	}
}

func TestAskValidatesAndReconciles(t *testing.T) {
	f := &fake{reply: `{"results":[
	 {"id":"a/notes","paths":["apps/productivity/notes","data/database","apps/productivity/todo"],"type":"app","shelf":"productivity/notes",
	  "hasGui":true,"taglineZh":"本地优先的笔记应用","taglineEn":"Local-first notes","alternatives":[{"name":"Notion","kind":"note-taking"},{"name":" notion ","kind":"x"},{"name":"","kind":""}],"tags":["Notes","notes","markdown"]},
	 {"id":"b/term","paths":["lang/utils"],"type":"cli","shelf":"devtools/terminal","hasGui":false,"taglineZh":"","taglineEn":"","alternatives":[],"tags":[]},
	 {"id":"c/lib","paths":["apps/productivity/notes","web/frontend","no/such-leaf"],"type":"library","shelf":"excluded","hasGui":false,"taglineZh":"","taglineEn":"","alternatives":[],"tags":[]},
	 {"id":"d/bad","paths":[],"type":"gadget","shelf":"excluded"},
	 {"id":"e/bad","paths":[],"type":"app","shelf":"no/such-shelf"},
	 {"id":"z/unasked","paths":[],"type":"app","shelf":"excluded"}
	]}`}
	j := New(f, tree(t))
	batch := []Input{{ID: "a/notes", Topics: []string{"notes"}, Readme: "Write notes."}, {ID: "b/term"}, {ID: "c/lib"}, {ID: "d/bad"}, {ID: "e/bad"}}
	got, err := j.Ask(context.Background(), batch, true)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Verdict{
		// Leaves of the apps branch are not domain answers: the shelf says that.
		"a/notes": {Paths: []string{"data/database"}, Type: "app", Shelf: "productivity/notes", HasGui: true,
			TaglineZh: "本地优先的笔记应用", TaglineEn: "Local-first notes",
			Alternatives: []item.Alternative{{Name: "Notion", Slug: "notion", Kind: "note-taking"}}, Tags: []string{"notes", "markdown"}},
		"b/term": {Paths: []string{"lang/utils"}, Type: "cli", Shelf: "devtools/terminal"},
		// Unknown paths are ignored.
		"c/lib": {Paths: []string{"web/frontend"}, Type: "library", Shelf: item.ShelfExcluded},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("verdicts\n got %+v\nwant %+v", got, want)
	}
	// What the item is filed under: the shelf's leaf, then the domain leaves.
	for id, domain := range map[string][]string{
		"a/notes": {"apps/productivity/notes", "data/database"},
		"b/term":  {"devtools/terminal", "lang/utils"}, // a dev shelf is a domain leaf itself
		"c/lib":   {"web/frontend"},
	} {
		if d := got[id].Domain(); !reflect.DeepEqual(d, domain) {
			t.Errorf("%s filed under %v, want %v", id, d, domain)
		}
	}
	if !strings.Contains(f.user, `readme="Write notes."`) || !strings.Contains(f.user, "taglineZh") {
		t.Errorf("full prompt lacks the README or the prose fields")
	}
}

func TestSchemaKeepsOrderAndEnums(t *testing.T) {
	f := &fake{reply: `{"results":[]}`}
	j := New(f, tree(t))
	if _, err := j.Ask(context.Background(), []Input{{ID: "a/b"}, {ID: "c/d"}}, true); err != nil {
		t.Fatal(err)
	}
	full := string(f.schema)
	order := []string{`"id"`, `"paths"`, `"type"`, `"shelf"`, `"hasGui"`, `"taglineZh"`, `"taglineEn"`, `"alternatives"`, `"tags"`}
	at := strings.Index(full, `"properties":{"id"`)
	if at < 0 {
		t.Fatalf("item properties do not start with id: %s", full)
	}
	for _, key := range order {
		next := strings.Index(full[at:], key+":")
		if next < 0 {
			t.Fatalf("schema property %s missing or out of order", key)
		}
		at += next
	}
	for _, want := range []string{`"enum":["a/b","c/d"]`, `"minItems":2,"maxItems":2`, `"web/frontend"`, `"productivity/notes"`, `"devtools/terminal"`, `"excluded"`} {
		if !strings.Contains(full, want) {
			t.Errorf("schema lacks %s", want)
		}
	}
	if strings.Contains(full, `"apps/`) {
		t.Error("schema offers apps leaves as domain paths")
	}
	if !json.Valid(f.schema) {
		t.Error("schema is not valid JSON")
	}

	if _, err := j.Ask(context.Background(), []Input{{ID: "a/b"}}, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(f.schema), "tagline") || strings.Contains(f.user, "taglineZh") {
		t.Error("a verification pass asks for prose")
	}
}

func TestAskFailureIsNotAVerdict(t *testing.T) {
	j := New(&fake{err: errors.New("down")}, tree(t))
	if got, err := j.Ask(context.Background(), []Input{{ID: "a/b"}}, true); err == nil || got != nil {
		t.Errorf("failed call returned %v, %v", got, err)
	}
	j = New(&fake{reply: "not json"}, tree(t))
	if _, err := j.Ask(context.Background(), []Input{{ID: "a/b"}}, true); err == nil {
		t.Error("unparseable reply was accepted")
	}
}

func TestExcerpt(t *testing.T) {
	readme := "# Title\n\n[![badge](x)](y)\n<p align=center>logo</p>\n| a | b |\nA fast tool.\n\nIt does things."
	if got := Excerpt(readme, 300); got != "A fast tool. It does things." {
		t.Errorf("excerpt %q", got)
	}
	if got := Excerpt("你好世界再见", 4); got != "你好世界" {
		t.Errorf("rune cut %q", got)
	}
}

func v(typ, shelf string, domain ...string) Verdict {
	return Verdict{Paths: domain, Type: typ, Shelf: shelf}
}

func TestDecide(t *testing.T) {
	ex := item.ShelfExcluded
	full := v("library", ex, "web/frontend", "lang/typescript")
	full.TaglineEn, full.Tags = "A UI library", []string{"ui"}

	cases := []struct {
		name   string
		votes  []Verdict
		more   bool
		conf   string
		domain []string
		typ    string
		shelf  string
	}{
		{"one answer is not enough", []Verdict{full}, true, "", nil, "", ""},
		{"two agree", []Verdict{full, v("library", ex, "web/frontend")}, false, Agreed,
			// Only the leaf both named survives.
			[]string{"web/frontend"}, "library", ex},
		{"two disagree on the domain", []Verdict{full, v("library", ex, "web/backend")}, true, "", nil, "", ""},
		{"two disagree on the form", []Verdict{full, v("software", ex, "web/frontend")}, true, "", nil, "", ""},
		{"third settles it", []Verdict{full, v("software", ex, "web/backend"), v("library", ex, "web/frontend", "lang/typescript")}, false, Majority,
			[]string{"web/frontend", "lang/typescript"}, "library", ex},
		{"three ways", []Verdict{full, v("library", ex, "web/backend"), v("library", ex, "web/build-tools")}, false, Split,
			// No majority on the domain: the first answer's primary stands.
			[]string{"web/frontend"}, "library", ex},
		{"shelf outvotes the first answer",
			[]Verdict{v("app", "productivity/notes"), v("app", ex, "data/database"), v("app", ex, "data/database")},
			false, Majority, []string{"data/database"}, "app", ex},
		{"both have no domain", []Verdict{v("awesome", ex), v("awesome", ex)}, false, Agreed, nil, "awesome", ex},
	}
	for _, c := range cases {
		d, more := Decide(c.votes)
		if more != c.more {
			t.Errorf("%s: more = %v", c.name, more)
			continue
		}
		if more {
			continue
		}
		if d.Confidence != c.conf || !reflect.DeepEqual(d.Verdict.Paths, c.domain) || d.Verdict.Type != c.typ || d.Verdict.Shelf != c.shelf {
			t.Errorf("%s: got %s %v %s %s", c.name, d.Confidence, d.Verdict.Paths, d.Verdict.Type, d.Verdict.Shelf)
		}
		if d.Votes != len(c.votes) {
			t.Errorf("%s: votes %d", c.name, d.Votes)
		}
	}
	// The prose is the first answer's, whatever the votes say.
	if d, _ := Decide([]Verdict{full, v("library", ex, "web/frontend")}); d.Verdict.TaglineEn != "A UI library" || len(d.Verdict.Tags) != 1 {
		t.Errorf("prose lost: %+v", d.Verdict)
	}
}
