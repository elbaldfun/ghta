// Package release holds the parts of scripts/release.sh that are worth testing:
// the date version a release gets, the pull requests a range of main carries,
// and the release notes made from them. The shell script does the git, gh and
// ssh work; cmd/release exposes these functions to it.
//
// See docs/BRANCHING_AND_RELEASE.md for the rules they implement.
package release

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// versionRe matches a date version tag: vYYYY.MM.DD.N, N from 1.
var versionRe = regexp.MustCompile(`^v(\d{4})\.(\d{2})\.(\d{2})\.([1-9]\d*)$`)

// Version is a parsed date version.
type Version struct {
	Day time.Time // UTC midnight
	N   int
}

func (v Version) String() string { return fmt.Sprintf("v%s.%d", v.Day.Format("2006.01.02"), v.N) }

// ParseVersion reads a date version tag. Other tags (v1-final, typos) are
// not versions: ok is false.
func ParseVersion(tag string) (v Version, ok bool) {
	m := versionRe.FindStringSubmatch(strings.TrimSpace(tag))
	if m == nil {
		return Version{}, false
	}
	day, err := time.Parse("2006.01.02", m[1]+"."+m[2]+"."+m[3])
	if err != nil {
		return Version{}, false
	}
	n, _ := strconv.Atoi(m[4])
	return Version{Day: day, N: n}, true
}

// NextVersion is the version the next release on day (taken in UTC) gets:
// one past the highest N already tagged that day, or N=1. Tags of other days
// and tags that are not date versions do not count. A tag is never reused,
// so a gap (a deleted tag) is skipped rather than filled.
func NextVersion(day time.Time, tags []string) Version {
	d := day.UTC()
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	next := Version{Day: d, N: 1}
	for _, t := range tags {
		if v, ok := ParseVersion(t); ok && v.Day.Equal(d) && v.N >= next.N {
			next.N = v.N + 1
		}
	}
	return next
}

// Latest is the highest date version among tags, by day then N.
func Latest(tags []string) (Version, bool) {
	var best Version
	found := false
	for _, t := range tags {
		v, ok := ParseVersion(t)
		if !ok {
			continue
		}
		if !found || v.Day.After(best.Day) || v.Day.Equal(best.Day) && v.N > best.N {
			best, found = v, true
		}
	}
	return best, found
}

// PR is a pull request merged into the range, as its merge commit records it.
type PR struct {
	Number int
	Branch string // head branch, without the owner
	Title  string
}

// Field and record separators of LogFormat.
const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

// LogFormat is the git log --format the output of ParseLog expects:
// subject and body of each merge commit.
const LogFormat = "%s%x1f%b%x1e"

var mergeRe = regexp.MustCompile(`^Merge pull request #(\d+) from [^/\s]+/(\S+)$`)

// ParseLog picks the pull requests out of `git log --merges --format=LogFormat`.
// GitHub's merge commit says "Merge pull request #N from owner/branch" and
// carries the PR title as the first line of its body. Merges that are not PR
// merges (develop merged into a work branch) are skipped, and so are release
// PRs (from develop) and old main → develop back-merges: they carry other PRs,
// not changes of their own. Newest first, as git log gives them; a PR seen
// twice is listed once.
func ParseLog(out string) []PR {
	var prs []PR
	seen := map[int]bool{}
	for _, rec := range strings.Split(out, recordSep) {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		subject, body, _ := strings.Cut(rec, fieldSep)
		m := mergeRe.FindStringSubmatch(strings.TrimSpace(subject))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		branch := m[2]
		if branch == "develop" || branch == "main" || seen[n] {
			continue
		}
		seen[n] = true
		title := ""
		for _, line := range strings.Split(body, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				title = line
				break
			}
		}
		if title == "" {
			title = branch
		}
		prs = append(prs, PR{Number: n, Branch: branch, Title: title})
	}
	return prs
}

// Section names of the release notes, in the order they are printed.
const (
	Added    = "新增"
	Fixed    = "修复"
	Improved = "改进"
)

var sections = []string{Added, Fixed, Improved}

var typeRe = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?!?:\s*`)

// Section is where a PR goes in the release notes, from the conventional
// type prefix of its title: feat → 新增, fix → 修复. Everything else —
// other types, scope-only prefixes like "web:", plain sentences — is 改进.
func Section(title string) string {
	m := typeRe.FindStringSubmatch(title)
	if m == nil {
		return Improved
	}
	switch strings.ToLower(m[1]) {
	case "feat", "feature":
		return Added
	case "fix", "hotfix":
		return Fixed
	}
	return Improved
}

// cleanTitle drops a feat/fix type prefix, which the section heading already
// says; any other prefix stays, as it names what was changed.
func cleanTitle(title string) string {
	if m := typeRe.FindStringSubmatch(title); m != nil && Section(title) != Improved {
		rest := strings.TrimSpace(title[len(m[0]):])
		if m[2] != "" {
			rest = strings.Trim(m[2], "()") + ": " + rest
		}
		if rest != "" {
			return rest
		}
	}
	return title
}

// Group sorts PRs into the release-notes sections, each in ascending PR order.
func Group(prs []PR) map[string][]PR {
	out := map[string][]PR{}
	for _, p := range prs {
		s := Section(p.Title)
		out[s] = append(out[s], p)
	}
	for _, s := range out {
		sort.Slice(s, func(i, j int) bool { return s[i].Number < s[j].Number })
	}
	return out
}

// Notes renders the release notes: a first line saying what was released,
// then 新增 / 修复 / 改进 with the PR number at the end of each line. Empty
// sections are left out.
func Notes(version string, services []string, prs []PR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "StarRank %s：发布 %s。\n", version, strings.Join(services, "、"))
	if len(prs) == 0 {
		b.WriteString("\n与上一个版本相比没有新合并的 PR。\n")
		return b.String()
	}
	g := Group(prs)
	for _, s := range sections {
		if len(g[s]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n", s)
		for _, p := range g[s] {
			fmt.Fprintf(&b, "- %s (#%d)\n", cleanTitle(p.Title), p.Number)
		}
	}
	return b.String()
}

// PromoteBody is the body of the develop → main release PR.
func PromoteBody(version string, prs []PR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "发版 PR：把 `develop` 合进 `main`。预计版本 `%s`，实际版本号在发版成功、打标签时确定。\n", version)
	b.WriteString("\n合并方式：merge commit（不要 squash）。不想现在发就加 `hold` 标签。\n")
	if len(prs) == 0 {
		b.WriteString("\n本次没有新合并的 PR。\n")
		return b.String()
	}
	g := Group(prs)
	for _, s := range sections {
		if len(g[s]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n", s)
		for _, p := range g[s] {
			fmt.Fprintf(&b, "- %s #%d\n", p.Title, p.Number)
		}
	}
	return b.String()
}

// The worker's nightly pipeline starts at 03:30 UTC and is done by 08:30.
// Restarting the worker inside that window kills the run: the day's
// remaining shards, the HuggingFace boards and the metrics are lost.
const (
	windowStart = 3*60 + 30
	windowEnd   = 8*60 + 30
)

// InPipelineWindow reports whether t falls in the worker's nightly window,
// UTC 03:30 (inclusive) to 08:30 (exclusive).
func InPipelineWindow(t time.Time) bool {
	u := t.UTC()
	m := u.Hour()*60 + u.Minute()
	return m >= windowStart && m < windowEnd
}
