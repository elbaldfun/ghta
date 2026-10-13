package main

import (
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/worker"
)

func TestPickKeepsPipelineOrder(t *testing.T) {
	jobs := nightly(nil, nil)
	all := strings.Join(names(jobs), ",")

	// Asked for in reverse, still returned in pipeline order.
	rev := names(jobs)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	got, err := pick(jobs, " "+strings.Join(rev, " , "))
	if err != nil || strings.Join(names(got), ",") != all {
		t.Fatalf("pick = %v, %v; want %s", names(got), err, all)
	}
	if _, err := pick(jobs, "rank,nope"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown job not rejected: %v", err)
	}
	if _, err := pick(jobs, " , "); err == nil {
		t.Fatal("empty -run not rejected")
	}
}

func TestNightlyNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, j := range nightly(nil, nil) {
		if j.Name() == "" || seen[j.Name()] {
			t.Fatalf("job name %q is empty or repeated", j.Name())
		}
		seen[j.Name()] = true
	}
	var _ []worker.Job = nightly(nil, nil)
}
