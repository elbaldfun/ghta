// Package source defines the adapter contract every data source implements and
// a registry the fetch job iterates over. New sources plug in by implementing
// Fetcher and registering — the core fetch/store/snapshot logic is untouched.
package source

import (
	"context"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
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

// ItemFetcher is the adapter contract of the rebuilt pipeline: the same shards
// as Fetcher, with typed output for the items collection. Adapters implement
// both until the old fetch job is removed.
type ItemFetcher interface {
	Source() domain.Source
	Shards() []string
	// FetchItems pulls one shard. Every returned item carries the fields the
	// fetcher owns; see item.Fetched.
	FetchItems(ctx context.Context, shard string) ([]item.Fetched, error)
}

// Registry holds the registered source adapters.
type Registry struct {
	fetchers map[domain.Source]Fetcher
	order    []domain.Source
}
