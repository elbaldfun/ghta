package pipeline

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/huggingface"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func TestBoardsJob(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 5, 0, 0, 0, time.UTC)

	d1 := 40.0
	a := gh("al/a", "Go", 5000, "ai/llm-serving")
	a.Gh, a.Growth.D1 = &item.GitHub{Owner: "al", PushedAt: now.Add(-48 * time.Hour)}, &d1
	gone := gh("al/gone", "Go", 90000, "ai/llm-serving")
	gone.Gh, gone.Stale = &item.GitHub{Owner: "al"}, true
	model := gh("m/x", "", 100)
	model.Source = item.SourceHuggingFace
	app := gh("al/app", "Go", 800, "apps/system/files")
	app.Labels.Type, app.Labels.Shelf = "app", "system/files"
	app.Labels.Alternatives = []item.Alternative{{Name: "Finder", Slug: "finder"}}
	lib := gh("al/lib", "Go", 900)
	lib.Labels.Type = "library"
	for _, it := range []item.Item{a, gone, model, app, lib} {
		if _, err := st.Items().InsertOne(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.Developers().InsertOne(ctx, bson.M{"login": "al", "type": "User", "name": "Al"}); err != nil {
		t.Fatal(err)
	}

	// A leftover row from an earlier night, gone from the corpus since.
	if _, err := st.Apps().InsertOne(ctx, bson.M{"_id": "old/app", "listed": true, "at": now.Add(-24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	tree := taxonomy.Tree{Nodes: []taxonomy.Node{
		{Path: "ai", Name: "人工智能"}, {Path: "ai/llm-serving"},
		{Path: "apps", Name: "应用"}, {Path: "apps/system"}, {Path: "apps/system/files"},
	}}
	job := NewBoards(st, tree, []boards.Facet{{Key: "library", Name: "库"}})
	job.now = func() time.Time { return now }
	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 3 {
		t.Fatalf("run = %+v, %v; want only the live GitHub repos read", rep, err)
	}

	var devs []boards.Developer
	at, err := st.Board(ctx, boards.DevelopersKey(boards.DevRising, "ai"), &devs)
	if err != nil || !at.Equal(now) || len(devs) != 1 || devs[0].Login != "al" || devs[0].TotalStars != 5000 || devs[0].Growth != 40 {
		t.Fatalf("ai rising = %+v at %v, %v; the stale repo must not count", devs, at, err)
	}
	var heat []boards.HeatCell
	if _, err := st.Board(ctx, boards.HeatKey, &heat); err != nil || len(heat) != 2 {
		t.Fatalf("heat = %+v, %v", heat, err)
	}
	var stale boards.Staleness
	if _, err := st.Board(ctx, boards.StalenessKey, &stale); err != nil || len(stale.Buckets) != 4 {
		t.Fatalf("staleness = %+v, %v", stale, err)
	}
	ids, err := st.BoardHead(ctx, boards.DevelopersKey(boards.DevRising, "all"), "2026-10-14")
	if err != nil || len(ids) != 1 || ids[0] != "al" {
		t.Fatalf("rising head = %v, %v", ids, err)
	}
	var rows []boards.App
	if _, err := st.ListApps(ctx, store.AppQuery{Sort: boards.AppHot, Limit: 10}, &rows); err != nil || len(rows) != 1 || rows[0].ExternalID != "al/app" {
		t.Fatalf("apps = %+v, %v; want the app listed and last night's leftover removed", rows, err)
	}
	var best []boards.App
	if _, err := st.Board(ctx, boards.BestOfKey("system/files"), &best); err != nil || len(best) != 1 {
		t.Fatalf("best = %+v, %v", best, err)
	}
	var verdict []boards.Verdict
	if _, err := st.Board(ctx, boards.DomainVerdictKey("ai/llm-serving"), &verdict); err != nil || len(verdict) != 1 ||
		verdict[0].Pick.ExternalID != "al/a" || verdict[0].Pick.PushedDaysAgo != 2 || len(verdict[0].Skipped) != 0 {
		t.Fatalf("ai/llm-serving verdict = %+v, %v; the stale repo is not a candidate", verdict, err)
	}
	// The one app has nothing to download: the shelf has no pick.
	var shelfVerdict []boards.Verdict
	if _, err := st.Board(ctx, boards.ShelfVerdictKey("system/files"), &shelfVerdict); err != nil || len(shelfVerdict) != 0 {
		t.Fatalf("system/files verdict = %+v, %v", shelfVerdict, err)
	}
	// Every group's developer and topic boards, every pillar, the five
	// singles, every model board, a best-of and a verdict per shelf, the
	// alternatives, and a verdict per tree node.
	groups := len(boards.Groups([]string{"ai", "apps"}))
	want := int64(groups*4 + len(boards.Pillars)*2 + 5 + (1+len(huggingface.TaskGroups))*len(boards.ModelSorts) + 2 + 1 + len(tree.Nodes))
	if n, _ := st.Boards().CountDocuments(ctx, bson.M{}); n != want {
		t.Fatalf("boards = %d; want %d", n, want)
	}
	if n, _ := st.DB.Collection(store.CollBoardsHistory).CountDocuments(ctx, bson.M{}); n != int64(groups*2+len(boards.Pillars)*2) {
		t.Fatalf("heads = %d", n)
	}
}
