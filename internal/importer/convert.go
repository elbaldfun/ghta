// Package importer turns the old store (tracked_items) into the rebuilt one
// (items + item_blobs). It is the only place that knows the legacy shapes:
// string categoryPath, the untyped sourceData map, job flags scattered across
// the top level. See docs/REBUILD.md §4.4 for the field mapping.
package importer

import (
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
)

// Rules is what Convert needs to know about the taxonomy.
type Rules struct {
	// ValidPath reports whether p is a leaf of the loaded tree.
	ValidPath func(p string) bool
	// ShelfPath maps a store shelf slug to its taxonomy leaf path.
	ShelfPath func(slug string) string
	// BaseVersion is the tree generation assumed for items classified before
	// versions were stamped.
	BaseVersion int
}

// Notes records what Convert had to drop or repair for one item, so the import
// report can show it instead of losing it silently.
type Notes struct {
	DroppedPaths []string // categoryPath entries that are not leaves of the tree
	UnknownKeys  []string // sourceData keys with no destination
}

// sourceData keys that are deliberately not carried over: url is derivable
// from the external id, topics duplicates topicNames with a URL per topic,
// the last two are leftovers of the pre-Go importer.
var ignoredKeys = map[string]bool{
	"url": true, "topics": true, "organization": true, "migratedFrom": true,
}

// Convert maps one old document to the new item and, when it has cold text,
// its blob. It is pure: no I/O, no clock.
func Convert(old domain.TrackedItem, r Rules) (item.Item, *item.Blob, Notes) {
	var notes Notes
	sd := old.SourceData
	used := map[string]bool{}
	take := func(k string) any { used[k] = true; return sd[k] }

	it := item.Item{
		ID:              old.ID,
		Source:          string(old.Source),
		ExternalID:      old.ExternalID,
		Name:            old.Name,
		Description:     old.Description,
		Language:        old.Language,
		License:         str(take("license")),
		Topics:          []string{},
		PrimaryMetric:   old.PrimaryMetric,
		MetricDirection: string(old.MetricDirection),
		Metrics:         old.Metrics,
		Growth:          item.Growth{D1: old.DailyIncrease, D7: old.WeeklyIncrease, D30: old.MonthlyIncrease},
		History:         []item.Point{},
		Assets:          item.Assets{IconURL: old.IconURL, ScreenshotURL: old.ScreenshotURL, ShotSource: old.ShotSource},
		Stale:           old.Stale,
		StaleReason:     old.StaleReason,
		StaleAt:         old.StaleAt,
		FetchedAt:       old.FetchedAt,
		CreatedAt:       old.CreatedAt,
		UpdatedAt:       old.UpdatedAt,
	}
	if it.Metrics == nil {
		it.Metrics = map[string]float64{}
	}

	var blob *item.Blob
	switch old.Source {
	case domain.SourceGitHub:
		it.Homepage = str(take("homepageUrl"))
		it.Topics = strs(take("topicNames"))
		it.Gh = &item.GitHub{
			Owner:          str(take("owner")),
			PushedAt:       timeOf(take("pushedAt")),
			Platforms:      nilIfEmpty(strs(take("platforms"))),
			PlatformSource: str(take("platformSource")),
			ReleaseAssets:  releaseAssets(take("releaseAssets")),
			LatestRelease:  latestRelease(take("latestRelease")),
		}
		readme, rels := str(take("readme")), releases(take("releases"))
		if readme != "" || len(rels) > 0 {
			blob = &item.Blob{Source: it.Source, ExternalID: it.ExternalID, Readme: readme, Releases: rels, UpdatedAt: old.FetchedAt}
		}
	case domain.SourceHuggingFace:
		it.Hf = &item.HuggingFace{
			Author:        str(take("author")),
			Task:          str(take("task")),
			PipelineTag:   str(take("pipelineTag")),
			Library:       str(take("library")),
			Gated:         boolOf(take("gated")),
			QuantFormats:  nilIfEmpty(strs(take("quantFormats"))),
			Params:        int64Of(take("params")),
			Tags:          nilIfEmpty(strs(take("tags"))),
			BaseModels:    nilIfEmpty(strs(take("baseModels"))),
			Datasets:      nilIfEmpty(strs(take("datasets"))),
			Arxiv:         nilIfEmpty(strs(take("arxiv"))),
			Languages:     nilIfEmpty(strs(take("languages"))),
			TrendingScore: floatOf(take("trendingScore")),
			CreatedAt:     timeOf(take("createdAt")),
			LastModified:  timeOf(take("lastModified")),
		}
	}
	for k := range sd {
		if !used[k] && !ignoredKeys[k] {
			notes.UnknownKeys = append(notes.UnknownKeys, k)
		}
	}
	sort.Strings(notes.UnknownKeys)

	it.Labels, notes.DroppedPaths = labels(old, r)
	it.Jobs = jobs(old)
	return it, blob, notes
}

func labels(old domain.TrackedItem, r Rules) (item.Labels, []string) {
	l := item.Labels{
		Domain: []string{},
		Type:   old.Type,
		Tags:   old.GeneratedTopics,
		By:     old.ClassifiedBy,
		Prev:   []string(old.CategoryPathPrev),
	}
	for _, a := range old.AlternativeTo {
		l.Alternatives = append(l.Alternatives, item.Alternative{Name: a.Name, Slug: a.Slug, Kind: a.Kind})
	}

	hf := old.Source == domain.SourceHuggingFace
	var dropped []string
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			l.Domain = append(l.Domain, p)
		}
	}
	for _, p := range old.CategoryPath {
		switch {
		case p == "":
		case hf || r.ValidPath(p):
			add(p)
		default:
			dropped = append(dropped, p)
		}
	}

	if hf {
		l.By = item.BySource
		return l, dropped
	}

	if s := old.Store; s != nil {
		l.ShelfOverride = s.CategoryOverride
		l.Shelf = s.Category
		if s.CategoryOverride != "" {
			l.Shelf = s.CategoryOverride
		}
		l.HasGui, l.TaglineZh, l.TaglineEn = s.HasGui, s.TaglineZh, s.TaglineEn
		// The effective shelf is also a domain leaf: one tree, shelves as leaves.
		if l.Shelf != "" && l.Shelf != item.ShelfExcluded {
			if p := r.ShelfPath(l.Shelf); r.ValidPath(p) {
				add(p)
			} else {
				dropped = append(dropped, p)
				l.Shelf = ""
			}
		}
	}

	// Only a classified item carries a version; one classified before versions
	// were stamped sits on the base generation.
	if len(l.Domain) > 0 || old.AnalysisStatus == domain.AnalysisDone {
		l.Version = old.TaxonomyVersion
		if l.Version == 0 {
			l.Version = r.BaseVersion
		}
	}
	return l, dropped
}

func jobs(old domain.TrackedItem) item.Jobs {
	j := item.Jobs{
		Judge: item.JobState{Status: old.AnalysisStatus, Fail: old.AnalysisFailCount},
		Alt:   item.JobState{Status: old.AltStatus},
		Icon:  item.JobState{Status: old.IconStatus},
		Shot:  item.JobState{Status: old.ShotStatus},
	}
	if j.Judge.Status == "" {
		j.Judge.Status = item.StatusPending
	}
	if s := old.Store; s != nil {
		j.Store = item.JobState{Status: s.Status, Fail: s.FailCount, Version: s.Version}
	}
	return j
}

// ---- sourceData decoding -----------------------------------------------------
//
// sourceData is an untyped map, so values arrive as whatever the BSON decoder
// produced (primitive.A, primitive.DateTime, int32/int64/float64, nested maps
// or primitive.D). These helpers accept every form and return the zero value
// for anything else.

func str(v any) string {
	s, _ := v.(string)
	return s
}

func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

func floatOf(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

func int64Of(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

func timeOf(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t.UTC()
	case *time.Time:
		if t != nil {
			return t.UTC()
		}
	case primitive.DateTime:
		return t.Time().UTC()
	case string:
		if p, err := time.Parse(time.RFC3339, t); err == nil {
			return p.UTC()
		}
	}
	return time.Time{}
}

func list(v any) []any {
	switch a := v.(type) {
	case primitive.A:
		return a
	case []any:
		return a
	case []string:
		out := make([]any, len(a))
		for i, s := range a {
			out[i] = s
		}
		return out
	}
	return nil
}

// strs always returns a non-nil slice so the field is stored as an array.
func strs(v any) []string {
	out := []string{}
	for _, e := range list(v) {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func asMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case primitive.M:
		return m
	case primitive.D:
		return m.Map()
	}
	return nil
}

func releaseAssets(v any) []item.ReleaseAsset {
	var out []item.ReleaseAsset
	for _, e := range list(v) {
		m := asMap(e)
		if m == nil || str(m["name"]) == "" {
			continue
		}
		out = append(out, item.ReleaseAsset{
			Name: str(m["name"]), Platform: str(m["platform"]), URL: str(m["url"]), Size: int64Of(m["size"]),
		})
	}
	return out
}

func latestRelease(v any) *item.ReleaseRef {
	m := asMap(v)
	if m == nil || str(m["tag"]) == "" {
		return nil
	}
	return &item.ReleaseRef{Tag: str(m["tag"]), PublishedAt: timeOf(m["publishedAt"])}
}

func releases(v any) []item.Release {
	var out []item.Release
	for _, e := range list(v) {
		m := asMap(e)
		if m == nil {
			continue
		}
		out = append(out, item.Release{
			Name: str(m["name"]), TagName: str(m["tagName"]),
			IsPrerelease: boolOf(m["isPrerelease"]), IsLatest: boolOf(m["isLatest"]), IsDraft: boolOf(m["isDraft"]),
			PublishedAt: timeOf(m["publishedAt"]),
		})
	}
	return out
}
