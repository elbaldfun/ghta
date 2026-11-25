package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

// iosNewKeep is how many days of new-apps boards stay: the page reads the
// newest, and a week back is enough to compare by hand.
const iosNewKeep = 14 * 24 * time.Hour

// IOSNew writes the new-apps boards (boards.IOSNew), one per storefront,
// for the newest chart day. It runs right after the ios job and reads only
// what that job left on the items (app.charts), so a rerun is cheap.
type IOSNew struct {
	st        *store.Store
	countries []string
	log       *slog.Logger
	now       func() time.Time
}

func NewIOSNew(st *store.Store, countries []string, log *slog.Logger) *IOSNew {
	if log == nil {
		log = slog.Default()
	}
	return &IOSNew{st: st, countries: countries, log: log, now: time.Now}
}

func (*IOSNew) Name() string { return "ios-new" }

func (j *IOSNew) Run(ctx context.Context) (rep worker.Report, err error) {
	day, err := j.st.LatestChartDay(ctx, item.SourceIOS)
	if err != nil {
		return rep, err
	}
	if day == "" {
		return rep, fmt.Errorf("ios-new: no chart snapshot yet")
	}
	at, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return rep, fmt.Errorf("ios-new: chart day %q: %w", day, err)
	}
	apps, err := j.st.ChartedSince(ctx, item.SourceIOS, day, at.Add(-boards.IOSNewWindow))
	if err != nil {
		return rep, err
	}
	now := j.now()
	written := 0
	for _, country := range j.countries {
		// The places come with each app's lookup. A storefront whose charts
		// all failed, or whose lookups did (throttled), has no app with
		// today's places there: an empty board would hide yesterday's good
		// one, so keep that.
		if n, err := j.st.ChartedOn(ctx, item.SourceIOS, day, country); err != nil {
			return rep, err
		} else if n == 0 {
			j.log.Warn("ios-new: no app carries today's places in this storefront; keeping its previous board", "country", country, "day", day)
			rep.Note(country, "kept")
			continue
		}
		rows := boards.IOSNew(apps, country, day)
		if err := j.st.PutBoard(ctx, boards.IOSNewKey(country, day), rows, now); err != nil {
			return rep, err
		}
		rep.Note(country, len(rows))
		written++
	}
	if written == 0 {
		return rep, fmt.Errorf("ios-new: no storefront has the chart places of %s; kept every previous board", day)
	}
	pruned, err := j.st.DeleteDatedBoards(ctx, boards.IOSNewPrefix, at.Add(-iosNewKeep).Format(time.DateOnly))
	if err != nil {
		j.log.Warn("ios-new: pruning old boards", "err", err)
	}
	rep.Processed = len(apps)
	rep.Note("day", day)
	rep.Note("pruned", pruned)
	return rep, nil
}
