package service

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// maxRankCacheKeys bounds every board cache's key space. Some board keys embed
// an unvalidated `domain`/`lang` query param, so without a cap a client could
// enumerate distinct values and grow the map without limit (memory exhaustion).
const maxRankCacheKeys = 256

// refreshTimeout caps a background recompute so a wedged aggregation can't pin a
// goroutine forever. It's generous — the board aggregations run in a few seconds
// on the prod box — because the refresh is off the request path.
const refreshTimeout = 60 * time.Second

// ttlCache is a small stale-while-revalidate cache shared by the board services.
// Design goals: (1) a request never blocks on a multi-second recompute except on
// the very first (cold) load — once a value exists, an expired key is served
// stale immediately and refreshed in the background; (2) concurrent cold misses
// for a key collapse into one compute (singleflight) and at most one background
// refresh runs per key at a time; (3) the key count is capped with
// oldest-eviction. The lock is only ever held for map access, never across
// compute.
type ttlCache[T any] struct {
	mu       sync.Mutex
	entries  map[string]cacheEntry[T]
	inflight map[string]bool // keys with a background refresh running
	sf       singleflight.Group
	ttl      time.Duration
	maxKeys  int
}

type cacheEntry[T any] struct {
	val T
	at  time.Time
}

func newTTLCache[T any](ttl time.Duration) *ttlCache[T] {
	return &ttlCache[T]{
		entries:  map[string]cacheEntry[T]{},
		inflight: map[string]bool{},
		ttl:      ttl,
		maxKeys:  maxRankCacheKeys,
	}
}

// get returns the cached value for key. Fresh values return directly; a stale
// value returns immediately while a background refresh is kicked off (so the
// caller never pays the recompute latency); a cold miss computes synchronously
// (nothing to serve yet). compute's ctx is the caller's for the cold path and a
// detached, timeout-bounded context for background refreshes — a background
// refresh must outlive the request that triggered it.
func (c *ttlCache[T]) get(ctx context.Context, key string, compute func(context.Context) (T, error)) (T, error) {
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok {
		if time.Since(e.at) >= c.ttl {
			c.refresh(key, compute) // stale: serve now, refresh behind the scenes
		}
		return e.val, nil
	}

	// Cold miss: no value to serve, so block (collapsing concurrent misses).
	v, err, _ := c.sf.Do(key, func() (any, error) {
		val, err := compute(ctx)
		if err != nil {
			return nil, err
		}
		c.store(key, val)
		return val, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}

// refresh recomputes key in the background, at most one goroutine per key. On
// error it keeps the stale value (a later stale read retries); on success it
// swaps in the fresh value. The context is detached from any request.
func (c *ttlCache[T]) refresh(key string, compute func(context.Context) (T, error)) {
	c.mu.Lock()
	if c.inflight[key] {
		c.mu.Unlock()
		return
	}
	c.inflight[key] = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.inflight, key)
			c.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()
		if val, err := compute(ctx); err == nil {
			c.store(key, val)
		}
	}()
}

func (c *ttlCache[T]) store(key string, val T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && c.maxKeys > 0 && len(c.entries) >= c.maxKeys {
		c.evictOldest()
	}
	c.entries[key] = cacheEntry[T]{val: val, at: time.Now()}
}

// evictOldest removes the least-recently-stored entry. Caller holds c.mu. O(n)
// over a capped map, so cheap.
func (c *ttlCache[T]) evictOldest() {
	var oldestKey string
	var oldestAt time.Time
	first := true
	for k, e := range c.entries {
		if first || e.at.Before(oldestAt) {
			oldestKey, oldestAt, first = k, e.at, false
		}
	}
	if !first {
		delete(c.entries, oldestKey)
	}
}
