package item

import (
	"math"
	"time"
)

const dayLayout = "2006-01-02"

// DayOf is the UTC date a History point for t belongs to.
func DayOf(t time.Time) string { return t.UTC().Format(dayLayout) }

// daysBefore returns the date n days before day, or "" if day is malformed.
func daysBefore(day string, n int) string {
	t, err := time.Parse(dayLayout, day)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -n).Format(dayLayout)
}

// daysBetween is the number of days from a to b (both UTC dates), or 0 if
// either is malformed.
func daysBetween(a, b string) int {
	ta, err1 := time.Parse(dayLayout, a)
	tb, err2 := time.Parse(dayLayout, b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(tb.Sub(ta).Hours() / 24)
}

// ComputeGrowth measures current against the history ring as of asOf (a UTC
// date). The baseline of an n-day window is the latest point dated n or more
// days before asOf; with no such point the window is nil — the item is younger
// than the window, which is different from not having grown.
//
// When the baseline is older than n days — the item is fetched every other
// day, or a fetch was missed — the increase is prorated to n days and rounded,
// so a window always reads as n days of growth: a repo seen every two days
// reports half its two-day gain as its daily growth, not all of it.
//
// history must be ascending by date, as SetPoint keeps it.
func ComputeGrowth(history []Point, current float64, asOf string) Growth {
	window := func(n int) *float64 {
		cut := daysBefore(asOf, n)
		if cut == "" {
			return nil
		}
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].D <= cut {
				inc := current - history[i].V
				if span := daysBetween(history[i].D, asOf); span > n {
					inc = math.Round(inc * float64(n) / float64(span))
				}
				return &inc
			}
		}
		return nil
	}
	return Growth{D1: window(1), D7: window(7), D30: window(30)}
}

// Equal reports whether g and o hold the same windows.
func (g Growth) Equal(o Growth) bool {
	eq := func(a, b *float64) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
	return eq(g.D1, o.D1) && eq(g.D7, o.D7) && eq(g.D30, o.D30)
}

// SetPoint records p as its day's closing value, keeping history ascending and
// at most HistoryDays long. A second point for the same day replaces the first.
func SetPoint(history []Point, p Point) []Point {
	i := len(history)
	for i > 0 && history[i-1].D > p.D {
		i--
	}
	if i > 0 && history[i-1].D == p.D {
		history[i-1] = p
	} else {
		history = append(history, Point{})
		copy(history[i+1:], history[i:])
		history[i] = p
	}
	if len(history) > HistoryDays {
		history = history[len(history)-HistoryDays:]
	}
	return history
}
