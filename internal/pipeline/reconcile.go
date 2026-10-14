package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

const (
	reconcileStaleAfter = 7 * 24 * time.Hour // untouched by the fetcher for a week
	reconcileMaxPerRun  = 300                // REST budget per nightly run
	rateMargin          = 5 * time.Second
)

// RepoChecker is the part of the GitHub adapter Reconcile needs.
type RepoChecker interface {
	FetchRepo(ctx context.Context, fullName string) (*github.RepoInfo, github.RateInfo, error)
}

// Reconcile hunts rename ghosts. The fetcher finds repositories through
// search shards, so one that was renamed or deleted just stops appearing and
// its record freezes with its leaderboard slot. Each night the
// longest-untouched records are checked against the REST API:
//
//   - 404                       → tombstone, reason "gone"
//   - taken down by GitHub      → tombstone, reason "blocked"
//   - 200 under another name    → move the record, or tombstone it when the
//     new name is already tracked
//   - 200 under the same name   → it fell below the fetch cutoff; record a
//     fresh reading so its metrics and growth stay honest
type Reconcile struct {
	st         *store.Store
	gh         RepoChecker
	rateBuffer int
	log        *slog.Logger
	now        func() time.Time
	wait       func(ctx context.Context, d time.Duration)
}

func NewReconcile(st *store.Store, gh RepoChecker, rateBuffer int, log *slog.Logger) *Reconcile {
	if log == nil {
		log = slog.Default()
	}
	return &Reconcile{st: st, gh: gh, rateBuffer: max(rateBuffer, 0), log: log, now: time.Now, wait: sleepCtx}
}

func (*Reconcile) Name() string { return "reconcile" }

func (j *Reconcile) Run(ctx context.Context) (rep worker.Report, err error) {
	const src = item.SourceGitHub
	// Named results: the deferred notes must land in the returned report.
	ids, err := j.st.StaleCandidates(ctx, src, j.now().Add(-reconcileStaleAfter), reconcileMaxPerRun)
	if err != nil {
		return rep, err
	}
	tally := map[string]int{}
	defer func() {
		for k, v := range tally {
			rep.Note(k, v)
		}
	}()

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		info, rate, err := j.gh.FetchRepo(ctx, id)
		verdict, err := j.settle(ctx, id, info, err)
		if err != nil {
			if ctx.Err() != nil {
				return rep, ctx.Err()
			}
			rep.Failed++
			j.log.Warn("reconcile: check failed", "repo", id, "err", err)
		} else {
			rep.Processed++
			tally[verdict]++
		}
		waitForRate(ctx, rate, j.rateBuffer, j.now, j.wait, j.log)
	}
	return rep, nil
}

// settle applies the verdict for one repository and names it.
func (j *Reconcile) settle(ctx context.Context, id string, info *github.RepoInfo, fetchErr error) (string, error) {
	const src = item.SourceGitHub
	now := j.now()
	switch {
	case errors.Is(fetchErr, github.ErrRepoNotFound):
		return "gone", j.st.MarkStale(ctx, src, id, "gone", now)
	case errors.Is(fetchErr, github.ErrRepoBlocked):
		return "blocked", j.st.MarkStale(ctx, src, id, "blocked", now)
	case fetchErr != nil:
		return "", fetchErr
	}

	metrics := map[string]float64{
		"stars": float64(info.Stars), "forks": float64(info.Forks), "openIssues": float64(info.OpenIssues),
	}
	if strings.EqualFold(info.FullName, id) {
		return "refreshed", j.st.RefreshMetrics(ctx, src, id, metrics, now)
	}

	// Renamed upstream. When the new name is already tracked — the fetcher
	// found it again — this record is a duplicate ghost. Otherwise the record
	// moves, history and all.
	owner, name, _ := strings.Cut(info.FullName, "/")
	err := j.st.Rename(ctx, src, id, info.FullName, owner, name, now)
	if errors.Is(err, store.ErrTaken) {
		return "merged", j.st.MarkStale(ctx, src, id, "renamed:"+info.FullName, now)
	}
	if err != nil {
		return "", err
	}
	j.log.Info("reconcile: renamed", "old", id, "new", info.FullName)
	return "renamed", j.st.RefreshMetrics(ctx, src, info.FullName, metrics, now)
}

// waitForRate blocks until GitHub's hourly window resets once the remaining
// budget has dropped to the buffer, so a sweep never runs into a hard 403.
func waitForRate(ctx context.Context, rate github.RateInfo, buffer int, now func() time.Time,
	wait func(context.Context, time.Duration), log *slog.Logger) {
	if rate.Remaining < 0 || rate.Remaining > buffer || rate.Reset.IsZero() {
		return
	}
	d := rate.Reset.Sub(now()) + rateMargin
	if d <= 0 {
		return
	}
	log.Info("github rate budget low, waiting for reset", "remaining", rate.Remaining, "wait", d.String())
	wait(ctx, d)
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
