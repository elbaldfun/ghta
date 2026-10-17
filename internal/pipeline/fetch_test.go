package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

// fakeSource serves canned shards and records which were asked for.
type fakeSource struct {
	src      domain.Source // GitHub when empty
	shards   []string
	fail     map[string]bool
	failOnce map[string]bool // fails the first time it is asked for only
	calls    []string
	stars    float64
	hook     func(shard string)
}

func (f *fakeSource) Source() domain.Source {
	if f.src == "" {
		return domain.SourceGitHub
	}
	return f.src
}
func (f *fakeSource) Shards() []string { return f.shards }
func (f *fakeSource) FetchItems(ctx context.Context, shard string) ([]item.Fetched, error) {
	f.calls = append(f.calls, shard)
	if f.hook != nil {
		f.hook(shard)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.fail[shard] {
		return nil, errors.New("rate limited")
	}
	if f.failOnce[shard] {
		delete(f.failOnce, shard)
		return nil, errors.New("github http 502")
	}
	out := make([]item.Fetched, 2)
	for i := range out {
		it := gh(fmt.Sprintf("%s/r%d", shard, i), "Go", f.stars)
		out[i] = item.Fetched{Item: it}
	}
	return out, nil
}

func TestFetchResumesByShard(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	day := time.Date(2026, 10, 12, 3, 30, 0, 0, time.UTC)
	src := &fakeSource{shards: []string{"a", "b", "c"}, fail: map[string]bool{"b": true}, stars: 100}
	job := NewFetch(st, nil, src)
	job.now = func() time.Time { return day }
	job.retryPause = 0

	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 4 || rep.Failed != 1 {
		t.Fatalf("first run = %+v, %v; one failing shard must not fail the run", rep, err)
	}
	if strings.Join(src.calls, ",") != "a,b,c,b" {
		t.Fatalf("first run fetched %v; the failed shard is tried once more at the end", src.calls)
	}
	tally := rep.Notes["github"].(sourceTally)
	if tally.Shards != 3 || tally.Failed != 1 || tally.New != 4 || tally.Snapshots != 4 {
		t.Fatalf("tally = %+v", tally)
	}
	if s, _ := st.ShardStatus(ctx, "github", "2026-10-12", "b"); s != store.ShardFailed {
		t.Fatalf("shard b status = %q", s)
	}

	// Same day again: only the failed shard is fetched.
	src.fail, src.calls = nil, nil
	rep, err = job.Run(ctx)
	if err != nil || strings.Join(src.calls, ",") != "b" || rep.Processed != 2 {
		t.Fatalf("rerun fetched %v, report %+v, %v", src.calls, rep, err)
	}
	if got := rep.Notes["github"].(sourceTally); got.Skipped != 2 {
		t.Fatalf("rerun tally = %+v", got)
	}

	// Next day: everything again, and growth appears.
	src.calls, src.stars = nil, 130
	job.now = func() time.Time { return day.Add(24 * time.Hour) }
	if rep, err = job.Run(ctx); err != nil || len(src.calls) != 3 || rep.Processed != 6 {
		t.Fatalf("next day fetched %v, report %+v, %v", src.calls, rep, err)
	}
	var it item.Item
	if err := st.Items().FindOne(ctx, bson.M{"externalId": "a/r0"}).Decode(&it); err != nil {
		t.Fatal(err)
	}
	if it.Growth.D1 == nil || *it.Growth.D1 != 30 || len(it.History) != 2 {
		t.Fatalf("growth after two days = %+v, history %v", it.Growth, it.History)
	}
}

func TestFetchRetriesFailedShardsOnce(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	src := &fakeSource{shards: []string{"a", "b", "c"}, failOnce: map[string]bool{"b": true}, stars: 100}
	job := NewFetch(st, nil, src)
	job.retryPause = 0

	rep, err := job.Run(ctx)
	if err != nil || rep.Failed != 0 || rep.Processed != 6 {
		t.Fatalf("report %+v, %v; a shard that fails once must be recovered in the same run", rep, err)
	}
	if got := rep.Notes["github"].(sourceTally); got.Retried != 1 || got.Failed != 0 || got.Snapshots != 6 {
		t.Fatalf("tally = %+v", got)
	}
	if s, _ := st.ShardStatus(ctx, "github", item.DayOf(job.now()), "b"); s != store.ShardDone {
		t.Fatalf("shard b status = %q", s)
	}
}

func TestFetchFailsWhenEverythingFails(t *testing.T) {
	st := testStore(t)
	src := &fakeSource{shards: []string{"a", "b"}, fail: map[string]bool{"a": true, "b": true}}
	rep, err := NewFetch(st, nil, src).Run(context.Background())
	if len(src.calls) != 2 {
		t.Fatalf("fetched %v; when every shard fails there is no retry pass", src.calls)
	}
	if err == nil || !strings.Contains(err.Error(), "rate limited") || rep.Failed != 2 {
		t.Fatalf("report %+v, err %v; a run where every shard fails must fail", rep, err)
	}
}

// A source that fetched nothing must not age its items' growth, and fails the
// run even when another source was fetched.
func TestFetchLeavesGrowthWhenASourceIsDown(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	known := gh("o/known", "Go", 120)
	known.History = []item.Point{{D: "2026-10-15", V: 100}, {D: "2026-10-16", V: 120}}
	known.Growth = item.ComputeGrowth(known.History, 120, "2026-10-16")
	if _, err := st.Items().InsertOne(ctx, known); err != nil {
		t.Fatal(err)
	}
	down := &fakeSource{shards: []string{"a", "b"}, fail: map[string]bool{"a": true, "b": true}}
	up := &fakeSource{src: domain.SourceHuggingFace, shards: []string{"m"}, stars: 5}
	job := NewFetch(st, nil, down, up)
	job.now = func() time.Time { return time.Date(2026, 10, 17, 3, 30, 0, 0, time.UTC) }

	rep, err := job.Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "github") || rep.Processed != 2 {
		t.Fatalf("report %+v, err %v; the other source is fetched, the run still fails", rep, err)
	}
	if got := rep.Notes["github"].(sourceTally); got.Aged != 0 || got.Failed != 2 {
		t.Fatalf("github tally = %+v", got)
	}
	var after item.Item
	if err := st.Items().FindOne(ctx, bson.M{"externalId": "o/known"}).Decode(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Growth.Equal(known.Growth) {
		t.Fatalf("growth = %+v; want it left at %+v", after.Growth, known.Growth)
	}
}

func TestFetchStopsOnCancel(t *testing.T) {
	st := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	src := &fakeSource{shards: []string{"a", "b", "c"}, stars: 1}
	src.hook = func(shard string) {
		if shard == "b" {
			cancel()
		}
	}
	job := NewFetch(st, nil, src)
	rep, err := job.Run(ctx)
	if !errors.Is(err, context.Canceled) || strings.Join(src.calls, ",") != "a,b" || rep.Processed != 2 {
		t.Fatalf("calls %v, report %+v, err %v", src.calls, rep, err)
	}
	// The cut-off shard is not left "running", so the next run retries it.
	date := item.DayOf(time.Now())
	if s, _ := st.ShardStatus(context.Background(), "github", date, "b"); s != store.ShardFailed {
		t.Fatalf("interrupted shard status = %q", s)
	}
	if rep.Failed != 0 {
		t.Fatalf("an interrupted shard is not a failed one: %+v", rep)
	}
}
