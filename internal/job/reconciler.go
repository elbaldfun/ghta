package job

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/source/github"
)

const (
	reconcileStaleAfter = 7 * 24 * time.Hour // untouched by the fetcher for a week = suspicious
	reconcileMaxPerRun  = 300                // REST budget cap per nightly run
	reconcileRateMargin = 5 * time.Second
)

// repoChecker is the slice of the GitHub adapter the reconciler needs
// (interface so tests can fake upstream answers).
type repoChecker interface {
	FetchRepo(ctx context.Context, fullName string) (*github.RepoInfo, github.RateInfo, error)
}

// Reconciler hunts rename ghosts. The fetcher discovers repos through search
// shards, so a renamed or deleted repo simply stops appearing: its record
// freezes at the last fetched values and keeps its leaderboard slot forever
// (facebook/react sat 17 months stale next to its live successor react/react).
// Nightly, the oldest-untouched records are checked against the REST API:
//
//   - 404                        -> tombstone (stale=true, reason "gone")
//   - 200 under a new full_name  -> rename: move the record, or tombstone it
//     if the new name is already tracked
//   - 200 under the same name    -> repo just fell below the fetch cutoff;
//     refresh its metrics so it stays honest
type Reconciler struct {
	store      *repository.Store
	gh         repoChecker
	staleAfter time.Duration
	maxPerRun  int
	rateBuffer int
	log        *slog.Logger
	running    atomic.Bool
}

func NewReconciler(store *repository.Store, gh repoChecker, rateBuffer int, log *slog.Logger) *Reconciler {
	if rateBuffer < 0 {
		rateBuffer = 0
	}
	return &Reconciler{
		store:      store,
		gh:         gh,
		staleAfter: reconcileStaleAfter,
		maxPerRun:  reconcileMaxPerRun,
		rateBuffer: rateBuffer,
		log:        log,
	}
}

// Run checks one batch of stale candidates. Safe to call repeatedly (a second
// concurrent call is a no-op); every verdict is persisted immediately, so an
// interrupted run just resumes with the next-oldest records tomorrow.
func (j *Reconciler) Run(ctx context.Context) {
	if !j.running.CompareAndSwap(false, true) {
		j.log.Info("reconciler: already running, skipping")
		return
	}
	defer j.running.Store(false)

	cutoff := time.Now().UTC().Add(-j.staleAfter)
	items, err := j.store.StaleCandidates(ctx, domain.SourceGitHub, cutoff, j.maxPerRun)
	if err != nil {
		j.log.Error("reconciler: list candidates failed", "err", err)
		return
	}
	if len(items) == 0 {
		j.log.Info("reconciler: no stale candidates")
		return
	}
	j.log.Info("reconciler starting", "candidates", len(items), "cutoff", cutoff.Format(time.DateOnly))

	var gone, renamed, merged, refreshed, failed int
	for _, it := range items {
		if ctx.Err() != nil {
			break
		}

		info, rate, err := j.gh.FetchRepo(ctx, it.ExternalID)
		switch {
		case errors.Is(err, github.ErrRepoNotFound):
			if err := j.store.MarkItemStale(ctx, domain.SourceGitHub, it.ExternalID, "gone"); err != nil {
				j.log.Warn("reconciler: tombstone failed", "repo", it.ExternalID, "err", err)
			} else {
				gone++
			}
		case err != nil:
			failed++
			j.log.Warn("reconciler: check failed", "repo", it.ExternalID, "err", err)
			j.respectRate(ctx, rate) // a 403 here means the budget is spent
		case !strings.EqualFold(info.FullName, it.ExternalID):
			// Renamed upstream. If the new name is already tracked (the fetcher
			// re-discovered it), this record is a ghost duplicate — tombstone it.
			// Otherwise carry the record (and its history) over to the new name.
			exists, exErr := j.store.ItemExists(ctx, domain.SourceGitHub, info.FullName)
			if exErr != nil {
				failed++
				j.log.Warn("reconciler: exists check failed", "repo", info.FullName, "err", exErr)
				break
			}
			if exists {
				if err := j.store.MarkItemStale(ctx, domain.SourceGitHub, it.ExternalID, "renamed:"+info.FullName); err != nil {
					j.log.Warn("reconciler: tombstone failed", "repo", it.ExternalID, "err", err)
				} else {
					merged++
					j.log.Info("reconciler: ghost merged", "old", it.ExternalID, "new", info.FullName)
				}
				break
			}
			name := info.FullName
			if i := strings.IndexByte(name, '/'); i >= 0 {
				name = name[i+1:]
			}
			if err := j.store.RenameItem(ctx, domain.SourceGitHub, it.ExternalID, info.FullName, name, repoMetrics(info)); err != nil {
				// Lost the race with the fetcher inserting the new name — the
				// unique (source, externalId) index rejects the move; tombstone.
				if err2 := j.store.MarkItemStale(ctx, domain.SourceGitHub, it.ExternalID, "renamed:"+info.FullName); err2 != nil {
					j.log.Warn("reconciler: rename+tombstone failed", "repo", it.ExternalID, "err", err2)
				} else {
					merged++
				}
			} else {
				renamed++
				j.log.Info("reconciler: renamed", "old", it.ExternalID, "new", info.FullName)
			}
		default:
			// Same name, still alive — it just fell out of the fetch shards.
			if err := j.store.RefreshItemMetrics(ctx, domain.SourceGitHub, it.ExternalID, repoMetrics(info)); err != nil {
				j.log.Warn("reconciler: refresh failed", "repo", it.ExternalID, "err", err)
			} else {
				refreshed++
			}
		}
		j.respectRate(ctx, rate)
	}
	j.log.Info("reconciler done", "candidates", len(items),
		"gone", gone, "renamed", renamed, "merged", merged, "refreshed", refreshed, "failed", failed)
}

func repoMetrics(info *github.RepoInfo) map[string]float64 {
	return map[string]float64{
		"stars":      float64(info.Stars),
		"forks":      float64(info.Forks),
		"openIssues": float64(info.OpenIssues),
	}
}

// respectRate blocks until the hourly window resets when the remaining budget
// has dropped to the buffer, so the sweep never trips a hard 403.
func (j *Reconciler) respectRate(ctx context.Context, rate github.RateInfo) {
	if rate.Remaining < 0 || rate.Remaining > j.rateBuffer || rate.Reset.IsZero() {
		return
	}
	wait := time.Until(rate.Reset) + reconcileRateMargin
	if wait <= 0 {
		return
	}
	j.log.Info("reconciler: rate budget low, waiting for reset", "remaining", rate.Remaining, "wait", wait.String())
	select {
	case <-ctx.Done():
	case <-time.After(wait):
	}
}
