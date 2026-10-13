package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

const (
	devRefreshAfter = 14 * 24 * time.Hour
	devWriteBatch   = 100
)

// UserFetcher is the part of the GitHub adapter DevSync needs.
type UserFetcher interface {
	FetchUser(ctx context.Context, login string) (*github.UserProfile, github.RateInfo, error)
}

// DevSync keeps the developers collection: for every account that owns a
// tracked repository it fetches the GitHub profile and stores it. Profiles
// fetched within the last two weeks are skipped, so the sweep is incremental
// and an interrupted one continues where it stopped.
type DevSync struct {
	st         *store.Store
	gh         UserFetcher
	rateBuffer int
	log        *slog.Logger
	now        func() time.Time
	wait       func(ctx context.Context, d time.Duration)
}

func NewDevSync(st *store.Store, gh UserFetcher, rateBuffer int, log *slog.Logger) *DevSync {
	if log == nil {
		log = slog.Default()
	}
	return &DevSync{st: st, gh: gh, rateBuffer: max(rateBuffer, 0), log: log, now: time.Now, wait: sleepCtx}
}

func (*DevSync) Name() string { return "devsync" }

func (j *DevSync) Run(ctx context.Context) (rep worker.Report, err error) {
	owners, err := j.st.Owners(ctx)
	if err != nil {
		return rep, err
	}
	fresh, err := j.st.FreshLogins(ctx, j.now().Add(-devRefreshAfter))
	if err != nil {
		return rep, err
	}
	skipped, notFound := 0, 0
	defer func() { rep.Note("owners", len(owners)); rep.Note("fresh", skipped); rep.Note("notFound", notFound) }()

	var buf []domain.Developer
	// Profiles already fetched are written even when the run is being shut
	// down: the requests are spent.
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		err := j.st.UpsertDevelopers(context.WithoutCancel(ctx), buf)
		if err == nil {
			rep.Processed += len(buf)
		}
		buf = buf[:0]
		return err
	}

	for _, login := range owners {
		if ctx.Err() != nil {
			break
		}
		if fresh[login] {
			skipped++
			continue
		}
		u, rate, err := j.gh.FetchUser(ctx, login)
		switch {
		case errors.Is(err, github.ErrUserNotFound):
			notFound++
		case err != nil:
			if ctx.Err() == nil {
				rep.Failed++
				j.log.Warn("devsync: fetch failed", "login", login, "err", err)
			}
		default:
			buf = append(buf, domain.Developer{
				Login: u.Login, Type: u.Type, Name: u.Name, Company: u.Company, Blog: u.Blog,
				Location: u.Location, Bio: u.Bio, TwitterUsername: u.TwitterUsername,
				Followers: u.Followers, Following: u.Following, PublicRepos: u.PublicRepos,
				AvatarURL: u.AvatarURL, GHCreatedAt: u.CreatedAt, FetchedAt: j.now().UTC(),
			})
			if len(buf) >= devWriteBatch {
				if err := flush(); err != nil {
					return rep, err
				}
			}
		}
		waitForRate(ctx, rate, j.rateBuffer, j.now, j.wait, j.log)
	}
	if err := flush(); err != nil {
		return rep, err
	}
	return rep, ctx.Err()
}
