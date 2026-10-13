package main

import (
	"errors"
	"testing"

	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// The run state is driven from the HTTP handler and the migration goroutine;
// walk it through a failed run and a second, successful one.
func TestMigrateRunLifecycle(t *testing.T) {
	r := &migrateRun{}
	if got := r.snapshot()["status"]; got != "idle" {
		t.Fatalf("fresh state: %v", got)
	}
	if !r.start(2, 3) {
		t.Fatal("first start refused")
	}
	if r.start(2, 3) {
		t.Fatal("second start accepted while running")
	}
	if got := r.snapshot()["status"]; got != "running" {
		t.Fatalf("while running: %v", got)
	}
	r.finish(taxonomy.Stats{DomainRewritten: 7}, errors.New("boom"))
	s := r.snapshot()
	if s["status"] != "failed" || s["error"] != "boom" || s["stats"].(taxonomy.Stats).DomainRewritten != 7 {
		t.Fatalf("after a failed run: %v", s)
	}

	// A new run clears the previous result.
	if !r.start(2, 3) {
		t.Fatal("restart refused after a finished run")
	}
	r.finish(taxonomy.Stats{}, nil)
	s = r.snapshot()
	if _, hasErr := s["error"]; s["status"] != "done" || hasErr {
		t.Fatalf("after a clean run: %v", s)
	}
}
