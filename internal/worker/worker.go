// Package worker runs the scheduled jobs of the rebuilt system
// (docs/REDESIGN.md §5.3): one job at a time, in a fixed order, every run
// recorded in job_runs.
//
// The host has one CPU, so jobs running side by side only slow each other
// down. A job that needs concurrency (LLM batches) manages it internally.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/elbaldfun/ghta/internal/store"
)

// Report is what one run of a job did.
type Report struct {
	Processed int            // units of work completed (items, shards, boards)
	Failed    int            // units that failed without failing the run
	Notes     map[string]any // job-specific counters, stored with the run
}

// Note sets one entry of Notes.
func (r *Report) Note(key string, v any) {
	if r.Notes == nil {
		r.Notes = map[string]any{}
	}
	r.Notes[key] = v
}

// Job is one scheduled unit of work. Run must honour ctx: the worker cancels
// it on shutdown. A returned error fails the run; per-item failures belong in
// Report.Failed.
type Job interface {
	Name() string
	Run(ctx context.Context) (Report, error)
}

// Recorder persists run records. *store.Store implements it.
type Recorder interface {
	StartRun(ctx context.Context, job string, at time.Time) (primitive.ObjectID, error)
	FinishRun(ctx context.Context, id primitive.ObjectID, end store.JobRun) error
}

// Result is the outcome of one job within a pipeline run.
type Result struct {
	Job      string
	Status   string
	Report   Report
	Err      error
	Duration time.Duration
}

// Runner executes jobs strictly one at a time.
type Runner struct {
	rec Recorder
	log *slog.Logger
	mu  sync.Mutex
	now func() time.Time
}

func NewRunner(rec Recorder, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{rec: rec, log: log, now: time.Now}
}

// Run executes jobs in order and returns one Result per job started. Calls
// are serialised: a second Run waits for the first to finish.
//
// A failing job does not stop the pipeline — the jobs after it work on
// whatever data is there. Cancelling ctx does: the running job is recorded as
// interrupted and the rest are not started.
func (r *Runner) Run(ctx context.Context, jobs ...Job) []Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []Result
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		out = append(out, r.runOne(ctx, j))
	}
	return out
}

func (r *Runner) runOne(ctx context.Context, j Job) Result {
	started := r.now()
	// Bookkeeping must outlive a shutdown, or an interrupted run would stay
	// "running" forever.
	bookCtx := context.WithoutCancel(ctx)

	id, recErr := r.rec.StartRun(bookCtx, j.Name(), started)
	if recErr != nil {
		r.log.Warn("worker: run not recorded", "job", j.Name(), "err", recErr)
	}
	r.log.Info("worker: job started", "job", j.Name())

	rep, err := safeRun(ctx, j)
	res := Result{Job: j.Name(), Report: rep, Err: err, Duration: r.now().Sub(started)}
	switch {
	case err == nil:
		res.Status = store.RunDone
	case ctx.Err() != nil:
		res.Status = store.RunInterrupted
	default:
		res.Status = store.RunFailed
	}

	if recErr == nil {
		end := store.JobRun{
			Status: res.Status, FinishedAt: started.Add(res.Duration),
			Processed: rep.Processed, Failed: rep.Failed, Notes: rep.Notes,
		}
		if err != nil {
			end.Error = err.Error()
		}
		if e := r.rec.FinishRun(bookCtx, id, end); e != nil {
			r.log.Warn("worker: run result not recorded", "job", j.Name(), "err", e)
		}
	}

	attrs := []any{"job", j.Name(), "status", res.Status, "processed", rep.Processed,
		"failed", rep.Failed, "seconds", res.Duration.Round(time.Millisecond).Seconds()}
	if err != nil {
		r.log.Error("worker: job finished", append(attrs, "err", err)...)
	} else {
		r.log.Info("worker: job finished", attrs...)
	}
	return res
}

// safeRun turns a panic in a job into a failed run, so one bad job cannot take
// the worker — and the jobs queued behind it — down.
func safeRun(ctx context.Context, j Job) (rep Report, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v\n%s", p, debug.Stack())
		}
	}()
	return j.Run(ctx)
}
