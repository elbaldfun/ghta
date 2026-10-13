package importer

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
)

var testRules = Rules{
	ValidPath: func(p string) bool {
		return map[string]bool{
			"web/frontend": true, "agents/frameworks": true, "data/database": true,
			"apps/productivity/notes": true, "apps/ai/assistant": true,
		}[p]
	},
	ShelfPath: func(slug string) string {
		if slug == "data/database" {
			return slug
		}
		return "apps/" + slug
	},
	BaseVersion: 2,
}

// decode round-trips a document through BSON so Convert sees exactly the Go
// types the driver produces for a stored tracked_items document.
func decode(t *testing.T, doc bson.M) domain.TrackedItem {
	t.Helper()
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var old domain.TrackedItem
	if err := bson.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	return old
}

func TestConvertGitHub(t *testing.T) {
	pushed := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	week := 900.0
	old := decode(t, bson.M{
		"source": "github", "externalId": "o/notes", "name": "notes", "description": "d", "language": "Go",
		"categoryPath": bson.A{"web/frontend", "ai/llm", "web/frontend"}, "categoryId": bson.A{"x"},
		"type": "app", "primaryMetric": "stars", "metricDirection": "desc-better",
		"metrics":       bson.M{"stars": 5000.0},
		"dailyIncrease": nil, "weeklyIncrease": week, "monthlyIncrease": nil,
		"analysisStatus": "done", "analysisFailCount": 1, "classifiedBy": "llm", "taxonomyVersion": 3,
		"generatedTopics": bson.A{"note"}, "categoryPathPrev": bson.A{"web/css-ui"},
		"alternativeTo": bson.A{bson.M{"name": "Notion", "slug": "notion", "kind": "note-taking"}},
		"altStatus":     "done", "iconUrl": "https://i/x.png", "iconStatus": "done",
		"screenshotUrl": "https://s/x.png", "shotSource": "readme", "shotStatus": "done",
		"store": bson.M{"category": "ai/assistant", "categoryOverride": "productivity/notes", "taglineZh": "笔记",
			"taglineEn": "Notes", "hasGui": true, "status": "done", "failCount": 2, "version": 2},
		"sourceData": bson.M{
			"owner": "o", "url": "https://github.com/o/notes", "homepageUrl": "https://notes.app", "license": "MIT License",
			"pushedAt": pushed, "topics": bson.A{bson.M{"name": "notes", "url": "u"}}, "topicNames": bson.A{"notes", "markdown"},
			"releases": bson.A{bson.M{"name": "v1", "tagName": "v1.0.0", "isLatest": true, "publishedAt": pushed}},
			"readme":   "# Notes", "platforms": bson.A{"macos", "linux"}, "platformSource": "asset",
			"releaseAssets": bson.A{bson.M{"name": "notes.dmg", "platform": "macos", "url": "https://d/notes.dmg", "size": int64(1234)}},
			"latestRelease": bson.M{"tag": "v1.0.0", "publishedAt": pushed},
			"migratedFrom":  "githubtrends", "mystery": 1,
		},
		"fetchedAt": pushed, "createdAt": pushed, "updatedAt": pushed,
	})

	it, blob, notes := Convert(old, testRules)

	if !reflect.DeepEqual(it.Labels.Domain, []string{"web/frontend", "apps/productivity/notes"}) {
		t.Errorf("domain: dedupe, drop non-leaf, add effective shelf leaf; got %v", it.Labels.Domain)
	}
	if !reflect.DeepEqual(notes.DroppedPaths, []string{"ai/llm"}) {
		t.Errorf("dropped paths %v", notes.DroppedPaths)
	}
	if !reflect.DeepEqual(notes.UnknownKeys, []string{"mystery"}) {
		t.Errorf("only keys with no destination are reported, got %v", notes.UnknownKeys)
	}
	l := it.Labels
	if l.Shelf != "productivity/notes" || l.ShelfOverride != "productivity/notes" || l.HasGui == nil || !*l.HasGui ||
		l.TaglineZh != "笔记" || l.TaglineEn != "Notes" || l.Type != "app" || l.By != "llm" || l.Version != 3 {
		t.Errorf("labels %+v", l)
	}
	if len(l.Alternatives) != 1 || l.Alternatives[0].Slug != "notion" || !reflect.DeepEqual(l.Tags, []string{"note"}) ||
		!reflect.DeepEqual(l.Prev, []string{"web/css-ui"}) {
		t.Errorf("alternatives/tags/prev %+v", l)
	}
	if it.Growth.D1 != nil || it.Growth.D7 == nil || *it.Growth.D7 != 900 || it.Growth.D30 != nil {
		t.Errorf("growth keeps nil windows: %+v", it.Growth)
	}
	if it.Homepage != "https://notes.app" || it.License != "MIT License" || !reflect.DeepEqual(it.Topics, []string{"notes", "markdown"}) {
		t.Errorf("top-level fields: %q %q %v", it.Homepage, it.License, it.Topics)
	}
	gh := it.Gh
	if gh == nil || gh.Owner != "o" || !gh.PushedAt.Equal(pushed) || gh.PlatformSource != "asset" ||
		!reflect.DeepEqual(gh.Platforms, []string{"macos", "linux"}) ||
		len(gh.ReleaseAssets) != 1 || gh.ReleaseAssets[0].Size != 1234 || gh.ReleaseAssets[0].Platform != "macos" ||
		gh.LatestRelease == nil || gh.LatestRelease.Tag != "v1.0.0" || !gh.LatestRelease.PublishedAt.Equal(pushed) {
		t.Errorf("gh %+v", gh)
	}
	if it.Hf != nil {
		t.Error("a GitHub item has no hf block")
	}
	j := it.Jobs
	if j.Judge != (item.JobState{Status: "done", Fail: 1}) || j.Store != (item.JobState{Status: "done", Fail: 2, Version: 2}) ||
		j.Alt.Status != "done" || j.Icon.Status != "done" || j.Shot.Status != "done" {
		t.Errorf("jobs %+v", j)
	}
	if it.Assets != (item.Assets{IconURL: "https://i/x.png", ScreenshotURL: "https://s/x.png", ShotSource: "readme"}) {
		t.Errorf("assets %+v", it.Assets)
	}
	if blob == nil || blob.Readme != "# Notes" || len(blob.Releases) != 1 || blob.Releases[0].TagName != "v1.0.0" ||
		!blob.Releases[0].IsLatest || !blob.Releases[0].PublishedAt.Equal(pushed) || blob.ExternalID != "o/notes" {
		t.Errorf("blob %+v", blob)
	}

	// The stored document has none of the fields the rebuild drops.
	raw, err := bson.Marshal(it)
	if err != nil {
		t.Fatal(err)
	}
	var stored bson.M
	if err := bson.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"categoryId", "categoryPath", "sourceData", "store", "analysisStatus", "weeklyIncrease", "ranks", "hf"} {
		if _, ok := stored[k]; ok {
			t.Errorf("stored item still has %q", k)
		}
	}
}

func TestConvertLegacyShapes(t *testing.T) {
	base := func(extra bson.M) bson.M {
		d := bson.M{"source": "github", "externalId": "o/x", "name": "x", "metrics": bson.M{"stars": 1.0}}
		for k, v := range extra {
			d[k] = v
		}
		return d
	}
	cases := []struct {
		name        string
		doc         bson.M
		wantDomain  []string
		wantShelf   string
		wantVersion int
		wantJudge   string
		wantDropped []string
	}{
		{"empty string path, never classified",
			base(bson.M{"categoryPath": "", "analysisStatus": "failed"}), []string{}, "", 0, "failed", nil},
		{"single string path, pre-versioning",
			base(bson.M{"categoryPath": "web/frontend", "analysisStatus": "done"}), []string{"web/frontend"}, "", 2, "done", nil},
		{"missing path and status",
			base(nil), []string{}, "", 0, "pending", nil},
		{"done with no labels keeps a version so it is not re-queued as unversioned",
			base(bson.M{"categoryPath": bson.A{}, "analysisStatus": "done"}), []string{}, "", 2, "done", nil},
		{"excluded shelf adds no apps leaf",
			base(bson.M{"categoryPath": bson.A{"web/frontend"}, "analysisStatus": "done", "store": bson.M{"category": "excluded", "status": "done"}}),
			[]string{"web/frontend"}, "excluded", 2, "done", nil},
		{"developer shelf is its own domain leaf",
			base(bson.M{"categoryPath": "", "store": bson.M{"category": "data/database"}}), []string{"data/database"}, "data/database", 2, "pending", nil},
		{"shelf that no longer exists is cleared and reported",
			base(bson.M{"categoryPath": bson.A{"web/frontend"}, "analysisStatus": "done", "store": bson.M{"category": "devtools/devops"}}),
			[]string{"web/frontend"}, "", 2, "done", []string{"apps/devtools/devops"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			it, blob, notes := Convert(decode(t, c.doc), testRules)
			if !reflect.DeepEqual(it.Labels.Domain, c.wantDomain) {
				t.Errorf("domain %v, want %v", it.Labels.Domain, c.wantDomain)
			}
			if it.Labels.Shelf != c.wantShelf || it.Labels.Version != c.wantVersion || it.Jobs.Judge.Status != c.wantJudge {
				t.Errorf("shelf=%q version=%d judge=%q", it.Labels.Shelf, it.Labels.Version, it.Jobs.Judge.Status)
			}
			if !reflect.DeepEqual(notes.DroppedPaths, c.wantDropped) {
				t.Errorf("dropped %v, want %v", notes.DroppedPaths, c.wantDropped)
			}
			if blob != nil {
				t.Error("no readme and no releases means no blob")
			}
			if it.Topics == nil || it.History == nil || it.Metrics == nil {
				t.Error("topics, history and metrics are never null in a stored item")
			}
		})
	}
}

func TestConvertHuggingFace(t *testing.T) {
	mod := time.Date(2026, 1, 17, 2, 22, 55, 0, time.UTC)
	old := decode(t, bson.M{
		"source": "huggingface", "externalId": "a/m", "name": "m", "categoryPath": bson.A{"hf/text-generation"},
		"primaryMetric": "downloads", "metrics": bson.M{"downloads": 10.0, "likes": 2.0}, "analysisStatus": "done",
		"sourceData": bson.M{
			"author": "a", "url": "https://huggingface.co/a/m", "pipelineTag": "text-generation", "task": "text-generation",
			"library": "transformers", "license": "apache-2.0", "gated": true, "quantFormats": bson.A{"gguf"},
			"params": int64(7_000_000_000), "tags": bson.A{"gguf", "en"}, "baseModels": bson.A{"b/base"},
			"datasets": nil, "arxiv": nil, "languages": bson.A{"en"}, "trendingScore": 1.5,
			"createdAt": mod, "lastModified": mod,
		},
	})
	it, blob, notes := Convert(old, testRules)
	if blob != nil || len(notes.UnknownKeys) != 0 || len(notes.DroppedPaths) != 0 {
		t.Errorf("blob=%v notes=%+v", blob, notes)
	}
	if !reflect.DeepEqual(it.Labels.Domain, []string{"hf/text-generation"}) || it.Labels.By != item.BySource || it.Labels.Version != 0 {
		t.Errorf("hf labels keep the source's own path and carry no tree version: %+v", it.Labels)
	}
	hf := it.Hf
	if hf == nil || hf.Author != "a" || hf.Task != "text-generation" || hf.Library != "transformers" || !hf.Gated ||
		hf.Params != 7_000_000_000 || hf.TrendingScore != 1.5 || !hf.LastModified.Equal(mod) || hf.Datasets != nil ||
		!reflect.DeepEqual(hf.BaseModels, []string{"b/base"}) || !reflect.DeepEqual(hf.QuantFormats, []string{"gguf"}) {
		t.Errorf("hf %+v", hf)
	}
	if it.License != "apache-2.0" || it.Gh != nil {
		t.Errorf("license=%q gh=%v", it.License, it.Gh)
	}
}
