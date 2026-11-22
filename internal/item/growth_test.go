package item

import (
	"reflect"
	"testing"
)

func TestSetPoint(t *testing.T) {
	var h []Point
	for _, p := range []Point{{"2026-10-03", 3}, {"2026-10-01", 1}, {"2026-10-02", 2}, {"2026-10-03", 30}} {
		h = SetPoint(h, p)
	}
	want := []Point{{"2026-10-01", 1}, {"2026-10-02", 2}, {"2026-10-03", 30}}
	if !reflect.DeepEqual(h, want) {
		t.Fatalf("ascending, same day replaced: %v", h)
	}
	for d := 0; d < 60; d++ {
		h = SetPoint(h, Point{daysBefore("2027-03-01", -d), float64(d)})
	}
	if len(h) != HistoryDays || h[len(h)-1].V != 59 || h[0].V != 59-HistoryDays+1 {
		t.Fatalf("ring keeps the newest %d: len=%d first=%v last=%v", HistoryDays, len(h), h[0], h[len(h)-1])
	}
}

func TestComputeGrowth(t *testing.T) {
	h := []Point{{"2026-09-01", 100}, {"2026-10-05", 400}, {"2026-10-11", 900}, {"2026-10-12", 1000}}
	g := ComputeGrowth(h, 1000, "2026-10-12")
	if g.D1 == nil || *g.D1 != 100 {
		t.Errorf("d1 baseline is yesterday's close: %v", g.D1)
	}
	if g.D7 == nil || *g.D7 != 600 {
		t.Errorf("d7 baseline is the latest point 7+ days back (a fetch gap falls back further): %v", g.D7)
	}
	// The only point 30+ days back is 41 days old: 900 over 41 days is 659 over 30.
	if g.D30 == nil || *g.D30 != 659 {
		t.Errorf("d30 prorated to 30 days: %v", g.D30)
	}

	// Fetched every other day: the two-day gain is halved, the 8-day gain
	// scaled to 7 days; a window whose baseline is exactly n days back is not
	// scaled.
	alt := []Point{{"2026-10-02", 100}, {"2026-10-04", 120}, {"2026-10-06", 140}, {"2026-10-08", 160}, {"2026-10-10", 180}}
	ag := ComputeGrowth(alt, 180, "2026-10-10")
	if ag.D1 == nil || *ag.D1 != 10 {
		t.Errorf("d1 from a two-day gap is halved: %v", ag.D1)
	}
	if ag.D7 == nil || *ag.D7 != 70 {
		t.Errorf("d7 from an eight-day gap is 80*7/8: %v", ag.D7)
	}
	if g := ComputeGrowth(alt[:4], 160, "2026-10-09"); g.D1 == nil || *g.D1 != 0 {
		t.Errorf("unchanged since the last point is zero: %+v", g.D1)
	}

	young := ComputeGrowth([]Point{{"2026-10-10", 5}, {"2026-10-12", 9}}, 9, "2026-10-12")
	if young.D1 == nil || *young.D1 != 2 || young.D7 != nil || young.D30 != nil {
		t.Errorf("an item younger than a window has nil, not zero: %+v", young)
	}
	if g := ComputeGrowth(nil, 9, "2026-10-12"); g.D1 != nil {
		t.Errorf("no history: %+v", g)
	}
}
