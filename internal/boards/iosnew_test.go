package boards

import (
	"testing"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
)

func TestIOSNew(t *testing.T) {
	day := "2026-11-25"
	at, _ := time.Parse(time.DateOnly, day)
	app := func(id string, released time.Time, chartsDay string, places ...item.ChartPos) item.Item {
		return item.Item{ExternalID: id, App: &item.StoreApp{ReleasedAt: released, ChartsDay: chartsDay, Charts: places}}
	}
	us := func(chart, genre string, rank int) item.ChartPos {
		return item.ChartPos{Country: "us", Chart: chart, Genre: genre, Rank: rank, Prev: rank + 3}
	}
	jp := item.ChartPos{Country: "jp", Chart: "free", Genre: "all", Rank: 1}
	recent, older := at.Add(-10*24*time.Hour), at.Add(-80*24*time.Hour)
	stale := app("9", recent, day, us("free", "all", 1))
	stale.Stale = true
	apps := []item.Item{
		app("1", recent, day, us("paid", "6007", 3), us("paid", "all", 40)),
		app("2", older, day, us("free", "6015", 3)),                        // ties 1 on rank 3, fewer charts
		app("3", recent, day, us("grossing", "all", 3)),                    // ties on rank 3 with an overall place: first
		app("4", recent, day, us("free", "6007", 7), jp),                   // its jp place does not count in us
		app("5", at.Add(-100*24*time.Hour), day, us("free", "all", 1)),     // released too long ago
		app("6", recent, "2026-11-24", us("free", "all", 2)),               // places from an older day
		app("7", recent, day, jp),                                          // not on any us chart
		app("8", older, day, us("paid", "6007", 3), us("free", "6011", 9)), // ties 1 on rank and charts, older
		stale,
		{ExternalID: "10"}, // no listing
	}
	rows := IOSNew(apps, "us", day)
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if got, want := ids, []string{"3", "1", "8", "2", "4"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] || got[4] != want[4] {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if r := rows[1]; r.Best.Chart != "paid" || r.Best.Genre != "6007" || r.Best.Rank != 3 || r.Best.Prev != 0 || r.Charts != 2 {
		t.Errorf("row 1 = %+v", r)
	}

	// Two places of the same rank: the overall chart shows, then free before paid.
	tie := IOSNew([]item.Item{app("1", recent, day, us("paid", "6007", 5), us("free", "6011", 5), us("paid", "all", 5))}, "us", day)
	if len(tie) != 1 || tie[0].Best.Chart != "paid" || tie[0].Best.Genre != "all" {
		t.Errorf("tie on the overall chart = %+v", tie)
	}
	tie = IOSNew([]item.Item{app("1", recent, day, us("paid", "6007", 5), us("free", "6011", 5))}, "us", day)
	if tie[0].Best.Chart != "free" {
		t.Errorf("tie between genre charts = %+v", tie)
	}
	if IOSNew(apps, "us", "bad day") == nil || len(IOSNew(nil, "us", day)) != 0 {
		t.Error("empty input should give an empty, non-nil board")
	}
	if IOSNewKey("us", day) != "ios-new|us|2026-11-25" {
		t.Error(IOSNewKey("us", day))
	}
}
