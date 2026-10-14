package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/github"
)

type fakeSearch map[string]item.Fetched

func (f fakeSearch) SearchItems(_ context.Context, query string, _ int) ([]item.Fetched, error) {
	if r, ok := f[query]; ok {
		return []item.Fetched{r}, nil
	}
	return nil, nil
}

func TestHotJob(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 5, 0, 0, 0, time.UTC)

	known := gh("al/known", "Go", 5000, "ai/llm-serving")
	known.Labels.TaglineZh = "已知"
	if _, err := st.Items().InsertOne(ctx, known); err != nil {
		t.Fatal(err)
	}
	if err := st.PutBoard(ctx, boards.HotKey("monthly"), []boards.HotRepo{{FullName: "old/kept"}}, now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	lists := map[string][]github.TrendingRepo{
		"daily":  {{ExternalID: "al/known", Owner: "al", Name: "known", StarsThisPeriod: 300}, {ExternalID: "new/one", StarsThisPeriod: 200}},
		"weekly": {{ExternalID: "new/one"}, {ExternalID: "gone/repo"}},
	}
	scrape := func(_ context.Context, w string) ([]github.TrendingRepo, error) {
		if r, ok := lists[w]; ok {
			return r, nil
		}
		return nil, errors.New("blocked")
	}
	newcomer := item.Fetched{Item: gh("new/one", "Rust", 1200)}
	job := NewHot(st, scrape, fakeSearch{"repo:new/one": newcomer}, nil)
	job.now = func() time.Time { return now }
	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 4 || rep.Failed != 1 {
		t.Fatalf("run = %+v, %v; want 4 rows written and monthly failed", rep, err)
	}

	var daily []boards.HotRepo
	if _, err := st.Board(ctx, boards.HotKey("daily"), &daily); err != nil || len(daily) != 2 {
		t.Fatalf("daily = %+v, %v", daily, err)
	}
	if d := daily[0]; !d.Tracked || d.TaglineZh != "已知" || d.StarsGained != 300 || len(d.Domain) != 1 {
		t.Fatalf("daily[0] = %+v; want joined to its labels", d)
	}
	if !daily[1].Tracked {
		t.Fatalf("daily[1] = %+v; a newcomer taken in counts as tracked", daily[1])
	}
	var weekly []boards.HotRepo
	if _, err := st.Board(ctx, boards.HotKey("weekly"), &weekly); err != nil || weekly[1].Tracked {
		t.Fatalf("weekly = %+v, %v; a repo search cannot find stays untracked", weekly, err)
	}
	var monthly []boards.HotRepo
	if _, err := st.Board(ctx, boards.HotKey("monthly"), &monthly); err != nil || len(monthly) != 1 || monthly[0].FullName != "old/kept" {
		t.Fatalf("monthly = %+v, %v; a failed scrape keeps the board", monthly, err)
	}
	if n, _ := st.Items().CountDocuments(ctx, bson.M{"externalId": "new/one"}); n != 1 {
		t.Fatalf("newcomer stored %d times", n)
	}
}

func TestHotJobAllFailed(t *testing.T) {
	st := testStore(t)
	scrape := func(context.Context, string) ([]github.TrendingRepo, error) { return nil, errors.New("blocked") }
	if _, err := NewHot(st, scrape, fakeSearch{}, nil).Run(context.Background()); err == nil {
		t.Fatal("want an error when no window scrapes")
	}
}
