package store

import (
	"strings"
	"testing"
)

func TestSpreadOwners(t *testing.T) {
	ids := strings.Split("a/1,a/2,a/3,a/4,b/1,c/1,a/5,d/1", ",")
	for _, c := range []struct {
		size int
		want string
	}{
		// a/3 and a/4 wait for the next page and lead it.
		{4, "a/1,a/2,b/1,c/1,a/3,a/4,d/1,a/5"},
		{3, "a/1,a/2,b/1,a/3,a/4,c/1,a/5,d/1"},
		// One page holds them all: two per owner, then what is left over.
		{0, "a/1,a/2,a/3,a/4,b/1,c/1,a/5,d/1"},
	} {
		if got := strings.Join(SpreadOwners(ids, c.size, 2), ","); got != c.want {
			t.Errorf("size %d = %s; want %s", c.size, got, c.want)
		}
	}
	// Only one owner left: the pages fill with it rather than stop.
	if got := strings.Join(SpreadOwners(strings.Split("a/1,a/2,a/3", ","), 2, 2), ","); got != "a/1,a/2,a/3" {
		t.Errorf("one owner = %s", got)
	}
}
