package github

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestFetchTrendingLive hits the real github.com/trending. Skipped unless
// TRENDING_LIVE=1 so CI stays offline/deterministic.
func TestFetchTrendingLive(t *testing.T) {
	if os.Getenv("TRENDING_LIVE") == "" {
		t.Skip("set TRENDING_LIVE=1 to run the live scrape")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	repos, err := FetchTrending(ctx, NewTrendingClient(), "weekly", "")
	if err != nil {
		t.Fatalf("FetchTrending: %v", err)
	}
	if len(repos) < 5 {
		t.Fatalf("got %d repos, want >=5", len(repos))
	}
	for i := 0; i < 5 && i < len(repos); i++ {
		r := repos[i]
		t.Logf("%2d. %-40s +%d this week (total %d, %s)", i+1, r.ExternalID, r.StarsThisPeriod, r.Stars, r.Language)
	}
}
