package pipeline

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

func TestBoardsJob(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 5, 0, 0, 0, time.UTC)

	d1 := 40.0
	a := gh("al/a", "Go", 5000, "ai/llm-serving")
	a.Gh, a.Growth.D1 = &item.GitHub{Owner: "al"}, &d1
	gone := gh("al/gone", "Go", 90000, "ai/llm-serving")
	gone.Gh, gone.Stale = &item.GitHub{Owner: "al"}, true
	model := gh("m/x", "", 100)
	model.Source = item.SourceHuggingFace
	for _, it := range []item.Item{a, gone, model} {
		if _, err := st.Items().InsertOne(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.Developers().InsertOne(ctx, bson.M{"login": "al", "type": "User", "name": "Al"}); err != nil {
		t.Fatal(err)
	}

	tree := taxonomy.Tree{Nodes: []taxonomy.Node{{Path: "ai", Name: "人工智能", Children: []taxonomy.Node{{Path: "ai/llm-serving"}}}}}
	job := NewBoards(st, tree, []boards.Facet{{Key: "library", Name: "库"}})
	job.now = func() time.Time { return now }
	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 1 {
		t.Fatalf("run = %+v, %v; want only the live GitHub repo read", rep, err)
	}

	var devs []boards.Developer
	at, err := st.Board(ctx, boards.DevelopersKey(boards.DevRising, "ai"), &devs)
	if err != nil || !at.Equal(now) || len(devs) != 1 || devs[0].Login != "al" || devs[0].TotalStars != 5000 || devs[0].Growth != 40 {
		t.Fatalf("ai rising = %+v at %v, %v; the stale repo must not count", devs, at, err)
	}
	var heat []boards.HeatCell
	if _, err := st.Board(ctx, boards.HeatKey, &heat); err != nil || len(heat) != 1 || heat[0].Name != "人工智能" {
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
	// Every group's developer and topic boards, every pillar, and the five singles.
	groups := len(boards.Groups([]string{"ai"}))
	want := int64(groups*4 + len(boards.Pillars)*2 + 5)
	if n, _ := st.Boards().CountDocuments(ctx, bson.M{}); n != want {
		t.Fatalf("boards = %d; want %d", n, want)
	}
	if n, _ := st.DB.Collection(store.CollBoardsHistory).CountDocuments(ctx, bson.M{}); n != int64(groups*2+len(boards.Pillars)*2) {
		t.Fatalf("heads = %d", n)
	}
}
