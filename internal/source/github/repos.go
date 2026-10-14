package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrRepoNotFound signals a 404 from /repos/{owner}/{repo}: the repository was
// deleted or made private. Renames do NOT 404 — GitHub 301-redirects them and
// the HTTP client follows, so a renamed repo comes back 200 under its new
// full_name (callers detect the rename by comparing FullName to what they
// asked for).
var ErrRepoNotFound = errors.New("github repo not found")

// ErrRepoBlocked signals a repository GitHub has taken down: 451 for a legal
// takedown, or 403 carrying a "block" object for a terms-of-service block. It
// still exists and still appears in search, but will not come back, so callers
// retire it instead of retrying.
var ErrRepoBlocked = errors.New("github repo blocked")

// RepoInfo is the subset of GitHub's /repos/{owner}/{repo} REST response the
// reconciler needs.
type RepoInfo struct {
	ID         int64  `json:"id"`
	FullName   string `json:"full_name"`
	Stars      int    `json:"stargazers_count"`
	Forks      int    `json:"forks_count"`
	OpenIssues int    `json:"open_issues_count"`
	Archived   bool   `json:"archived"`
	Disabled   bool   `json:"disabled"`
}

// FetchRepo retrieves one repository via the REST API, following rename
// redirects. Returns ErrRepoNotFound on 404, ErrRepoBlocked for a takedown and
// a rate-limit error on any other 403 and on 429;
// RateInfo is populated whenever the headers are present, even on error.
func (c *Client) FetchRepo(ctx context.Context, fullName string) (*RepoInfo, RateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restBase+"/repos/"+fullName, nil)
	if err != nil {
		return nil, RateInfo{Remaining: -1}, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, RateInfo{Remaining: -1}, err
	}
	defer resp.Body.Close()

	rate := parseRate(resp.Header)
	switch resp.StatusCode {
	case http.StatusOK:
		var r RepoInfo
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, rate, fmt.Errorf("decode repo %s: %w", fullName, err)
		}
		return &r, rate, nil
	case http.StatusNotFound:
		return nil, rate, ErrRepoNotFound
	case http.StatusUnavailableForLegalReasons:
		return nil, rate, ErrRepoBlocked
	case http.StatusForbidden, http.StatusTooManyRequests:
		// A blocked repository answers 403 like a rate limit does; the body
		// tells them apart.
		var body struct {
			Block *json.RawMessage `json:"block"`
		}
		if resp.StatusCode == http.StatusForbidden && json.NewDecoder(resp.Body).Decode(&body) == nil && body.Block != nil {
			return nil, rate, ErrRepoBlocked
		}
		return nil, rate, fmt.Errorf("github rate limited fetching %s: status %d", fullName, resp.StatusCode)
	default:
		return nil, rate, fmt.Errorf("github repo %s: status %d", fullName, resp.StatusCode)
	}
}

// FetchRepo delegates to the underlying client.
func (a *Adapter) FetchRepo(ctx context.Context, fullName string) (*RepoInfo, RateInfo, error) {
	return a.client.FetchRepo(ctx, fullName)
}
