package pipeline

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/store"
)

// scripted answers each item with the next verdict of its script; an item
// with no script left is missing from the reply. It records what it saw.
type scripted struct {
	answers  map[string][]judge.Verdict
	failCall map[int]bool // 1-based call numbers that fail outright
	calls    int
	asked    map[string]int
	fullSeen map[string][]bool
	readme   map[string]string
}

func (s *scripted) Ask(_ context.Context, batch []judge.Input, full bool) (map[string]judge.Verdict, error) {
	s.calls++
	if s.failCall[s.calls] {
		return nil, errors.New("model down")
	}
	out := map[string]judge.Verdict{}
	for _, in := range batch {
		n := s.asked[in.ID]
		s.asked[in.ID]++
		s.fullSeen[in.ID] = append(s.fullSeen[in.ID], full)
		s.readme[in.ID] = in.Readme
		if n < len(s.answers[in.ID]) {
			out[in.ID] = s.answers[in.ID][n]
		}
	}
	return out, nil
}

func newScripted(answers map[string][]judge.Verdict) *scripted {
	return &scripted{answers: answers, failCall: map[int]bool{}, asked: map[string]int{}, fullSeen: map[string][]bool{}, readme: map[string]string{}}
}

func verdict(typ, shelf string, domain ...string) judge.Verdict {
	return judge.Verdict{Paths: domain, Type: typ, Shelf: shelf}
}

func itemsByID(t *testing.T, st *store.Store) map[string]item.Item {
	t.Helper()
	cur, err := st.Items().Find(context.Background(), bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	var items []item.Item
	if err := cur.All(context.Background(), &items); err != nil {
		t.Fatal(err)
	}
	out := map[string]item.Item{}
	for _, it := range items {
		out[it.ExternalID] = it
	}
	return out
}

func pending(it item.Item) item.Item {
	it.Jobs.Judge.Status = item.StatusPending
	return it
}

func TestJudge(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	ex := item.ShelfExcluded
	seven := 80.0

	agree := pending(gh("o/agree", "Go", 9000))
	settle := pending(gh("o/settle", "Go", 8000))
	split := pending(gh("o/split", "Go", 7000))
	override := pending(gh("o/override", "Go", 6000))
	override.Labels.ShelfOverride = "productivity/todo"
	unfit := pending(gh("o/unfit", "Go", 5000))
	unfit.Jobs.Judge.Fail = 2
	// Not in the queue:
	done := gh("o/done", "Go", 4000, "web/frontend")
	done.Jobs.Judge.Status, done.Labels.Version, done.Labels.By = item.StatusDone, 3, item.ByLLM
	behind := gh("o/behind", "Go", 3000, "web/frontend") // done, but on an older tree
	behind.Jobs.Judge.Status, behind.Labels.Version = item.StatusDone, 2
	manual := pending(gh("o/manual", "Go", 2500))
	manual.Labels.By = item.ByManual
	tail := pending(gh("o/tail", "Go", 40))
	hot := pending(gh("o/hot", "Go", 60))
	hot.Growth.D7 = &seven
	stale := pending(gh("o/stale", "Go", 9999))
	stale.Stale = true
	failed := gh("o/failed", "Go", 9998)
	failed.Jobs.Judge.Status = item.StatusFailed
	model := item.Item{Source: item.SourceHuggingFace, ExternalID: "a/model", Topics: []string{}, History: []item.Point{},
		PrimaryMetric: "downloads", Metrics: map[string]float64{"downloads": 1e6, "stars": 1e6}}
	model.Jobs.Judge.Status = item.StatusPending

	for _, it := range []item.Item{agree, settle, split, override, unfit, done, behind, manual, tail, hot, stale, failed, model} {
		if _, err := st.Items().InsertOne(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.Blobs().InsertOne(ctx, item.NewBlob(item.SourceGitHub, "o/agree", "# Agree\n\nA notes app.", nil, time.Now())); err != nil {
		t.Fatal(err)
	}

	full := verdict("app", "productivity/notes", "data/database", "web/frontend")
	full.HasGui, full.TaglineZh, full.TaglineEn, full.Tags = true, "本地笔记", "Local notes", []string{"notes"}
	full.Alternatives = []item.Alternative{{Name: "Notion", Slug: "notion", Kind: "note-taking"}}
	lib := verdict("library", ex, "web/frontend")
	m := newScripted(map[string][]judge.Verdict{
		"o/agree":    {full, verdict("app", "productivity/notes", "data/database")},
		"o/settle":   {lib, verdict("library", ex, "web/backend"), verdict("library", ex, "web/frontend")},
		"o/split":    {lib, verdict("library", ex, "web/backend"), verdict("library", ex, "web/build-tools")},
		"o/override": {full, full},
		"o/unfit":    {},
		"o/behind":   {lib, lib},
		"o/hot":      {verdict("awesome", ex), verdict("awesome", ex)},
	})
	at := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	j := NewJudge(st, m, JudgeConfig{TreeVersion: 3, MaxPerRun: 100, MinStars: 1000, HotWeekly: 50}, nil)
	j.now = func() time.Time { return at }
	rep, err := j.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Processed != 6 || rep.Failed != 1 {
		t.Errorf("report %+v", rep)
	}
	for k, want := range map[string]any{"queued": 7, "agreed": 4, "majority": 1, "split": 1} {
		if rep.Notes[k] != want {
			t.Errorf("note %s = %v, want %v", k, rep.Notes[k], want)
		}
	}
	// One full answer, then verification passes; a third only on disagreement.
	if got := m.fullSeen["o/agree"]; !reflect.DeepEqual(got, []bool{true, false}) {
		t.Errorf("o/agree asked %v", got)
	}
	if got := m.fullSeen["o/settle"]; !reflect.DeepEqual(got, []bool{true, false, false}) {
		t.Errorf("o/settle asked %v", got)
	}
	if m.readme["o/agree"] != "A notes app." {
		t.Errorf("readme excerpt %q", m.readme["o/agree"])
	}
	for _, id := range []string{"o/done", "o/manual", "o/tail", "o/stale", "o/failed", "a/model"} {
		if m.asked[id] != 0 {
			t.Errorf("%s was judged", id)
		}
	}

	got := itemsByID(t, st)
	a := got["o/agree"]
	yes := true
	wantLabels := item.Labels{
		// The shelf's leaf, then the domain leaf both answers named.
		Domain: []string{"apps/productivity/notes", "data/database"}, Type: "app", Shelf: "productivity/notes", HasGui: &yes,
		TaglineZh: "本地笔记", TaglineEn: "Local notes", Alternatives: []item.Alternative{{Name: "Notion", Slug: "notion", Kind: "note-taking"}},
		Tags: []string{"notes"}, By: item.ByLLM, Confidence: judge.Agreed, Version: 3, At: at,
	}
	if !reflect.DeepEqual(a.Labels, wantLabels) {
		t.Errorf("o/agree labels\n got %+v\nwant %+v", a.Labels, wantLabels)
	}
	if a.Jobs.Judge != (item.JobState{Status: item.StatusDone, Version: judge.PromptVersion, At: at}) {
		t.Errorf("o/agree judge state %+v", a.Jobs.Judge)
	}
	if l := got["o/settle"].Labels; l.Confidence != judge.Majority || !reflect.DeepEqual(l.Domain, []string{"web/frontend"}) {
		t.Errorf("o/settle %+v", l)
	}
	if l := got["o/split"].Labels; l.Confidence != judge.Split || !reflect.DeepEqual(l.Domain, []string{"web/frontend"}) {
		t.Errorf("o/split %+v", l)
	}
	// A person's shelf wins over the model's, and the domain follows it.
	if l := got["o/override"].Labels; l.Shelf != "productivity/todo" || l.ShelfOverride != "productivity/todo" ||
		!reflect.DeepEqual(l.Domain, []string{"apps/productivity/todo", "data/database", "web/frontend"}) {
		t.Errorf("o/override %+v", l)
	}
	if l := got["o/behind"].Labels; l.Version != 3 || l.Type != "library" {
		t.Errorf("o/behind %+v", l)
	}
	if l := got["o/hot"].Labels; l.Type != "awesome" || l.Domain == nil || len(l.Domain) != 0 {
		t.Errorf("o/hot %+v", l)
	}
	// The third unusable answer retires the item.
	if s := got["o/unfit"].Jobs.Judge; s.Status != item.StatusFailed || s.Fail != 3 {
		t.Errorf("o/unfit judge state %+v", s)
	}
	if s := got["o/done"]; !reflect.DeepEqual(s.Labels.Domain, []string{"web/frontend"}) || s.Labels.Confidence != "" {
		t.Errorf("o/done was rewritten: %+v", s.Labels)
	}

	// Nothing is left: a second run asks nothing.
	before := m.calls
	if rep, err = j.Run(ctx); err != nil || rep.Processed != 0 || m.calls != before {
		t.Errorf("second run: %+v %v, %d new calls", rep, err, m.calls-before)
	}
}

func TestJudgeModelFailures(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	if _, err := st.Items().InsertOne(ctx, pending(gh("o/a", "Go", 9000))); err != nil {
		t.Fatal(err)
	}
	lib := verdict("library", item.ShelfExcluded, "web/frontend")
	cfg := JudgeConfig{TreeVersion: 3, MaxPerRun: 100, MinStars: 1000, HotWeekly: 50}

	// The verification call fails: one answer is not a verdict, the item stays
	// pending with no failure counted against it.
	m := newScripted(map[string][]judge.Verdict{"o/a": {lib, lib, lib}})
	m.failCall[2], m.failCall[3] = true, true
	rep, err := NewJudge(st, m, cfg, nil).Run(ctx)
	if err != nil || rep.Processed != 0 || rep.Failed != 0 {
		t.Errorf("report %+v, %v", rep, err)
	}
	if s := itemsByID(t, st)["o/a"]; s.Jobs.Judge.Status != item.StatusPending || s.Jobs.Judge.Fail != 0 || s.Labels.Type != "" {
		t.Errorf("item after failed calls: %+v %+v", s.Jobs.Judge, s.Labels)
	}

	// The model is gone: the run stops with an error instead of grinding on.
	m = newScripted(nil)
	m.failCall[1], m.failCall[2], m.failCall[3] = true, true, true
	for _, id := range []string{"o/b", "o/c", "o/d", "o/e", "o/f", "o/g", "o/h", "o/i", "o/j", "o/k", "o/l", "o/m", "o/n", "o/o", "o/p",
		"o/q", "o/r", "o/s", "o/t", "o/u", "o/v", "o/w", "o/x", "o/y", "o/z"} {
		if _, err := st.Items().InsertOne(ctx, pending(gh(id, "Go", 5000))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewJudge(st, m, cfg, nil).Run(ctx); err == nil || m.calls != 3 {
		t.Errorf("run with a dead model: err %v after %d calls", err, m.calls)
	}

	// The next run, with the model back, finishes the job.
	m = newScripted(map[string][]judge.Verdict{"o/a": {lib, lib}})
	if _, err := NewJudge(st, m, JudgeConfig{TreeVersion: 3, MaxPerRun: 1, MinStars: 1000, HotWeekly: 50}, nil).Run(ctx); err != nil {
		t.Fatal(err)
	}
	if s := itemsByID(t, st)["o/a"]; s.Jobs.Judge.Status != item.StatusDone || s.Labels.Confidence != judge.Agreed {
		t.Errorf("item after recovery: %+v %+v", s.Jobs.Judge, s.Labels)
	}
}
