package boards

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/appstore"
)

// The App Store "new" board: apps first released within IOSNewWindow,
// one board per storefront, ordered by the app's best place on any of that
// storefront's charts that day. It is StarRank's view, not an Apple chart:
// it merges free, paid and grossing, overall and per genre.
//
// Later (once a week of snapshots is in, 2026-12-01): a second order by
// rank momentum, the same measure as the "rising" board. The row keeps the
// best place so that order can be added without changing the key.

// IOSNewWindow is how recent a first release must be.
const IOSNewWindow = 90 * 24 * time.Hour

// IOSNewPrefix starts every new-apps board key: "ios-new|us|2026-11-25".
const IOSNewPrefix = "ios-new|"

// IOSNewKey is a storefront's new-apps board for one chart day.
func IOSNewKey(country, day string) string { return IOSNewPrefix + country + "|" + day }

// IOSNewRow is one app on a new-apps board.
type IOSNewRow struct {
	ID         string    `bson:"id" json:"id"`
	ReleasedAt time.Time `bson:"releasedAt" json:"releasedAt"`
	// Best is the app's best place in the storefront that day.
	Best item.ChartPos `bson:"best" json:"best"`
	// Charts is how many of the storefront's charts the app is on that day.
	Charts int `bson:"charts" json:"charts"`
}

// chartOrder breaks a tie between two places of the same rank, for the
// place a row shows: an overall chart before a genre chart (a place among
// every app is the harder one), then free, paid, grossing, then genre id.
func chartOrder(a, b item.ChartPos) bool {
	if (a.Genre == appstore.GenreAll) != (b.Genre == appstore.GenreAll) {
		return a.Genre == appstore.GenreAll
	}
	if a.Chart != b.Chart {
		return chartIndex(a.Chart) < chartIndex(b.Chart)
	}
	return a.Genre < b.Genre
}

func chartIndex(c string) int {
	if i := slices.Index(appstore.Charts, c); i >= 0 {
		return i
	}
	return len(appstore.Charts)
}

// IOSNew builds a storefront's board from the apps charted on day. An app
// counts when its first release is within IOSNewWindow of day and its chart
// places are that day's.
//
// Order: best rank; on a tie, the app with the overall-chart place first,
// then the one on more of the storefront's charts (a wider reach), then the
// newer release (this board is about new apps), then the track id, so a
// rerun gives the same order.
func IOSNew(apps []item.Item, country, day string) []IOSNewRow {
	at, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return []IOSNewRow{}
	}
	since := at.Add(-IOSNewWindow)
	rows := []IOSNewRow{}
	for i := range apps {
		s := apps[i].App
		if s == nil || apps[i].Stale || s.ChartsDay != day || s.ReleasedAt.IsZero() || s.ReleasedAt.Before(since) {
			continue
		}
		var best item.ChartPos
		n := 0
		for _, p := range s.Charts {
			if p.Country != country || p.Rank <= 0 {
				continue
			}
			n++
			if best.Rank == 0 || p.Rank < best.Rank || (p.Rank == best.Rank && chartOrder(p, best)) {
				best = p
			}
		}
		if n == 0 {
			continue
		}
		best.Prev = 0 // a move belongs to one chart; the row does not show it
		rows = append(rows, IOSNewRow{ID: apps[i].ExternalID, ReleasedAt: s.ReleasedAt, Best: best, Charts: n})
	}
	sort.Slice(rows, func(a, b int) bool {
		x, y := rows[a], rows[b]
		if x.Best.Rank != y.Best.Rank {
			return x.Best.Rank < y.Best.Rank
		}
		if xa, ya := x.Best.Genre == appstore.GenreAll, y.Best.Genre == appstore.GenreAll; xa != ya {
			return xa
		}
		if x.Charts != y.Charts {
			return x.Charts > y.Charts
		}
		if !x.ReleasedAt.Equal(y.ReleasedAt) {
			return x.ReleasedAt.After(y.ReleasedAt)
		}
		return strings.Compare(x.ID, y.ID) < 0
	})
	return rows
}
