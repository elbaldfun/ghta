package pipeline

import (
	"context"
	"reflect"
	"testing"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func TestMigrate(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	// judged is an item the judge finished on tree v2.
	judged := func(id, shelf string, domain ...string) item.Item {
		it := gh(id, "Go", 1000, domain...)
		it.Labels.Version, it.Labels.Shelf, it.Labels.By = 2, shelf, item.ByLLM
		it.Jobs.Judge.Status = item.StatusDone
		return it
	}
	renamed := judged("o/renamed", "", "web/css-ui", "lang/utils2")
	merged := judged("o/merged", "", "infra/ci-cd", "infra/automation") // lands on a leaf it already has
	split := judged("o/split", "", "ai/llm", "data/database")
	manual := judged("o/manual", "", "ai/llm")
	manual.Labels.By = item.ByManual
	onShelf := judged("o/onshelf", "productivity/notes", "apps/productivity", "data/database")
	excluded := judged("o/excluded", item.ShelfExcluded, "apps/productivity", "web/backend")
	bare := judged("o/bare", "", "apps/productivity") // no shelf, no other domain
	ext := judged("o/ext", "", "apps/browser-ext")    // the fallback shelf applies
	shelfMerge := judged("o/shelfmerge", "selfhosted/photos", "apps/selfhosted/photos", "data/database")
	shelfMoved := judged("o/shelfmoved", "devtools/git", "lang/utils2")
	override := judged("o/override", "ai/platform", "ai/ml-framework")
	override.Labels.ShelfOverride = "ai/platform"
	rejudge := judged("o/rejudge", "devtools/devops", "infra/containers")
	pinned := judged("o/pinned", "devtools/devops", "infra/containers")
	pinned.Labels.ShelfOverride = "devtools/devops"
	both := judged("o/both", "social/chat", "web/css-ui") // a renamed leaf and a merged shelf
	untouched := judged("o/untouched", "", "data/database")
	current := judged("o/current", "", "ai/llm") // already on v3: "ai/llm" here is not the old leaf's business
	current.Labels.Version = 3
	keptPrev := judged("o/keptprev", "", "web/css-ui")
	keptPrev.Labels.Prev = []string{"older/snapshot"}
	model := item.Item{Source: item.SourceHuggingFace, ExternalID: "a/model", Topics: []string{}, History: []item.Point{},
		Labels: item.Labels{Domain: []string{"hf/text-generation"}, By: item.BySource}}
	model.Jobs.Judge.Status = item.StatusDone
	neverJudged := gh("o/pending", "Go", 1000, "web/css-ui")
	neverJudged.Jobs.Judge.Status = item.StatusPending

	for _, it := range []item.Item{renamed, merged, split, manual, onShelf, excluded, bare, ext, shelfMerge, shelfMoved,
		override, rejudge, pinned, both, untouched, current, keptPrev, neverJudged, model} {
		if _, err := st.Items().InsertOne(ctx, it); err != nil {
			t.Fatal(err)
		}
	}

	steps := []taxonomy.Migration{
		// A step for a tree this build does not carry: ignored.
		{From: 3, To: 4, Domain: []taxonomy.Op{{Kind: taxonomy.OpRename, Old: "data/database", New: "data/stores"}}},
		{From: 2, To: 3,
			Domain: []taxonomy.Op{
				{Kind: taxonomy.OpRename, Old: "web/css-ui", New: "web/ui-components"},
				{Kind: taxonomy.OpMerge, Old: "infra/ci-cd", New: "infra/automation"},
				{Kind: taxonomy.OpSplit, Old: "ai/llm", Candidates: []string{"ai/llm-serving", "ai/llm-training"}},
				{Kind: taxonomy.OpShelf, Old: "apps/productivity"},
				{Kind: taxonomy.OpShelf, Old: "apps/browser-ext", Fallback: "network/browser-ext"},
			},
			Shelves: []taxonomy.Op{
				{Kind: taxonomy.OpMerge, Old: "selfhosted/photos", New: "selfhosted/cloud"},
				{Kind: taxonomy.OpMerge, Old: "social/chat", New: "social/messaging"},
				{Kind: taxonomy.OpRename, Old: "ai/platform", New: "ai/builder"},
				{Kind: taxonomy.OpMoved, Old: "devtools/git", New: "devtools/vcs"},
				{Kind: taxonomy.OpRejudge, Old: "devtools/devops", Candidates: []string{"infra/containers", "infra/automation"}},
			}},
	}
	job := NewMigrate(st, taxonomy.Tree{Version: 3}, steps, nil)
	rep, err := job.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	type want struct {
		domain  []string
		shelf   string
		version int
		status  string
		prev    []string
	}
	done, pend := item.StatusDone, item.StatusPending
	cases := map[string]want{
		"o/renamed": {[]string{"web/ui-components", "lang/utils2"}, "", 3, done, []string{"web/css-ui", "lang/utils2"}},
		"o/merged":  {[]string{"infra/automation"}, "", 3, done, []string{"infra/ci-cd", "infra/automation"}},
		// Split: back to the judge, old labels and old version kept meanwhile.
		"o/split":  {[]string{"ai/llm", "data/database"}, "", 2, pend, []string{"ai/llm", "data/database"}},
		"o/manual": {[]string{"ai/llm"}, "", 3, done, nil}, // a person's labels are not re-judged
		// The retired apps leaf becomes the leaf of the item's shelf.
		"o/onshelf":  {[]string{"data/database", "apps/productivity/notes"}, "productivity/notes", 3, done, []string{"apps/productivity", "data/database"}},
		"o/excluded": {[]string{"web/backend"}, item.ShelfExcluded, 3, done, []string{"apps/productivity", "web/backend"}},
		"o/bare":     {[]string{}, "", 2, pend, []string{"apps/productivity"}},
		"o/ext":      {[]string{"apps/network/browser-ext"}, "", 3, done, []string{"apps/browser-ext"}},
		// A shelf's leaf in the domain follows the shelf.
		"o/shelfmerge": {[]string{"apps/selfhosted/cloud", "data/database"}, "selfhosted/cloud", 3, done, []string{"apps/selfhosted/photos", "data/database"}},
		"o/shelfmoved": {[]string{"lang/utils2", "devtools/vcs"}, "devtools/vcs", 3, done, []string{"lang/utils2"}},
		"o/override":   {[]string{"ai/ml-framework", "apps/ai/builder"}, "ai/builder", 3, done, []string{"ai/ml-framework"}},
		"o/rejudge":    {[]string{"infra/containers"}, "devtools/devops", 2, pend, []string{"infra/containers"}},
		"o/pinned":     {[]string{"infra/containers"}, "devtools/devops", 3, done, nil},
		// Two steps hit the same item; the first does not hide it from the second.
		"o/both":      {[]string{"web/ui-components", "apps/social/messaging"}, "social/messaging", 3, done, []string{"web/css-ui"}},
		"o/untouched": {[]string{"data/database"}, "", 3, done, nil},
		"o/current":   {[]string{"ai/llm"}, "", 3, done, nil},
		"o/keptprev":  {[]string{"web/ui-components"}, "", 3, done, []string{"older/snapshot"}},
		// Labelled by its source: outside the tree, never versioned.
		"a/model": {[]string{"hf/text-generation"}, "", 0, done, nil},
		// Never judged: renamed, and still the judge's to finish.
		"o/pending": {[]string{"web/ui-components"}, "", 0, pend, []string{"web/css-ui"}},
	}
	check := func(when string) {
		t.Helper()
		got := itemsByID(t, st)
		for id, w := range cases {
			it := got[id]
			if !reflect.DeepEqual(it.Labels.Domain, w.domain) || it.Labels.Shelf != w.shelf || it.Labels.Version != w.version ||
				it.Jobs.Judge.Status != w.status || !(len(it.Labels.Prev) == 0 && len(w.prev) == 0 || reflect.DeepEqual(it.Labels.Prev, w.prev)) {
				t.Errorf("%s %s: domain %v shelf %q v%d %s prev %v\n   want domain %v shelf %q v%d %s prev %v", when, id,
					it.Labels.Domain, it.Labels.Shelf, it.Labels.Version, it.Jobs.Judge.Status, it.Labels.Prev,
					w.domain, w.shelf, w.version, w.status, w.prev)
			}
		}
		if o := got["o/override"].Labels.ShelfOverride; o != "ai/builder" {
			t.Errorf("%s: override %q did not follow the rename", when, o)
		}
	}
	check("first run")
	if rep.Notes["requeued"] != int64(3) || rep.Notes["rewritten"] == int64(0) {
		t.Errorf("report %+v", rep)
	}

	// Nothing is left behind except what waits for the judge: a second run is a no-op.
	rep, err = job.Run(ctx)
	if err != nil || rep.Processed != 0 || rep.Notes["stamped"] != int64(0) {
		t.Errorf("second run: %+v, %v", rep, err)
	}
	check("second run")
}
