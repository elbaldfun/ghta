package service

import (
	"context"
	"os"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// These are characterization (golden) tests: they lock the CURRENT behavior of
// TrendService.List so the P0 performance work (compound indexes, count caching,
// search index) can be proven to change speed WITHOUT changing results. A perf
// optimization that makes one of these fail has changed behavior and must be
// re-examined, not have the test "fixed".
//
// They follow the existing MONGODB_URI integration pattern (see
// repository/items_test.go): set MONGODB_URI to a throwaway mongo to run them,
// otherwise they skip.

func testTrendService(t *testing.T) (*TrendService, *repository.Store) {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	cfg := &config.Config{MongoURI: uri, MongoDB: "ghta_service_test"}
	store, err := repository.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	_, _ = store.Items().DeleteMany(context.Background(), bson.M{"source": domain.SourceGitHub})
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	// History is never used by List; nil is fine.
	return NewTrendService(store, nil), store
}

// item builds a minimal github TrackedItem for seeding.
func item(id string, stars float64) domain.TrackedItem {
	return domain.TrackedItem{
		Source:          domain.SourceGitHub,
		ExternalID:      id,
		Name:            id,
		PrimaryMetric:   "stars",
		MetricDirection: domain.DirectionDescBetter,
		Metrics:         map[string]float64{"stars": stars},
	}
}

func seed(t *testing.T, store *repository.Store, items ...domain.TrackedItem) {
	t.Helper()
	docs := make([]any, len(items))
	for i := range items {
		docs[i] = items[i]
	}
	if _, err := store.Items().InsertMany(context.Background(), docs); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
}

func ids(items []domain.TrackedItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ExternalID
	}
	return out
}

// Golden: default board is stars-descending and total reflects the full match.
func TestListStarsDescOrderAndTotal(t *testing.T) {
	svc, store := testTrendService(t)
	seed(t, store,
		item("o/a", 300),
		item("o/b", 100),
		item("o/c", 500),
		item("o/d", 200),
	)

	items, total, err := svc.List(context.Background(), TrendQuery{
		Source: "github", Sort: "stars:desc", Limit: 50, Page: 1,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	want := []string{"o/c", "o/a", "o/d", "o/b"}
	if got := ids(items); !equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

// Golden + risk guard: paging through tied-stars items must cover every item
// exactly once — no duplicates, no gaps. Skip-based pagination over a sort with
// tie values is only stable if the sort is total-order deterministic; adding a
// compound index can change tie order, which THIS test will catch. If it starts
// failing after an index change, the fix is a tiebreaker (e.g. add _id to the
// sort), not deleting the test.
func TestListPaginationNoDupesOrGapsWithTies(t *testing.T) {
	svc, store := testTrendService(t)
	// 10 items, all the same star count -> all ties on the sort key.
	var seedItems []domain.TrackedItem
	for i := 0; i < 10; i++ {
		seedItems = append(seedItems, item("o/tie"+string(rune('0'+i)), 100))
	}
	seed(t, store, seedItems...)

	seen := map[string]int{}
	const perPage = 3
	for page := 1; page <= 4; page++ {
		items, total, err := svc.List(context.Background(), TrendQuery{
			Source: "github", Sort: "stars:desc", Limit: perPage, Page: page,
		})
		if err != nil {
			t.Fatalf("list page %d: %v", page, err)
		}
		if total != 10 {
			t.Fatalf("total = %d, want 10", total)
		}
		for _, it := range items {
			seen[it.ExternalID]++
		}
	}
	if len(seen) != 10 {
		t.Fatalf("covered %d distinct items across pages, want 10 (gap detected)", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("item %s returned %d times across pages, want 1 (duplicate detected)", id, n)
		}
	}
}

// Golden: type facet filters the board and the total counts only that facet.
func TestListTypeFilterAndTotal(t *testing.T) {
	svc, store := testTrendService(t)
	cli := item("o/cli", 100)
	cli.Type = "cli"
	app := item("o/app", 100)
	app.Type = "app"
	seed(t, store, cli, app, item("o/none", 100))

	items, total, err := svc.List(context.Background(), TrendQuery{
		Source: "github", Type: "cli", Sort: "stars:desc", Limit: 50, Page: 1,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ExternalID != "o/cli" {
		t.Fatalf("type=cli -> items=%v total=%d, want [o/cli] total=1", ids(items), total)
	}
}

// Characterization of the CURRENT search: case-insensitive SUBSTRING regex over
// externalId/name/description. Note the substring semantics that a $text index
// would NOT reproduce: "auth" matches "oauth". When migrating search, record the
// delta against this baseline and decide whether it is acceptable.
func TestListSearchSubstringSemantics(t *testing.T) {
	svc, store := testTrendService(t)
	oauth := item("acme/oauth-proxy", 100)   // "auth" is a substring of "oauth"
	authz := item("acme/authz", 100)         // starts with "auth"
	other := item("acme/router", 100)        // must not match
	seed(t, store, oauth, authz, other)

	items, total, err := svc.List(context.Background(), TrendQuery{
		Source: "github", Q: "auth", Sort: "stars:desc", Limit: 50, Page: 1,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.ExternalID] = true
	}
	if total != 2 || !got["acme/oauth-proxy"] || !got["acme/authz"] || got["acme/router"] {
		t.Fatalf("q=auth -> items=%v total=%d, want {oauth-proxy, authz} (substring match)", ids(items), total)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
