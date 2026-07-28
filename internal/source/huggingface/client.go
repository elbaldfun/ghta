// Package huggingface fetches model rankings from the HuggingFace Hub API and
// normalizes them into TrackedItems (change 14).
package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://huggingface.co/api/models"

// Model is one row of the Hub list API (with our expand set).
type Model struct {
	ID            string   `json:"id"`
	Likes         int      `json:"likes"`
	Downloads     int64    `json:"downloads"` // rolling last-30-days, NOT cumulative
	TrendingScore float64  `json:"trendingScore"`
	PipelineTag   string   `json:"pipeline_tag"`
	LibraryName   string   `json:"library_name"`
	Tags          []string `json:"tags"`
	Gated         any      `json:"gated"` // false | "auto" | "manual"
	Private       bool     `json:"private"`
	CreatedAt     string   `json:"createdAt"`
	LastModified  string   `json:"lastModified"`
}

// Client pages through the Hub list API following Link-header cursors.
type Client struct {
	hc    *http.Client
	token string // optional HF_TOKEN; the list API works unauthenticated
	pace  time.Duration
}

func NewClient(token string) *Client {
	return &Client{
		hc:    &http.Client{Timeout: 30 * time.Second},
		token: token,
		pace:  250 * time.Millisecond,
	}
}

// expand lists the exact fields we need; the API then returns only these,
// keeping pages small and the schema explicit.
var expand = []string{
	"downloads", "likes", "trendingScore", "pipeline_tag", "library_name",
	"tags", "gated", "createdAt", "lastModified",
}

// ListSorted pages through the list API sorted by `sortKey` (downloads | likes |
// trendingScore) until `max` models are collected or pages run out.
func (c *Client) ListSorted(ctx context.Context, sortKey string, max int) ([]Model, error) {
	q := url.Values{}
	q.Set("sort", sortKey)
	q.Set("direction", "-1")
	q.Set("limit", "100")
	for _, e := range expand {
		q.Add("expand[]", e)
	}
	next := apiBase + "?" + q.Encode()

	var out []Model
	for next != "" && len(out) < max {
		page, nextURL, err := c.page(ctx, next)
		if err != nil {
			return nil, fmt.Errorf("hf list %s (have %d): %w", sortKey, len(out), err)
		}
		out = append(out, page...)
		next = nextURL

		if err := sleep(ctx, c.pace); err != nil {
			return nil, err
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out, nil
}

// page fetches one URL and returns its models plus the rel="next" cursor URL.
func (c *Client) page(ctx context.Context, pageURL string) ([]Model, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "ghta/1.0 (starrank.dev)")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
		} else {
			models, next, err := decodePage(resp)
			if err == nil {
				return models, next, nil
			}
			lastErr = err
			if !retryable(resp.StatusCode) {
				return nil, "", lastErr
			}
		}
		if attempt < 3 {
			if err := sleep(ctx, time.Duration(attempt)*2*time.Second); err != nil {
				return nil, "", err
			}
		}
	}
	return nil, "", lastErr
}

func decodePage(resp *http.Response) ([]Model, string, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", fmt.Errorf("hf api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var models []Model
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&models); err != nil {
		return nil, "", fmt.Errorf("hf api decode: %w", err)
	}
	return models, parseNextLink(resp.Header.Get("Link")), nil
}

// parseNextLink extracts the rel="next" URL from a Link header, or "".
func parseNextLink(link string) string {
	for _, part := range strings.Split(link, ",") {
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.IndexByte(part, '<')
		end := strings.IndexByte(part, '>')
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
