package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/appstore"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

// AppStore is what the ios job needs of Apple: *appstore.Client.
type AppStore interface {
	Chart(ctx context.Context, country, chart, genre string) ([]appstore.ChartEntry, error)
	Lookup(ctx context.Context, country string, ids []string) ([]appstore.App, error)
}

// IOS reads today's App Store charts and the listings of the apps on them
// (docs/MOBILE-APPS-DATA.md). It runs on its own schedule after the nightly
// pipeline, not inside it.
//
// Every chart of every country is read first and stored in chart_snapshots;
// then each app found is looked up once, in its home storefront, and written
// to items. The home storefront is fixed the first day an app is seen — the
// first configured country that charted it — so its rating history never
// mixes two countries. An app that left every chart is not looked up; its
// growth ages like a GitHub item's that fell out of range and its app.charts
// is removed, but only on a day every chart and lookup succeeded: an app
// missed because a request failed has not stopped growing or left the charts.
type IOS struct {
	st        *store.Store
	client    AppStore
	countries []string
	genres    []string // appstore.GenreAll first, then genre ids
	shelve    appstore.Shelver
	log       *slog.Logger
	now       func() time.Time
}

// NewIOS returns the job for the given storefronts, in priority order.
func NewIOS(st *store.Store, client AppStore, countries []string, shelve appstore.Shelver, log *slog.Logger) *IOS {
	if log == nil {
		log = slog.Default()
	}
	genres := []string{appstore.GenreAll}
	for _, g := range appstore.Genres {
		genres = append(genres, g.ID)
	}
	return &IOS{st: st, client: client, countries: countries, genres: genres, shelve: shelve, log: log, now: time.Now}
}

func (*IOS) Name() string { return "ios" }

// seenApp is what today's charts say about one app.
type seenApp struct {
	first     string          // first configured country that charted it today
	countries []string        // countries that charted it today, in order
	rank      map[string]int  // best rank today, by country
	charts    []item.ChartPos // every chart it is on today, in fetch order
}

func (j *IOS) Run(ctx context.Context) (rep worker.Report, err error) {
	now := j.now().UTC()
	day := item.DayOf(now)
	var charts, missing, failed, looked, gone int
	defer func() {
		rep.Note("charts", charts)
		rep.Note("missingCharts", missing)
		rep.Note("failedCharts", failed)
		rep.Note("lookedUp", looked)
		rep.Note("notListed", gone)
	}()

	seen := map[string]*seenApp{}
	var order []string // first-seen order, so lookups batch deterministically
	throttled := false
charts:
	for _, country := range j.countries {
		for _, chart := range appstore.Charts {
			for _, genre := range j.genres {
				if err := ctx.Err(); err != nil {
					return rep, err
				}
				entries, err := j.client.Chart(ctx, country, chart, genre)
				switch {
				case errors.Is(err, appstore.ErrNoChart):
					missing++
					continue
				case errors.Is(err, appstore.ErrThrottled):
					j.log.Error("ios: throttled reading charts; stopping", "country", country, "chart", chart, "genre", genre, "err", err)
					failed++
					throttled = true
					break charts
				case err != nil:
					if ctx.Err() != nil {
						return rep, ctx.Err()
					}
					j.log.Warn("ios: chart failed", "country", country, "chart", chart, "genre", genre, "err", err)
					failed++
					continue
				}
				// The rank each app had on this chart's previous snapshot, for
				// app.charts: the chart page compares the same two snapshots.
				prev, ok, err := j.st.PrevChart(ctx, item.SourceIOS, country, chart, genre, day)
				if err != nil {
					return rep, err
				}
				prevRank := map[string]int{}
				if ok {
					for i, id := range prev.Entries {
						prevRank[id] = i + 1
					}
				}
				ids := make([]string, len(entries))
				for i, e := range entries {
					ids[i] = e.ID
					a := seen[e.ID]
					if a == nil {
						a = &seenApp{first: country, rank: map[string]int{}}
						seen[e.ID] = a
						order = append(order, e.ID)
					}
					if n := len(a.countries); n == 0 || a.countries[n-1] != country {
						a.countries = append(a.countries, country)
					}
					if r, ok := a.rank[country]; !ok || e.Rank < r {
						a.rank[country] = e.Rank
					}
					a.charts = append(a.charts, item.ChartPos{
						Country: country, Chart: chart, Genre: genre, Rank: e.Rank, Prev: prevRank[e.ID],
					})
				}
				if err := j.st.WriteChart(ctx, store.ChartSnapshot{
					Source: item.SourceIOS, Country: country, Chart: chart, Genre: genre,
					Day: day, FetchedAt: now, Entries: ids,
				}); err != nil {
					return rep, err
				}
				charts++
			}
		}
	}
	if charts == 0 {
		// Nothing read: leave every item's metrics and growth as they were.
		return rep, fmt.Errorf("ios: no chart could be read (%d failed, %d missing)", failed, missing)
	}

	// One lookup per batch of apps sharing a home storefront: the stored one,
	// or for an app new today the first country that charted it.
	stored, err := j.st.HomeCountries(ctx, item.SourceIOS, order)
	if err != nil {
		return rep, err
	}
	home := make(map[string]string, len(order))
	byHome := map[string][]string{}
	homes := slices.Clone(j.countries)
	for _, id := range order {
		h := stored[id]
		if h == "" {
			h = seen[id].first
		}
		home[id] = h
		if !slices.Contains(homes, h) {
			homes = append(homes, h) // a storefront dropped from IOS_COUNTRIES
		}
		byHome[h] = append(byHome[h], id)
	}
lookups:
	for _, country := range homes {
		ids := byHome[country]
		for start := 0; start < len(ids); start += appstore.LookupBatch {
			if err := ctx.Err(); err != nil {
				return rep, err
			}
			if throttled {
				break lookups
			}
			batch := ids[start:min(start+appstore.LookupBatch, len(ids))]
			apps, err := j.client.Lookup(ctx, country, batch)
			if err != nil {
				if ctx.Err() != nil {
					return rep, ctx.Err()
				}
				j.log.Warn("ios: lookup failed", "country", country, "apps", len(batch), "err", err)
				rep.Failed += len(batch)
				throttled = errors.Is(err, appstore.ErrThrottled)
				continue
			}
			fetched := make([]item.Fetched, 0, len(apps))
			for _, a := range apps {
				id := strconv.FormatInt(a.TrackID, 10)
				s := seen[id]
				if s == nil || home[id] != country {
					continue // Apple answered for an id we did not ask about
				}
				fetched = append(fetched, appstore.ToFetched(a, appstore.Listing{
					HomeCountry: country, Countries: s.countries, BestRank: s.rank[country],
					Charts: s.charts, Day: day,
				}, j.shelve))
			}
			gone += len(batch) - len(fetched)
			if _, err := j.st.WriteFetched(ctx, fetched, now); err != nil {
				return rep, err
			}
			looked += len(fetched)
		}
	}
	rep.Processed = looked
	switch {
	case throttled:
		return rep, fmt.Errorf("ios: throttled by Apple; %d charts and %d listings written, growth not aged", charts, looked)
	case looked == 0:
		return rep, fmt.Errorf("ios: %d charts read but no listing could be looked up", charts)
	case rep.Failed > 0 || failed > 0:
		// A chart or a listing was not read: an app missing today may only
		// be missing from what failed, so growth is left as it was (as fetch
		// does, #61).
		j.log.Warn("ios: some requests failed; growth not aged", "charts", failed, "apps", rep.Failed)
		return rep, nil
	}
	if _, err := j.st.AgeGrowth(ctx, item.SourceIOS, day); err != nil {
		return rep, err
	}
	// Every chart was read, so an app whose charts are older than today is
	// on none of them now.
	cleared, err := j.st.ClearCharts(ctx, item.SourceIOS, day)
	if err != nil {
		return rep, err
	}
	rep.Note("chartsCleared", cleared)
	return rep, nil
}
