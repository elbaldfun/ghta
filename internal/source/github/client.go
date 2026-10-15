// Package github implements the GitHub source adapter: a GraphQL client that
// respects the API rate limit and retries transient errors, plus normalization
// of repositories into domain.TrackedItem.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	endpoint   = "https://api.github.com/graphql"
	maxRetries = 5

	// statusClientClosed is what GitHub answers when it gives up on a slow
	// search on its side; the same query usually succeeds a moment later.
	statusClientClosed = 499

	// A rate-limited answer that names no wait gets this one; a named wait is
	// capped so a bogus reset time cannot stall the run for hours.
	defaultLimitWait = time.Minute
	maxLimitWait     = 15 * time.Minute
)

// Client talks to the GitHub GraphQL API.
type Client struct {
	http       *http.Client
	endpoint   string
	token      string
	rateBuffer int
	log        *slog.Logger
}

func NewClient(token string, rateBuffer int, log *slog.Logger) *Client {
	return &Client{
		http:       &http.Client{Timeout: 60 * time.Second},
		endpoint:   endpoint,
		token:      token,
		rateBuffer: rateBuffer,
		log:        log,
	}
}

type rateLimit struct {
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
	Cost      int       `json:"cost"`
}

type pageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
	StartCursor *string `json:"startCursor"`
}

type searchResponse struct {
	Data struct {
		RateLimit rateLimit `json:"rateLimit"`
		Search    struct {
			PageInfo pageInfo `json:"pageInfo"`
			Edges    []struct {
				Node repoNode `json:"node"`
			} `json:"edges"`
		} `json:"search"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const searchQuery = `
query($q: String!, $first: Int!, $after: String) {
  rateLimit { remaining resetAt cost }
  search(query: $q, type: REPOSITORY, first: $first, after: $after) {
    pageInfo { hasNextPage endCursor startCursor }
    edges {
      node {
        ... on Repository {
          name
          owner { login }
          description
          url
          homepageUrl
          stargazerCount
          forkCount
          pushedAt
          primaryLanguage { name }
          issues(states: OPEN) { totalCount }
          licenseInfo { name }
          repositoryTopics(first: 20) { edges { node { topic { name } url } } }
          releases(first: 5, orderBy: {field: CREATED_AT, direction: DESC}) {
            edges { node {
              name tagName isPrerelease isLatest isDraft publishedAt
              releaseAssets(first: 40) { nodes { name downloadUrl size } }
            } }
          }
          readme: object(expression: "HEAD:README.md") { ... on Blob { text } }
        }
      }
    }
  }
}`

// search runs one search query for a star range page, honoring rate limits and
// retrying transient failures with exponential backoff.
func (c *Client) search(ctx context.Context, query string, first int, after string) (*searchResponse, error) {
	vars := map[string]any{"q": query, "first": first}
	if after != "" {
		vars["after"] = after
	}
	body, err := json.Marshal(map[string]any{"query": searchQuery, "variables": vars})
	if err != nil {
		return nil, err
	}

	var lastErr error
	var wait time.Duration // before the next attempt; 0 means the backoff
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := wait
			if delay == 0 {
				delay = time.Duration(math.Pow(2, float64(attempt))) * time.Second
			}
			wait = 0
			c.log.Warn("github retry", "attempt", attempt, "delay", delay.String(), "err", lastErr)
			if err := sleep(ctx, delay); err != nil {
				return nil, err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue // network errors are retryable
		}

		if resp.StatusCode != http.StatusOK {
			limited, d := rateLimited(resp, time.Now())
			resp.Body.Close()
			switch {
			case limited:
				lastErr = fmt.Errorf("github http %d (rate limited)", resp.StatusCode)
				wait = d
				continue
			case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == statusClientClosed || resp.StatusCode >= 500:
				lastErr = fmt.Errorf("github http %d", resp.StatusCode)
				continue
			default:
				return nil, fmt.Errorf("github http %d (non-retryable)", resp.StatusCode)
			}
		}

		var out searchResponse
		dec := json.NewDecoder(resp.Body)
		decErr := dec.Decode(&out)
		resp.Body.Close()
		if decErr != nil {
			lastErr = decErr
			continue
		}
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("graphql error: %s", out.Errors[0].Message)
		}

		c.waitIfRateLimited(ctx, out.Data.RateLimit)
		return &out, nil
	}
	return nil, fmt.Errorf("github search failed after %d attempts: %w", maxRetries, lastErr)
}

// waitIfRateLimited pauses until resetAt when the remaining quota falls below the
// configured buffer, so the process recovers on its own without a restart.
func (c *Client) waitIfRateLimited(ctx context.Context, rl rateLimit) {
	if rl.Remaining >= c.rateBuffer || rl.ResetAt.IsZero() {
		return
	}
	wait := time.Until(rl.ResetAt)
	if wait <= 0 {
		return
	}
	c.log.Warn("github rate limit low, pausing", "remaining", rl.Remaining, "resetAt", rl.ResetAt, "wait", wait.String())
	_ = sleep(ctx, wait)
}

// rateLimited reports whether a non-200 answer is GitHub's rate limiting
// (primary or secondary) rather than a refusal, and how long to wait before
// trying again. GitHub answers both kinds of limit with 403 or 429: a limited
// answer carries Retry-After, an exhausted X-RateLimit-Remaining, or names the
// secondary limit in its body. Any other 403 (a bad token, a blocked
// repository) is not worth retrying.
func rateLimited(resp *http.Response, now time.Time) (bool, time.Duration) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return false, 0
	}
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return true, capWait(time.Duration(secs) * time.Second)
		}
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			return true, capWait(time.Unix(reset, 0).Sub(now) + time.Second)
		}
		return true, defaultLimitWait
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if strings.Contains(strings.ToLower(string(body)), "rate limit") {
		return true, defaultLimitWait
	}
	return false, 0
}

func capWait(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return time.Second
	case d > maxLimitWait:
		return maxLimitWait
	}
	return d
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
