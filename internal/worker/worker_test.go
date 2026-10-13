package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/elbaldfun/ghta/internal/store"
)

type fakeRecorder struct {
	mu       sync.Mutex
	started  []string
	finished map[primitive.ObjectID]store.JobRun
	failAll  bool
}

func (f *fakeRecorder) StartRun(_ context.Context, job string, _ time.Time) (primitive.ObjectID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAll {
		return primitive.NilObjectID, errors.New("db down")
	}
	f.started = append(f.started, job)
	return primitive.NewObjectID(), nil
}

func (f *fakeRecorder) FinishRun(ctx context.Context, id primitive.ObjectID, end store.JobRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if f.finished == nil {
		f.finished = map[primitive.ObjectID]store.JobRun{}
	}
	f.finished[id] = end
	return nil
}

func (f *fakeRecorder) statuses() map[string]int {
	out := map[string]int{}
	for _, r := range f.finished {
		out[r.Status]++
	}
	return out
}

type fn struct {
	name string
	run  func(ctx context.Context) (Report, error)
}

func (f fn) Name() string                            { return f.name }
func (f fn) Run(ctx context.Context) (Report, error) { return f.run(ctx) }

func TestRunnerRunsInOrderAndRecords(t *testing.T) {
	rec := &fakeRecorder{}
	var order []string
	job := func(name string, err error) Job {
		return fn{name, func(context.Context) (Report, error) {
			order = append(order, name)
			rep := Report{Processed: 3, Failed: 1}
			rep.Note("k", name)
			return rep, err
		}}
	}
	res := NewRunner(rec, nil).Run(context.Background(),
		job("a", nil), job("b", errors.New("boom")), job("c", nil))

	if got := strings.Join(order, ""); got != "abc" {
		t.Fatalf("order = %q; a failing job must not stop the pipeline", got)
	}
	if len(res) != 3 || res[0].Status != store.RunDone || res[1].Status != store.RunFailed || res[2].Status != store.RunDone {
		t.Fatalf("results = %+v", res)
	}
	if got := rec.statuses(); got[store.RunDone] != 2 || got[store.RunFailed] != 1 {
		t.Fatalf("recorded statuses = %v", got)
	}
	for _, r := range rec.finished {
		if r.Processed != 3 || r.Failed != 1 || r.Notes["k"] == nil || r.FinishedAt.IsZero() {
			t.Fatalf("recorded run = %+v", r)
		}
		if (r.Status == store.RunFailed) != (r.Error == "boom") {
			t.Fatalf("error not recorded with the failed run: %+v", r)
		}
	}
}

func TestRunnerSurvivesPanic(t *testing.T) {
	rec := &fakeRecorder{}
	ran := false
	res := NewRunner(rec, nil).Run(context.Background(),
		fn{"bad", func(context.Context) (Report, error) { panic("nil map") }},
		fn{"next", func(context.Context) (Report, error) { ran = true; return Report{}, nil }})
	if !ran || res[0].Status != store.RunFailed || !strings.Contains(res[0].Err.Error(), "nil map") {
		t.Fatalf("panic not contained: ran=%v res=%+v", ran, res)
	}
}

func TestRunnerStopsOnCancelAndRecordsInterrupted(t *testing.T) {
	rec := &fakeRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	ran := false
	res := NewRunner(rec, nil).Run(ctx,
		fn{"long", func(ctx context.Context) (Report, error) {
			cancel()
			<-ctx.Done()
			return Report{Processed: 7}, ctx.Err()
		}},
		fn{"never", func(context.Context) (Report, error) { ran = true; return Report{}, nil }})

	if ran || len(res) != 1 || res[0].Status != store.RunInterrupted {
		t.Fatalf("ran=%v res=%+v", ran, res)
	}
	// The record is written even though the run's context is cancelled.
	if got := rec.statuses(); got[store.RunInterrupted] != 1 {
		t.Fatalf("recorded statuses = %v", got)
	}
}

func TestRunnerRunsJobWhenRecordingFails(t *testing.T) {
	ran := false
	res := NewRunner(&fakeRecorder{failAll: true}, nil).Run(context.Background(),
		fn{"a", func(context.Context) (Report, error) { ran = true; return Report{}, nil }})
	if !ran || res[0].Status != store.RunDone {
		t.Fatalf("a job must run even when its record cannot be written: %+v", res)
	}
}

func TestRunnerSerialisesPipelines(t *testing.T) {
	r := NewRunner(&fakeRecorder{}, nil)
	var mu sync.Mutex
	active, peak := 0, 0
	job := fn{"j", func(context.Context) (Report, error) {
		mu.Lock()
		active++
		peak = max(peak, active)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return Report{}, nil
	}}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { r.Run(context.Background(), job, job) })
	}
	wg.Wait()
	if peak != 1 {
		t.Fatalf("%d jobs ran at once", peak)
	}
}
