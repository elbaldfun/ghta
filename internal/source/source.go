// Package source defines the adapter contract every data source implements and
// a registry the fetch job iterates over. New sources plug in by implementing
// Fetcher and registering — the core fetch/store/snapshot logic is untouched.
package source

import (
	"context"

	"github.com/elbaldfun/ghta/internal/domain"
)

// Fetcher is the contract for a single data source (github, appstore, ...).
// Each adapter handles its own paging/rate-limiting internally and returns
// items already normalized into the source-agnostic TrackedItem shape.
type Fetcher interface {
	// Source returns the source this adapter serves.
	Source() domain.Source

	// Shards returns the units of work for one run. A source that fetches
	// everything in one pass returns a single shard (e.g. []string{""}).
	Shards() []string

	// Fetch pulls and normalizes all items for one shard.
	Fetch(ctx context.Context, shard string) ([]domain.TrackedItem, error)
}

// Registry holds the registered source adapters.
type Registry struct {
	fetchers map[domain.Source]Fetcher
	order    []domain.Source
}

func NewRegistry() *Registry {
	return &Registry{fetchers: make(map[domain.Source]Fetcher)}
}

// Register adds a fetcher. The last registration for a source wins.
func (r *Registry) Register(f Fetcher) {
	if _, exists := r.fetchers[f.Source()]; !exists {
		r.order = append(r.order, f.Source())
	}
	r.fetchers[f.Source()] = f
}

// Get returns the fetcher for a source, if registered.
func (r *Registry) Get(s domain.Source) (Fetcher, bool) {
	f, ok := r.fetchers[s]
	return f, ok
}

// All returns fetchers in registration order.
func (r *Registry) All() []Fetcher {
	out := make([]Fetcher, 0, len(r.order))
	for _, s := range r.order {
		out = append(out, r.fetchers[s])
	}
	return out
}
