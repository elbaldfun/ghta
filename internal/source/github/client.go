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
	Errors []graphqlError `json:"errors"`
}

// repoLightFields are the repository fields every daily fetch reads: the
// metrics and the descriptive fields, all cheap scalars. repoDetailFields are
// the expensive part — the README blob and the release list with its assets —
// which only change when the repository is pushed to (see Adapter.FetchItems).
const repoLightFields = `
          id
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
          repositoryTopics(first: 20) { edges { node { topic { name } url } } }`

const repoDetailFields = `
          releases(first: 5, orderBy: {field: CREATED_AT, direction: DESC}) {
            edges { node {
              name tagName isPrerelease isLatest isDraft publishedAt
              releaseAssets(first: 40) { nodes { name downloadUrl size } }
            } }
          }
          readme: object(expression: "HEAD:README.md") { ... on Blob { text } }`

func searchQueryWith(fields string) string {
	return `
query($q: String!, $first: Int!, $after: String) {
  rateLimit { remaining resetAt cost }
  search(query: $q, type: REPOSITORY, first: $first, after: $after) {
    pageInfo { hasNextPage endCursor startCursor }
    edges {
      node {
        ... on Repository {` + fields + `
        }
      }
    }
  }
}`
}

var (
	// searchQuery reads everything about each repository found.
	searchQuery = searchQueryWith(repoLightFields + repoDetailFields)
	// lightSearchQuery reads the daily fields only: about a quarter of the
	// time per repository.
	lightSearchQuery = searchQueryWith(repoLightFields)
	// nodesQuery reads everything about the repositories given by node id.
	nodesQuery = `
query($ids: [ID!]!) {
  rateLimit { remaining resetAt cost }
  nodes(ids: $ids) {
    ... on Repository {` + repoLightFields + repoDetailFields + `
    }
  }
}`
)

type nodesResponse struct {
	Data struct {
		RateLimit rateLimit   `json:"rateLimit"`
		Nodes     []*repoNode `json:"nodes"`
	} `json:"data"`
	Errors []graphqlError `json:"errors"`
}

type graphqlError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// graphqlResult is a decoded answer: its errors and the rate limit it reports.
type graphqlResult interface {
	errs() []graphqlError
	rate() rateLimit
}

func (r *searchResponse) errs() []graphqlError { return r.Errors }
func (r *searchResponse) rate() rateLimit      { return r.Data.RateLimit }
func (r *nodesResponse) errs() []graphqlError  { return r.Errors }
func (r *nodesResponse) rate() rateLimit       { return r.Data.RateLimit }

// search runs one search page with the full repository fields.
func (c *Client) search(ctx context.Context, query string, first int, after string) (*searchResponse, error) {
	return c.searchWith(ctx, searchQuery, query, first, after)
}

// searchLight runs one search page with the daily fields only.
func (c *Client) searchLight(ctx context.Context, query string, first int, after string) (*searchResponse, error) {
	return c.searchWith(ctx, lightSearchQuery, query, first, after)
}

func (c *Client) searchWith(ctx context.Context, gql, query string, first int, after string) (*searchResponse, error) {
	vars := map[string]any{"q": query, "first": first}
	if after != "" {
		vars["after"] = after
	}
	var out searchResponse
	if err := c.do(ctx, gql, vars, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// nodes reads the full fields of the repositories with the given node ids, in
// order; an id that no longer resolves (deleted, made private) is nil.
// GraphQL reports such an id as a NOT_FOUND error next to the data, which is
// not a failure here.
func (c *Client) nodes(ctx context.Context, ids []string) ([]*repoNode, error) {
	var out nodesResponse
	if err := c.do(ctx, nodesQuery, map[string]any{"ids": ids}, &out); err != nil {
		return nil, err
	}
	return out.Data.Nodes, nil
}

// do posts one GraphQL query, honoring rate limits and retrying transient
// failures with exponential backoff.
func (c *Client) do(ctx context.Context, gql string, vars map[string]any, out graphqlResult) error {
	body, err := json.Marshal(map[string]any{"query": gql, "variables": vars})
	if err != nil {
		return err
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
				return err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return err
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
				return fmt.Errorf("github http %d (non-retryable)", resp.StatusCode)
			}
		}

		dec := json.NewDecoder(resp.Body)
		decErr := dec.Decode(out)
		resp.Body.Close()
		if decErr != nil {
			lastErr = decErr
			continue
		}
		_, byID := out.(*nodesResponse)
		for _, e := range out.errs() {
			if !byID || e.Type != "NOT_FOUND" {
				return fmt.Errorf("graphql error: %s", e.Message)
			}
		}

		c.waitIfRateLimited(ctx, out.rate())
		return nil
	}
	return fmt.Errorf("github query failed after %d attempts: %w", maxRetries, lastErr)
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
