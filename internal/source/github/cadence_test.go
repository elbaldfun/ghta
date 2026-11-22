package github

import "testing"

func TestCadence(t *testing.T) {
	a := NewAdapter("", 0, 50, nil)
	days := []string{"2026-11-21", "2026-11-22"}
	due := map[string][2]bool{}
	for _, sh := range a.Shards() {
		due[sh] = [2]bool{a.Due(sh, days[0]), a.Due(sh, days[1])}
	}
	low, high := 0, 0
	for sh, d := range due {
		lo, _, ok := parseShard(sh)
		if !ok {
			t.Fatalf("shard %q does not parse", sh)
		}
		switch {
		case lo >= altStars:
			if !d[0] || !d[1] {
				t.Errorf("%s is above %d and must be due every day: %v", sh, altStars, d)
			}
		case d[0] == d[1]:
			t.Errorf("%s below %d must be due on exactly one of two days: %v", sh, altStars, d)
		case lo < altSplit:
			low++
			if d != due["1000..1010"] {
				t.Errorf("%s is in the lower half and must follow 1000..1010: %v", sh, d)
			}
		default:
			high++
			if d != due["2990..3000"] {
				t.Errorf("%s is in the upper half and must follow 2990..3000: %v", sh, d)
			}
		}
	}
	if due["1000..1010"] == due["2990..3000"] {
		t.Errorf("the two halves must take turns: %v %v", due["1000..1010"], due["2990..3000"])
	}
	if low != 50 || high != 150 {
		t.Errorf("halves: %d shards below %d, %d above; want 50 and 150", low, altSplit, high)
	}

	// Anything unreadable is due: a bad date never silently drops shards.
	if !a.Due("1000..1010", "not a date") || !a.Due("weird", days[0]) {
		t.Error("unreadable shard or date must be due")
	}
	if m, lo, hi, ok := a.Span("1490..1500"); !ok || m != "stars" || lo != 1490 || hi != 1500 {
		t.Errorf("span = %s %v..%v %v", m, lo, hi, ok)
	}
	if _, _, _, ok := a.Span("junk"); ok {
		t.Error("junk shard has no span")
	}
}
