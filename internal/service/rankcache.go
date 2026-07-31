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
const maxRankCacheKeys = 1024

// ttlCache is a small TTL cache shared by the board services. It exists to fix
// two problems the hand-rolled caches had: (1) they held a single mutex across
// the multi-second recompute, so a miss on one key blocked callers of every
// other key; (2) their key space was unbounded. Here compute runs OUTSIDE the
// lock, concurrent misses for the same key collapse into one call (singleflight),
// and the key count is capped with oldest-eviction.
type ttlCache[T any] struct {
	mu      sync.Mutex
	entries map[string]cacheEntry[T]
	sf      singleflight.Group
	ttl     time.Duration
	maxKeys int
}

type cacheEntry[T any] struct {
	val T
	at  time.Time
}

func newTTLCache[T any](ttl time.Duration) *ttlCache[T] {
	return &ttlCache[T]{entries: map[string]cacheEntry[T]{}, ttl: ttl, maxKeys: maxRankCacheKeys}
}

// get returns a fresh cached value for key, or computes it. On a compute error a
// stale value is served if one exists, otherwise the error is returned. The lock
// is only ever held for map access, never across compute.
func (c *ttlCache[T]) get(ctx context.Context, key string, compute func(context.Context) (T, error)) (T, error) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && time.Since(e.at) < c.ttl {
		c.mu.Unlock()
		return e.val, nil
	}
	c.mu.Unlock()

	v, err, _ := c.sf.Do(key, func() (any, error) {
		val, err := compute(ctx)
		if err != nil {
			if e, ok := c.read(key); ok {
				return e.val, nil // serve stale rather than fail
			}
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

func (c *ttlCache[T]) read(key string) (cacheEntry[T], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e, ok
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
