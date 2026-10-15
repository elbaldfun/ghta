package github

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// flakyServer answers with the given failures in turn, then a one-repo page.
func flakyServer(t *testing.T, fail ...func(w http.ResponseWriter)) (*Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		calls++
		if calls <= len(fail) {
			fail[calls-1](w)
			return
		}
		page := searchResponse{}
		page.Data.RateLimit = rateLimit{Remaining: 4000}
		page.Data.Search.Edges = []struct {
			Node repoNode `json:"node"`
		}{{Node: repoWith("o", "r", 1500)}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(page)
	}))
	t.Cleanup(srv.Close)
	c := NewClient("tok", 50, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.endpoint = srv.URL
	return c, &calls
}

func TestSearchRetriesSecondaryRateLimit(t *testing.T) {
	c, calls := flakyServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit."}`))
	})
	start := time.Now()
	out, err := c.search(context.Background(), "stars:1..2", 10, "")
	if err != nil || *calls != 2 || len(out.Data.Search.Edges) != 1 {
		t.Fatalf("calls %d, err %v; a rate-limited 403 must be retried", *calls, err)
	}
	// Retry-After: 0 means the floor of one second, not the 2s backoff.
	if d := time.Since(start); d >= 2*time.Second {
		t.Fatalf("waited %v; the retry must follow Retry-After", d)
	}
}

func TestSearchRetriesClientClosed(t *testing.T) {
	c, calls := flakyServer(t, func(w http.ResponseWriter) { w.WriteHeader(statusClientClosed) })
	if _, err := c.search(context.Background(), "stars:1..2", 10, ""); err != nil || *calls != 2 {
		t.Fatalf("calls %d, err %v; a 499 must be retried", *calls, err)
	}
}

func TestSearchGivesUpOnPlainForbidden(t *testing.T) {
	c, calls := flakyServer(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
	})
	_, err := c.search(context.Background(), "stars:1..2", 10, "")
	if err == nil || !strings.Contains(err.Error(), "non-retryable") || *calls != 1 {
		t.Fatalf("calls %d, err %v; a 403 that is not a rate limit must not be retried", *calls, err)
	}
}

func TestRateLimitedWait(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cases := []struct {
		name   string
		status int
		header map[string]string
		body   string
		want   bool
		wait   time.Duration
	}{
		{"retry-after", 403, map[string]string{"Retry-After": "30"}, "", true, 30 * time.Second},
		{"retry-after capped", 429, map[string]string{"Retry-After": "86400"}, "", true, maxLimitWait},
		{"quota exhausted", 403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1000120"}, "", true, 121 * time.Second},
		{"secondary in body", 403, nil, `{"message":"You have exceeded a secondary rate limit"}`, true, defaultLimitWait},
		{"plain forbidden", 403, nil, `{"message":"Bad credentials"}`, false, 0},
		{"server error", 502, map[string]string{"Retry-After": "5"}, "", false, 0},
	}
	for _, tc := range cases {
		resp := &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.body))}
		for k, v := range tc.header {
			resp.Header.Set(k, v)
		}
		got, wait := rateLimited(resp, now)
		if got != tc.want || wait != tc.wait {
			t.Errorf("%s: limited %v wait %v, want %v %v", tc.name, got, wait, tc.want, tc.wait)
		}
	}
}
