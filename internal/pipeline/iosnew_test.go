package pipeline

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func TestIOSNewJob(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	day := "2026-11-25"
	at, _ := time.Parse(time.DateOnly, day)
	if err := st.WriteChart(ctx, store.ChartSnapshot{Source: item.SourceIOS, Country: "us", Chart: "free", Genre: "all",
		Day: day, FetchedAt: at.Add(9 * time.Hour), Entries: []string{"1"}}); err != nil {
		t.Fatal(err)
	}
	listing := func(id string, released time.Time, places ...item.ChartPos) item.Item {
		return item.Item{Source: item.SourceIOS, ExternalID: id, Topics: []string{}, History: []item.Point{},
			App: &item.StoreApp{ReleasedAt: released, ChartsDay: day, Charts: places}}
	}
	us := item.ChartPos{Country: "us", Chart: "free", Genre: "all", Rank: 5}
	jp := item.ChartPos{Country: "jp", Chart: "paid", Genre: "6007", Rank: 2}
	if _, err := st.Items().InsertMany(ctx, []any{
		listing("1", at.Add(-5*24*time.Hour), us, jp),
		listing("2", at.Add(-200*24*time.Hour), us), // not new
		listing("3", at.Add(-30*24*time.Hour), jp),
	}); err != nil {
		t.Fatal(err)
	}
	// A board older than the kept fortnight goes; a recent one stays.
	for _, k := range []string{boards.IOSNewKey("us", "2026-11-01"), boards.IOSNewKey("us", "2026-11-20")} {
		if err := st.PutBoard(ctx, k, []boards.IOSNewRow{}, at); err != nil {
			t.Fatal(err)
		}
	}

	job := NewIOSNew(st, []string{"us", "jp", "gb"}, nil)
	rep, err := job.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Processed != 2 || rep.Notes["us"] != 1 || rep.Notes["jp"] != 2 || rep.Notes["gb"] != 0 || rep.Notes["pruned"] != int64(1) {
		t.Fatalf("report %+v", rep)
	}
	var jpRows []boards.IOSNewRow
	if _, err := st.Board(ctx, boards.IOSNewKey("jp", day), &jpRows); err != nil {
		t.Fatal(err)
	}
	if len(jpRows) != 2 || jpRows[0].Best != jp || jpRows[0].Charts != 1 {
		t.Errorf("jp board = %+v", jpRows)
	}
	var gb []boards.IOSNewRow
	if _, err := st.Board(ctx, boards.IOSNewKey("gb", day), &gb); err != nil || gb == nil || len(gb) != 0 {
		t.Errorf("gb board = %+v %v; want an empty board", gb, err)
	}
	if n, _ := st.Boards().CountDocuments(ctx, bson.M{"key": boards.IOSNewKey("us", "2026-11-01")}); n != 0 {
		t.Error("old board kept")
	}
	if key, _ := st.LatestDatedBoard(ctx, boards.IOSNewPrefix+"us|"); key != boards.IOSNewKey("us", day) {
		t.Errorf("latest us board = %q", key)
	}
}
