// Package appstore reads Apple's public App Store charts and listings
// (docs/MOBILE-APPS-DATA.md §2.1): the legacy iTunes RSS charts — the only
// public feed with top grossing and per-genre charts — and the iTunes Lookup
// API. Neither needs a key. Requests are paced well under Apple's documented
// ~20 per minute.
package appstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Chart kinds, as stored in chart_snapshots.
const (
	ChartFree     = "free"
	ChartPaid     = "paid"
	ChartGrossing = "grossing"
)

// Charts lists the chart kinds in fetch order.
var Charts = []string{ChartFree, ChartPaid, ChartGrossing}

var feedName = map[string]string{
	ChartFree:     "topfreeapplications",
	ChartPaid:     "toppaidapplications",
	ChartGrossing: "topgrossingapplications",
}

// GenreAll is the genre key of a store-wide chart.
const GenreAll = "all"

// LookupBatch is how many ids one Lookup request carries (tested: 100 back).
const LookupBatch = 100

const userAgent = "Mozilla/5.0 (compatible; StarRank/1.0; +https://starrank.dev)"

var (
	// ErrThrottled means Apple refused us (429 or 403): stop for the day.
	ErrThrottled = errors.New("appstore: throttled")
	// ErrNoChart means the storefront has no such chart (404).
	ErrNoChart = errors.New("appstore: no such chart")
)

// Client talks to itunes.apple.com, one request at a time, at least Pace
// apart.
type Client struct {
	hc      *http.Client
	base    string
	pace    time.Duration
	retries []time.Duration // waits before each retry of a failed request

	mu   sync.Mutex
	last time.Time
}

// NewClient returns a client pacing requests pace apart.
func NewClient(pace time.Duration) *Client {
	return &Client{
		hc:      &http.Client{Timeout: 45 * time.Second},
		base:    "https://itunes.apple.com",
		pace:    pace,
		retries: []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second},
	}
}

// ChartEntry is one row of a chart; Rank starts at 1.
type ChartEntry struct {
	ID       string
	BundleID string
	Name     string
	Rank     int
}

// Chart reads one chart of a storefront: the top 100 at most.
func (c *Client) Chart(ctx context.Context, country, chart, genre string) ([]ChartEntry, error) {
	name, ok := feedName[chart]
	if !ok {
		return nil, fmt.Errorf("appstore: unknown chart %q", chart)
	}
	path := fmt.Sprintf("/%s/rss/%s/limit=200", url.PathEscape(country), name)
	if genre != "" && genre != GenreAll {
		path += "/genre=" + url.PathEscape(genre)
	}
	body, err := c.get(ctx, path+"/json")
	if err != nil {
		return nil, err
	}
	return parseChart(body)
}

// Lookup reads the listings of up to LookupBatch apps in one storefront. Apps
// the storefront no longer sells are missing from the result.
func (c *Client) Lookup(ctx context.Context, country string, ids []string) ([]App, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > LookupBatch {
		return nil, fmt.Errorf("appstore: lookup of %d ids, at most %d", len(ids), LookupBatch)
	}
	q := url.Values{"id": {strings.Join(ids, ",")}, "country": {country}, "entity": {"software"}}
	body, err := c.get(ctx, "/lookup?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var res struct {
		Results []App `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("appstore: lookup: %w", err)
	}
	out := res.Results[:0]
	for _, a := range res.Results {
		if a.Kind == "software" && a.TrackID != 0 {
			out = append(out, a)
		}
	}
	return out, nil
}

// get fetches one path, retrying server errors and timeouts.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		body, retry, err := c.once(ctx, path)
		if err == nil || !retry {
			return body, err
		}
		lastErr = err
		if attempt >= len(c.retries) {
			return nil, lastErr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.retries[attempt]):
		}
	}
}

func (c *Client) once(ctx context.Context, path string) (body []byte, retry bool, err error) {
	if err := c.wait(ctx); err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, fmt.Errorf("appstore: %w", err)
	}
	defer resp.Body.Close()
	// A chart is ~0.5 MB and a lookup of 100 apps ~1 MB; 8 MB is a broken reply.
	body, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	switch {
	case resp.StatusCode == http.StatusOK && err == nil:
		return body, false, nil
	case resp.StatusCode == http.StatusOK:
		return nil, true, fmt.Errorf("appstore: read body: %w", err)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden:
		return nil, false, fmt.Errorf("%w: http %d", ErrThrottled, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return nil, false, ErrNoChart
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("appstore: http %d", resp.StatusCode)
	default:
		return nil, false, fmt.Errorf("appstore: http %d", resp.StatusCode)
	}
}

// wait blocks until pace has passed since the previous request.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := c.pace - time.Since(c.last); d > 0 && !c.last.IsZero() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	c.last = time.Now()
	return nil
}

// ---- chart feed ---------------------------------------------------------------

type rssEntry struct {
	ID struct {
		Attributes struct {
			ID       string `json:"im:id"`
			BundleID string `json:"im:bundleId"`
		} `json:"attributes"`
	} `json:"id"`
	Name struct {
		Label string `json:"label"`
	} `json:"im:name"`
}

// parseChart decodes a legacy RSS chart. The feed writes a one-entry chart
// as a bare object rather than a list, and leaves "entry" out when empty.
func parseChart(body []byte) ([]ChartEntry, error) {
	var feed struct {
		Feed struct {
			Entry json.RawMessage `json:"entry"`
		} `json:"feed"`
	}
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("appstore: chart: %w", err)
	}
	raw := []byte(strings.TrimSpace(string(feed.Feed.Entry)))
	var entries []rssEntry
	switch {
	case len(raw) == 0 || string(raw) == "null":
	case raw[0] == '{':
		var one rssEntry
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, fmt.Errorf("appstore: chart entry: %w", err)
		}
		entries = []rssEntry{one}
	default:
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("appstore: chart entries: %w", err)
		}
	}
	out := make([]ChartEntry, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		id := e.ID.Attributes.ID
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, ChartEntry{ID: id, BundleID: e.ID.Attributes.BundleID, Name: e.Name.Label, Rank: len(out) + 1})
	}
	return out, nil
}

// ---- lookup -------------------------------------------------------------------

// App is one Lookup result, with the fields we keep.
type App struct {
	Kind                      string   `json:"kind"`
	TrackID                   int64    `json:"trackId"`
	BundleID                  string   `json:"bundleId"`
	TrackName                 string   `json:"trackName"`
	Description               string   `json:"description"`
	ArtistID                  int64    `json:"artistId"`
	ArtistName                string   `json:"artistName"`
	SellerURL                 string   `json:"sellerUrl"`
	PrimaryGenreID            int      `json:"primaryGenreId"`
	PrimaryGenreName          string   `json:"primaryGenreName"`
	GenreIDs                  []string `json:"genreIds"`
	Price                     float64  `json:"price"`
	Currency                  string   `json:"currency"`
	AverageUserRating         float64  `json:"averageUserRating"`
	UserRatingCount           float64  `json:"userRatingCount"`
	Version                   string   `json:"version"`
	CurrentVersionReleaseDate string   `json:"currentVersionReleaseDate"`
	ReleaseDate               string   `json:"releaseDate"`
	ContentAdvisoryRating     string   `json:"contentAdvisoryRating"`
	ArtworkURL512             string   `json:"artworkUrl512"`
	ArtworkURL100             string   `json:"artworkUrl100"`
	ScreenshotURLs            []string `json:"screenshotUrls"`
	IPadScreenshotURLs        []string `json:"ipadScreenshotUrls"`
}
