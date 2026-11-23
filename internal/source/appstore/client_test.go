package appstore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient(0)
	c.base = srv.URL
	c.retries = []time.Duration{time.Millisecond, time.Millisecond}
	return c
}

const twoEntries = `{"feed":{"entry":[
 {"im:name":{"label":"ChatGPT"},"id":{"label":"x","attributes":{"im:id":"6448311069","im:bundleId":"com.openai.chat"}}},
 {"im:name":{"label":"Dup"},"id":{"attributes":{"im:id":"6448311069"}}},
 {"im:name":{"label":"Threads"},"id":{"attributes":{"im:id":"6446901002","im:bundleId":"com.burbn.barcelona"}}}]}}`

func TestChartPathAndParse(t *testing.T) {
	var path string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if !strings.Contains(r.UserAgent(), "StarRank") {
			t.Errorf("user agent %q", r.UserAgent())
		}
		w.Write([]byte(twoEntries))
	})
	got, err := c.Chart(context.Background(), "jp", ChartGrossing, "6007")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/jp/rss/topgrossingapplications/limit=200/genre=6007/json" {
		t.Errorf("path %s", path)
	}
	// The repeated id is dropped and ranks stay contiguous.
	if len(got) != 2 || got[0].ID != "6448311069" || got[0].Rank != 1 || got[1].Rank != 2 || got[0].BundleID != "com.openai.chat" {
		t.Fatalf("entries %+v", got)
	}
	if _, err := c.Chart(context.Background(), "us", ChartFree, GenreAll); err != nil || path != "/us/rss/topfreeapplications/limit=200/json" {
		t.Fatalf("overall chart path %s, %v", path, err)
	}
}

func TestChartOddShapes(t *testing.T) {
	for name, body := range map[string]string{
		"one entry as object": `{"feed":{"entry":{"im:name":{"label":"A"},"id":{"attributes":{"im:id":"1"}}}}}`,
		"no entry":            `{"feed":{"author":{}}}`,
	} {
		got, err := parseChart([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := strings.Count(body, "im:id"); len(got) != want {
			t.Errorf("%s: %d entries, want %d", name, len(got), want)
		}
	}
	if _, err := parseChart([]byte("<html>")); err == nil {
		t.Error("html accepted as a chart")
	}
}

func TestStatusHandling(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
		calls  int32
	}{
		{404, ErrNoChart, 1},
		{429, ErrThrottled, 1},
		{403, ErrThrottled, 1},
		{502, nil, 3}, // retried twice, then given up
	} {
		var calls atomic.Int32
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(tc.status)
		})
		_, err := c.Chart(context.Background(), "us", ChartFree, GenreAll)
		if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("http %d: err %v, want %v", tc.status, err, tc.want)
		}
		if calls.Load() != tc.calls {
			t.Errorf("http %d: %d calls, want %d", tc.status, calls.Load(), tc.calls)
		}
	}
}

func TestServerErrorThenSuccess(t *testing.T) {
	var calls atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte(twoEntries))
	})
	got, err := c.Chart(context.Background(), "us", ChartPaid, GenreAll)
	if err != nil || len(got) != 2 {
		t.Fatalf("%d entries, %v", len(got), err)
	}
}

func TestLookup(t *testing.T) {
	var query string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{"resultCount":2,"results":[
		 {"kind":"software","trackId":6448311069,"trackName":"ChatGPT","userRatingCount":661494,"averageUserRating":4.8},
		 {"kind":"mac-software","trackId":1}]}`))
	})
	got, err := c.Lookup(context.Background(), "cn", []string{"6448311069", "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "id=6448311069%2C1") || !strings.Contains(query, "country=cn") {
		t.Errorf("query %s", query)
	}
	if len(got) != 1 || got[0].UserRatingCount != 661494 {
		t.Fatalf("results %+v", got)
	}
	if _, err := c.Lookup(context.Background(), "us", make([]string, LookupBatch+1)); err == nil {
		t.Error("oversized lookup accepted")
	}
}

func TestPace(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"feed":{}}`)) })
	c.pace = 40 * time.Millisecond
	start := time.Now()
	for range 3 {
		if _, err := c.Chart(context.Background(), "us", ChartFree, GenreAll); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Fatalf("3 requests in %v, want at least 2 paces", d)
	}
}
