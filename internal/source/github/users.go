package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const restBase = "https://api.github.com"

// ErrUserNotFound signals a 404 from /users/{login} (renamed/deleted account),
// which the sweep skips rather than treating as a hard failure.
var ErrUserNotFound = errors.New("github user not found")

// UserProfile is the subset of GitHub's /users/{login} REST response we store.
// `type` is "User" or "Organization"; `twitter_username` is the account's own
// self-declared X handle (empty when not set).
type UserProfile struct {
	Login           string    `json:"login"`
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	Company         string    `json:"company"`
	Blog            string    `json:"blog"`
	Location        string    `json:"location"`
	Bio             string    `json:"bio"`
	TwitterUsername string    `json:"twitter_username"`
	Followers       int       `json:"followers"`
	Following       int       `json:"following"`
	PublicRepos     int       `json:"public_repos"`
	AvatarURL       string    `json:"avatar_url"`
	CreatedAt       time.Time `json:"created_at"`
}

// RateInfo carries the REST rate-limit headers so a sweep can pace itself to
// GitHub's 5000-req/hour budget without tripping a 403.
type RateInfo struct {
	Remaining int // -1 when the header was absent
	Reset     time.Time
}

// FetchUser retrieves one account profile via the REST API. Returns
// ErrUserNotFound on 404 and a rate-limit error on 403/429; RateInfo is
// populated whenever the headers are present, even on error.
func (c *Client) FetchUser(ctx context.Context, login string) (*UserProfile, RateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restBase+"/users/"+url.PathEscape(login), nil)
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
		var u UserProfile
		if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
			return nil, rate, fmt.Errorf("decode user %s: %w", login, err)
		}
		return &u, rate, nil
	case http.StatusNotFound:
		return nil, rate, ErrUserNotFound
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, rate, fmt.Errorf("github rate limited fetching %s: status %d", login, resp.StatusCode)
	default:
		return nil, rate, fmt.Errorf("github user %s: status %d", login, resp.StatusCode)
	}
}

// FetchUser delegates to the underlying client.
func (a *Adapter) FetchUser(ctx context.Context, login string) (*UserProfile, RateInfo, error) {
	return a.client.FetchUser(ctx, login)
}

func parseRate(h http.Header) RateInfo {
	ri := RateInfo{Remaining: -1}
	if v := h.Get("X-RateLimit-Remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ri.Remaining = n
		}
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			ri.Reset = time.Unix(n, 0)
		}
	}
	return ri
}
