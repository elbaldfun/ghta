package main

import (
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/worker"
)

func pipelineJobs(t *testing.T) []worker.Job {
	t.Helper()
	jobs, err := nightly(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

func TestPickKeepsPipelineOrder(t *testing.T) {
	jobs := pipelineJobs(t)
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
	for _, j := range pipelineJobs(t) {
		if j.Name() == "" || seen[j.Name()] {
			t.Fatalf("job name %q is empty or repeated", j.Name())
		}
		seen[j.Name()] = true
	}
}

// The judge needs a model the production host cannot reach, so it is part of
// the pipeline only where one is configured, and then it runs before rank.
func TestJudgeJoinsOnlyWithAModel(t *testing.T) {
	t.Setenv("JUDGE_BASE_URL", "")
	if got := strings.Join(names(pipelineJobs(t)), ","); strings.Contains(got, "judge") {
		t.Fatalf("pipeline without a model: %s", got)
	}

	t.Setenv("JUDGE_BASE_URL", "http://127.0.0.1:1234/v1")
	t.Setenv("JUDGE_MODEL", "")
	if _, err := nightly(nil, nil); err == nil {
		t.Fatal("JUDGE_BASE_URL without JUDGE_MODEL was accepted")
	}
	t.Setenv("JUDGE_MODEL", "some-model")
	t.Setenv("TAXONOMY_PATH", "../../taxonomy/taxonomy.yaml")
	if got := strings.Join(names(pipelineJobs(t)), ","); got != "fetch,reconcile,judge,rank,icon,shot,devsync" {
		t.Fatalf("pipeline with a model: %s", got)
	}
}
