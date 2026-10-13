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
	if g.D30 == nil || *g.D30 != 900 {
		t.Errorf("d30: %v", g.D30)
	}

	young := ComputeGrowth([]Point{{"2026-10-10", 5}, {"2026-10-12", 9}}, 9, "2026-10-12")
	if young.D1 == nil || *young.D1 != 4 || young.D7 != nil || young.D30 != nil {
		t.Errorf("an item younger than a window has nil, not zero: %+v", young)
	}
	if g := ComputeGrowth(nil, 9, "2026-10-12"); g.D1 != nil {
		t.Errorf("no history: %+v", g)
	}
}
