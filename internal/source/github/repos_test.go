package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type cannedTransport struct {
	status int
	body   string
}

func (c cannedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: c.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(c.body))}, nil
}

// A repository GitHub took down answers 451, or 403 with a "block" object
// (bodies trimmed from real responses). A 403 without one is a rate limit and
// must stay retryable.
func TestFetchRepoTellsBlockedFromRateLimited(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		blocked bool
		gone    bool
	}{
		{"legal takedown", 451, `{"message":"Repository access blocked","block":{"reason":"dmca"}}`, true, false},
		{"terms-of-service block", 403, `{"message":"Repository access blocked","block":{"reason":"tos","html_url":"https://github.com/tos"}}`, true, false},
		{"rate limit", 403, `{"message":"API rate limit exceeded for user ID 1."}`, false, false},
		{"secondary rate limit", 429, `{"message":"You have exceeded a secondary rate limit."}`, false, false},
		{"unreadable 403", 403, `<html>`, false, false},
		{"deleted", 404, `{"message":"Not Found"}`, false, true},
	}
	for _, c := range cases {
		client := NewClient("", 0, nil)
		client.http = &http.Client{Transport: cannedTransport{c.status, c.body}}
		_, _, err := client.FetchRepo(context.Background(), "o/r")
		if err == nil || errors.Is(err, ErrRepoBlocked) != c.blocked || errors.Is(err, ErrRepoNotFound) != c.gone {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}
