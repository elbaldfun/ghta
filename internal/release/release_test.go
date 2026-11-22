package release

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNextVersion(t *testing.T) {
	day := time.Date(2026, 11, 22, 23, 59, 0, 0, time.UTC)
	cases := []struct {
		name string
		tags []string
		want string
	}{
		{"first of the day", nil, "v2026.11.22.1"},
		{"old tags do not count", []string{"v1-final", "v2026.11.21.3"}, "v2026.11.22.1"},
		{"same day goes up", []string{"v2026.11.22.1", "v2026.11.22.2"}, "v2026.11.22.3"},
		{"a gap is skipped, not filled", []string{"v2026.11.22.1", "v2026.11.22.4"}, "v2026.11.22.5"},
		{"N counts as a number", []string{"v2026.11.22.9", "v2026.11.22.10"}, "v2026.11.22.11"},
		{"malformed tags are ignored", []string{"v2026.11.22.0", "v2026.11.22", "2026.11.22.7", "v2026.11.22.2-rc"}, "v2026.11.22.1"},
	}
	for _, c := range cases {
		if got := NextVersion(day, c.tags).String(); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	// The day is the UTC day: 07:30 in Beijing on the 23rd is still the 22nd.
	cst := time.FixedZone("CST", 8*3600)
	if got := NextVersion(time.Date(2026, 11, 23, 7, 30, 0, 0, cst), nil).String(); got != "v2026.11.22.1" {
		t.Errorf("UTC day: %s", got)
	}
}

func TestLatest(t *testing.T) {
	if _, ok := Latest([]string{"v1-final"}); ok {
		t.Error("v1-final is not a date version")
	}
	v, ok := Latest([]string{"v2026.11.22.2", "v2026.12.01.1", "v2026.11.22.10", "v1-final"})
	if !ok || v.String() != "v2026.12.01.1" {
		t.Errorf("latest = %v %v", v, ok)
	}
	v, _ = Latest([]string{"v2026.11.22.2", "v2026.11.22.10"})
	if v.String() != "v2026.11.22.10" {
		t.Errorf("same day: %v", v)
	}
}

// log builds `git log --merges --format=LogFormat` output.
func log(recs ...[2]string) string {
	var b strings.Builder
	for _, r := range recs {
		b.WriteString(r[0] + fieldSep + r[1] + recordSep + "\n")
	}
	return b.String()
}

func TestParseLog(t *testing.T) {
	out := log(
		[2]string{"Merge pull request #80 from elbaldfun/develop", "Release v2026.11.22.1\n"},
		[2]string{"Merge pull request #76 from elbaldfun/claude/github-repo-pages", "web: GitHub 榜单页迁到新设计\n"},
		[2]string{"Merge remote-tracking branch 'origin/develop' into claude/x", "# Conflicts:\n#\tweb/a.json\n"},
		[2]string{"Merge pull request #74 from elbaldfun/claude/growth-gap-fix", "\nfix: reconcile treats a change of case as a rename\n\nmore\n"},
		[2]string{"Merge pull request #61 from elbaldfun/main", "Back-merge hotfix\n"},
		[2]string{"Merge pull request #70 from elbaldfun/feat/hf-base", ""},
		[2]string{"Merge pull request #76 from elbaldfun/claude/github-repo-pages", "dup\n"},
	)
	want := []PR{
		{76, "claude/github-repo-pages", "web: GitHub 榜单页迁到新设计"},
		{74, "claude/growth-gap-fix", "fix: reconcile treats a change of case as a rename"},
		{70, "feat/hf-base", "feat/hf-base"}, // no title in the body: the branch stands in
	}
	if got := ParseLog(out); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseLog:\n got %+v\nwant %+v", got, want)
	}
	if got := ParseLog(""); len(got) != 0 {
		t.Errorf("empty log: %+v", got)
	}
}

func TestSection(t *testing.T) {
	for title, want := range map[string]string{
		"feat: rank change arrows":          Added,
		"feat(web): rank change arrows":     Added,
		"Feat!: breaking":                   Added,
		"fix: growth of a case twin":        Fixed,
		"fix(fetch): retry EOF":             Fixed,
		"hotfix: api 500":                   Fixed,
		"chore: bump deps":                  Improved,
		"web: move the header to tokens":    Improved, // a scope, not a type
		"reconcile: treat a change of case": Improved,
		"货架页与领域页加首选结论和排名规则":                 Improved,
		"fixes the thing":                   Improved, // no colon: not a prefix
	} {
		if got := Section(title); got != want {
			t.Errorf("Section(%q) = %s, want %s", title, got, want)
		}
	}
}

func TestNotes(t *testing.T) {
	prs := []PR{
		{76, "a", "web: GitHub 榜单页迁到新设计"},
		{74, "b", "fix(reconcile): treat a change of case as a rename"},
		{72, "c", "feat: rank change arrows"},
		{71, "d", "fix: eco growth d1"},
	}
	got := Notes("v2026.11.22.1", []string{"api", "worker", "web"}, prs)
	want := `StarRank v2026.11.22.1：发布 api、worker、web。

## 新增

- rank change arrows (#72)

## 修复

- eco growth d1 (#71)
- reconcile: treat a change of case as a rename (#74)

## 改进

- web: GitHub 榜单页迁到新设计 (#76)
`
	if got != want {
		t.Errorf("Notes:\n%s\nwant:\n%s", got, want)
	}
	if got := Notes("v2026.11.22.2", []string{"web"}, nil); !strings.Contains(got, "发布 web") || !strings.Contains(got, "没有新合并的 PR") {
		t.Errorf("empty notes: %q", got)
	}
}

func TestPromoteBody(t *testing.T) {
	got := PromoteBody("v2026.11.22.1", []PR{{74, "b", "fix: case twin"}, {76, "a", "web: pages"}})
	for _, s := range []string{"`v2026.11.22.1`", "merge commit", "`hold`", "## 修复\n\n- fix: case twin #74", "## 改进\n\n- web: pages #76"} {
		if !strings.Contains(got, s) {
			t.Errorf("promote body lacks %q:\n%s", s, got)
		}
	}
	if strings.Contains(got, "## 新增") {
		t.Errorf("empty section printed:\n%s", got)
	}
}

func TestInPipelineWindow(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 22, h, m, 0, 0, time.UTC) }
	for _, c := range []struct {
		t    time.Time
		want bool
	}{
		{at(3, 29), false}, {at(3, 30), true}, {at(6, 0), true}, {at(8, 29), true}, {at(8, 30), false}, {at(23, 0), false},
		{time.Date(2026, 11, 22, 12, 0, 0, 0, time.FixedZone("CST", 8*3600)), true}, // 04:00 UTC
	} {
		if got := InPipelineWindow(c.t); got != c.want {
			t.Errorf("InPipelineWindow(%v) = %v", c.t, got)
		}
	}
}
