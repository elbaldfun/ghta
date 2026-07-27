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

// SWR contract: a stale hit returns the OLD value IMMEDIATELY (never blocking on
// the recompute) and refreshes in the background; a later read sees the fresh
// value. This is the latency cliff removed — no caller ever waits for a
// multi-second board recompute except the very first (cold) load.
func TestTTLCacheServesStaleImmediatelyThenRefreshes(t *testing.T) {
	c := newTTLCache[int](20 * time.Millisecond)
	var calls atomic.Int64
	var fresh atomic.Int64
	fresh.Store(1)
	compute := func(context.Context) (int, error) {
		calls.Add(1)
		time.Sleep(60 * time.Millisecond) // a "slow" recompute
		return int(fresh.Load()), nil
	}

	// Cold load blocks (nothing to serve yet) and returns 1.
	if v, err := c.get(context.Background(), "k", compute); err != nil || v != 1 {
		t.Fatalf("cold load: v=%d err=%v, want 1", v, err)
	}
	fresh.Store(2)
	time.Sleep(30 * time.Millisecond) // now stale

	// Stale hit must return the OLD value fast, without waiting for the recompute.
	start := time.Now()
	v, err := c.get(context.Background(), "k", compute)
	elapsed := time.Since(start)
	if err != nil || v != 1 {
		t.Fatalf("stale hit: v=%d err=%v, want stale 1", v, err)
	}
	if elapsed > 15*time.Millisecond {
		t.Fatalf("stale hit blocked %v — should return immediately", elapsed)
	}

	// The background refresh eventually swaps in the fresh value.
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := c.get(context.Background(), "k", compute)
		if got == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("background refresh never updated the value (still %d)", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Under a burst of concurrent stale reads, at most one background refresh runs
// per key — the stale reads all return immediately and only one compute fires.
func TestTTLCacheSingleBackgroundRefresh(t *testing.T) {
	c := newTTLCache[int](10 * time.Millisecond)
	var calls atomic.Int64
	gate := make(chan struct{})
	compute := func(context.Context) (int, error) {
		n := calls.Add(1)
		if n == 1 {
			return 1, nil // cold seed returns immediately
		}
		<-gate // refresh blocks until released, so we can inspect in-flight count
		return int(n), nil
	}

	if _, err := c.get(context.Background(), "k", compute); err != nil {
		t.Fatalf("seed: %v", err)
	}
	time.Sleep(20 * time.Millisecond) // stale

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, err := c.get(context.Background(), "k", compute); err != nil || v != 1 {
				t.Errorf("stale read: v=%d err=%v, want stale 1", v, err)
			}
		}()
	}
	wg.Wait()
	time.Sleep(30 * time.Millisecond) // let the single refresh goroutine reach compute

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected exactly 1 background refresh (calls=2), got calls=%d", got)
	}
	close(gate)
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
