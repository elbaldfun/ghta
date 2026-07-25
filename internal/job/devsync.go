package job

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/source/github"
)

const (
	devUpsertBatch    = 100
	devRateWaitMargin = 5 * time.Second
)

// DevSync builds the developer layer: for every distinct repo owner it fetches
// the GitHub account profile (/users/{login}) and upserts it into `developers`.
// It is source-of-truth for the account's self-declared identity (type, twitter,
// company, followers…). The sweep is serial, paces itself to GitHub's hourly
// budget, and is resumable — accounts fetched within refreshTTL are skipped, so
// a crash or a nightly cron just continues where it left off.
type DevSync struct {
	store      *repository.Store
	gh         *github.Adapter
	refreshTTL time.Duration
	rateBuffer int
	log        *slog.Logger
	running    atomic.Bool
}

func NewDevSync(store *repository.Store, gh *github.Adapter, refreshTTL time.Duration, rateBuffer int, log *slog.Logger) *DevSync {
	if refreshTTL <= 0 {
		refreshTTL = 14 * 24 * time.Hour
	}
	if rateBuffer < 0 {
		rateBuffer = 0
	}
	return &DevSync{store: store, gh: gh, refreshTTL: refreshTTL, rateBuffer: rateBuffer, log: log}
}

// Run sweeps every owner once. Safe to call repeatedly (a second concurrent call
// is a no-op) and to interrupt (it stops promptly on ctx cancel, having already
// persisted every account fetched so far).
func (j *DevSync) Run(ctx context.Context) {
	if !j.running.CompareAndSwap(false, true) {
		j.log.Info("devsync: already running, skipping")
		return
	}
	defer j.running.Store(false)

	owners, err := j.store.DistinctOwners(ctx)
	if err != nil {
		j.log.Error("devsync: list owners failed", "err", err)
		return
	}
	fresh, err := j.store.FreshLogins(ctx, time.Now().Add(-j.refreshTTL))
	if err != nil {
		j.log.Error("devsync: fresh logins failed", "err", err)
		return
	}
	j.log.Info("devsync starting", "owners", len(owners), "alreadyFresh", len(fresh))

	var buf []domain.Developer
	var fetched, skipped, notFound, failed int
	flush := func() {
		if len(buf) == 0 {
			return
		}
		if _, err := j.store.UpsertDevelopers(ctx, buf); err != nil {
			j.log.Warn("devsync: upsert batch failed", "size", len(buf), "err", err)
		}
		buf = buf[:0]
	}

	for _, login := range owners {
		if ctx.Err() != nil {
			break
		}
		if _, ok := fresh[login]; ok {
			skipped++
			continue
		}

		u, rate, err := j.gh.FetchUser(ctx, login)
		if errors.Is(err, github.ErrUserNotFound) {
			notFound++
			continue
		}
		if err != nil {
			failed++
			j.log.Warn("devsync: fetch failed", "login", login, "err", err)
			j.respectRate(ctx, rate) // a 403 here means we're out of budget
			continue
		}

		buf = append(buf, toDeveloper(u))
		fetched++
		if len(buf) >= devUpsertBatch {
			flush()
		}
		j.respectRate(ctx, rate)
	}
	flush()
	j.log.Info("devsync done", "owners", len(owners), "fetched", fetched,
		"skipped", skipped, "notFound", notFound, "failed", failed)
}

// respectRate blocks until the hourly window resets when the remaining budget
// has dropped to the buffer, so the sweep never trips a hard 403.
func (j *DevSync) respectRate(ctx context.Context, rate github.RateInfo) {
	if rate.Remaining < 0 || rate.Remaining > j.rateBuffer || rate.Reset.IsZero() {
		return
	}
	wait := time.Until(rate.Reset) + devRateWaitMargin
	if wait <= 0 {
		return
	}
	j.log.Info("devsync: rate budget low, waiting for reset", "remaining", rate.Remaining, "wait", wait.String())
	select {
	case <-ctx.Done():
	case <-time.After(wait):
	}
}

func toDeveloper(u *github.UserProfile) domain.Developer {
	return domain.Developer{
		Login:           u.Login,
		Type:            u.Type,
		Name:            u.Name,
		Company:         u.Company,
		Blog:            u.Blog,
		Location:        u.Location,
		Bio:             u.Bio,
		TwitterUsername: u.TwitterUsername,
		Followers:       u.Followers,
		Following:       u.Following,
		PublicRepos:     u.PublicRepos,
		AvatarURL:       u.AvatarURL,
		GHCreatedAt:     u.CreatedAt,
		FetchedAt:       time.Now().UTC(),
	}
}
