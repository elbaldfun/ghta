package github

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
)

// Adapter implements source.Fetcher for GitHub repositories.
type Adapter struct {
	client   *Client
	pageSize int
	log      *slog.Logger
}

func NewAdapter(token string, rateBuffer, pageSize int, log *slog.Logger) *Adapter {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}
	return &Adapter{
		client:   NewClient(token, rateBuffer, log),
		pageSize: pageSize,
		log:      log,
	}
}

func (a *Adapter) Source() domain.Source { return domain.SourceGitHub }

// starRanges shard the star space so no single search exceeds GitHub's 1000-result
// cap: higher star counts are rarer (bigger step), lower counts denser (smaller step).
var starRanges = []struct{ start, end, step int }{
	{100000, 800000, 100000},
	{50000, 100000, 2000},
	{30000, 50000, 100},
	{10000, 30000, 50},
	{9000, 10000, 20},
	{8000, 9000, 20},
	{7000, 8000, 10},
	{6000, 7000, 10},
	{5000, 6000, 10},
	{4000, 5000, 10},
	{3000, 4000, 10},
	{2000, 3000, 10},
	{1000, 2000, 10},
}

// Shards returns star-range query fragments like "1000..2000", high to low.
func (a *Adapter) Shards() []string {
	var shards []string
	for _, r := range starRanges {
		for i := r.end; i > r.start; i -= r.step {
			shards = append(shards, fmt.Sprintf("%d..%d", i-r.step, i))
		}
	}
	return shards
}

// Fetch pages through one star range and normalizes every repository.
func (a *Adapter) Fetch(ctx context.Context, shard string) ([]domain.TrackedItem, error) {
	query := "stars:" + shard
	var items []domain.TrackedItem
	after := ""

	for {
		resp, err := a.client.search(ctx, query, a.pageSize, after)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", shard, err)
		}
		search := resp.Data.Search
		if search.PageInfo.StartCursor == nil {
			break // no data for this range
		}
		for _, edge := range search.Edges {
			items = append(items, mapRepo(edge.Node))
		}
		if !search.PageInfo.HasNextPage || search.PageInfo.EndCursor == nil {
			break
		}
		after = *search.PageInfo.EndCursor

		if err := sleep(ctx, 300*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return items, nil
}

type repoNode struct {
	Name        string                 `json:"name"`
	Owner       struct{ Login string } `json:"owner"`
	Description string                 `json:"description"`
	URL         string                 `json:"url"`
	HomepageURL string                 `json:"homepageUrl"`

	StargazerCount int        `json:"stargazerCount"`
	ForkCount      int        `json:"forkCount"`
	PushedAt       *time.Time `json:"pushedAt"`

	PrimaryLanguage *struct{ Name string }   `json:"primaryLanguage"`
	Issues          struct{ TotalCount int } `json:"issues"`
	LicenseInfo     *struct{ Name string }   `json:"licenseInfo"`

	RepositoryTopics struct {
		Edges []struct {
			Node struct {
				Topic struct{ Name string } `json:"topic"`
				URL   string                `json:"url"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"repositoryTopics"`

	Releases struct {
		Edges []struct {
			Node struct {
				Name         string     `json:"name"`
				TagName      string     `json:"tagName"`
				IsPrerelease bool       `json:"isPrerelease"`
				IsLatest     bool       `json:"isLatest"`
				IsDraft      bool       `json:"isDraft"`
				PublishedAt  *time.Time `json:"publishedAt"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"releases"`

	Readme *struct{ Text string } `json:"readme"`
}

// mapRepo normalizes a GitHub repository into a source-agnostic TrackedItem,
// keeping GitHub-specific fields (topics, releases, readme, ...) under sourceData.
func mapRepo(n repoNode) domain.TrackedItem {
	externalID := n.Owner.Login + "/" + n.Name

	language := ""
	if n.PrimaryLanguage != nil {
		language = n.PrimaryLanguage.Name
	}
	license := ""
	if n.LicenseInfo != nil {
		license = n.LicenseInfo.Name
	}
	readme := ""
	if n.Readme != nil {
		readme = n.Readme.Text
	}

	topics := make([]map[string]string, 0, len(n.RepositoryTopics.Edges))
	topicNames := make([]string, 0, len(n.RepositoryTopics.Edges))
	for _, e := range n.RepositoryTopics.Edges {
		topics = append(topics, map[string]string{"name": e.Node.Topic.Name, "url": e.Node.URL})
		topicNames = append(topicNames, e.Node.Topic.Name)
	}

	releases := make([]map[string]any, 0, len(n.Releases.Edges))
	for _, e := range n.Releases.Edges {
		releases = append(releases, map[string]any{
			"name":         e.Node.Name,
			"tagName":      e.Node.TagName,
			"isPrerelease": e.Node.IsPrerelease,
			"isLatest":     e.Node.IsLatest,
			"isDraft":      e.Node.IsDraft,
			"publishedAt":  e.Node.PublishedAt,
		})
	}

	return domain.TrackedItem{
		Source:          domain.SourceGitHub,
		ExternalID:      externalID,
		Name:            n.Name,
		Description:     n.Description,
		Language:        language,
		PrimaryMetric:   "stars",
		MetricDirection: domain.DirectionDescBetter,
		Metrics: map[string]float64{
			"stars":      float64(n.StargazerCount),
			"forks":      float64(n.ForkCount),
			"openIssues": float64(n.Issues.TotalCount),
		},
		SourceData: map[string]any{
			"owner":       n.Owner.Login,
			"url":         n.URL,
			"homepageUrl": n.HomepageURL,
			"license":     license,
			"pushedAt":    n.PushedAt,
			"topics":      topics,
			"topicNames":  topicNames,
			"releases":    releases,
			"readme":      readme,
		},
	}
}
