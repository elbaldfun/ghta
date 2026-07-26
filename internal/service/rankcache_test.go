package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTTLCacheCollapsesConcurrentMisses(t *testing.T) {
	c := newTTLCache[int](time.Minute)
	var calls atomic.Int64
	release := make(chan struct{})

	const n = 20
	var wg sync.WaitGroup
	results := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := c.get(context.Background(), "k", func(context.Context) (int, error) {
				<-release // hold all callers in the same compute window
				calls.Add(1)
				return 42, nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			results[i] = v
		}(i)
	}
	// Give the goroutines time to pile onto the same key, then let compute finish.
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 compute for concurrent same-key misses, got %d", got)
	}
	for i, v := range results {
		if v != 42 {
			t.Fatalf("caller %d got %d, want 42", i, v)
		}
	}
}

func TestTTLCacheServesFreshWithoutRecompute(t *testing.T) {
	c := newTTLCache[int](time.Minute)
	var calls atomic.Int64
	compute := func(context.Context) (int, error) { calls.Add(1); return 7, nil }

	for i := 0; i < 5; i++ {
		if v, err := c.get(context.Background(), "k", compute); err != nil || v != 7 {
			t.Fatalf("get: v=%d err=%v", v, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 compute across repeated fresh hits, got %d", got)
	}
}

func TestTTLCacheServesStaleOnError(t *testing.T) {
	c := newTTLCache[int](time.Nanosecond) // everything is immediately stale
	if _, err := c.get(context.Background(), "k", func(context.Context) (int, error) { return 5, nil }); err != nil {
		t.Fatalf("seed: %v", err)
	}
	time.Sleep(time.Millisecond) // ensure TTL elapsed
	v, err := c.get(context.Background(), "k", func(context.Context) (int, error) {
		return 0, errors.New("upstream down")
	})
	if err != nil {
		t.Fatalf("expected stale value served, got error %v", err)
	}
	if v != 5 {
		t.Fatalf("expected stale value 5, got %d", v)
	}
}

func TestTTLCacheReturnsErrorWhenNoStale(t *testing.T) {
	c := newTTLCache[int](time.Minute)
	_, err := c.get(context.Background(), "k", func(context.Context) (int, error) {
		return 0, errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error when compute fails and no stale value exists")
	}
}

// Characterization of CURRENT behavior: once an entry is stale, get() BLOCKS the
// triggering caller on the full recompute and returns the fresh value (never the
// stale one). This is the latency cliff the stale-while-revalidate work targets:
// after SWR lands, a stale hit should return the OLD value immediately and
// refresh in the background — at which point this test is updated to assert the
// new contract (stale value returned, compute observed asynchronously).
func TestTTLCacheBlocksOnStaleRecompute(t *testing.T) {
	c := newTTLCache[int](time.Nanosecond) // entries are stale almost immediately
	if _, err := c.get(context.Background(), "k", func(context.Context) (int, error) { return 1, nil }); err != nil {
		t.Fatalf("seed: %v", err)
	}
	time.Sleep(time.Millisecond) // ensure TTL elapsed

	v, err := c.get(context.Background(), "k", func(context.Context) (int, error) { return 2, nil })
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if v != 2 {
		t.Fatalf("stale hit returned %d, want 2 (current contract: caller blocks for fresh value)", v)
	}
}

func TestTTLCacheCapsKeySpace(t *testing.T) {
	c := newTTLCache[int](time.Minute)
	total := maxRankCacheKeys + 50
	for i := 0; i < total; i++ {
		key := "k" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + string(rune(i))
		if _, err := c.get(context.Background(), key, func(context.Context) (int, error) { return i, nil }); err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
	}
	c.mu.Lock()
	size := len(c.entries)
	c.mu.Unlock()
	if size > maxRankCacheKeys {
		t.Fatalf("cache grew past cap: %d > %d", size, maxRankCacheKeys)
	}
}
