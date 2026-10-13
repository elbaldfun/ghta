package github

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
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
	nodes, err := a.fetchNodes(ctx, shard)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TrackedItem, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, mapRepo(n))
	}
	return items, nil
}

// FetchItems is Fetch with typed output, for the rebuilt pipeline.
func (a *Adapter) FetchItems(ctx context.Context, shard string) ([]item.Fetched, error) {
	nodes, err := a.fetchNodes(ctx, shard)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]item.Fetched, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, mapRepoItem(n, now))
	}
	return out, nil
}

// fetchNodes pages through one star range.
func (a *Adapter) fetchNodes(ctx context.Context, shard string) ([]repoNode, error) {
	query := "stars:" + shard
	var nodes []repoNode
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
			nodes = append(nodes, edge.Node)
		}
		if !search.PageInfo.HasNextPage || search.PageInfo.EndCursor == nil {
			break
		}
		after = *search.PageInfo.EndCursor

		if err := sleep(ctx, 300*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// Search runs one search page for an arbitrary query (e.g. "created:>2026-07-17
// sort:stars-desc") and returns up to `limit` mapped repositories. Unlike Fetch
// it does not paginate — it is for small top-N lists, not full ingestion.
func (a *Adapter) Search(ctx context.Context, query string, limit int) ([]domain.TrackedItem, error) {
	if limit < 1 || limit > 100 {
		limit = 30 // GitHub search returns at most 100 per page
	}
	resp, err := a.client.search(ctx, query, limit, "")
	if err != nil {
		return nil, err
	}
	items := make([]domain.TrackedItem, 0, len(resp.Data.Search.Edges))
	for _, edge := range resp.Data.Search.Edges {
		items = append(items, mapRepo(edge.Node))
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
				Name          string     `json:"name"`
				TagName       string     `json:"tagName"`
				IsPrerelease  bool       `json:"isPrerelease"`
				IsLatest      bool       `json:"isLatest"`
				IsDraft       bool       `json:"isDraft"`
				PublishedAt   *time.Time `json:"publishedAt"`
				ReleaseAssets struct {
					Nodes []struct {
						Name        string `json:"name"`
						DownloadURL string `json:"downloadUrl"`
						Size        int64  `json:"size"`
					} `json:"nodes"`
				} `json:"releaseAssets"`
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

	// Platform detection: parse the assets of the release we'd point a user at —
	// the one flagged latest, else the most recent non-draft (edges are
	// CREATED_AT desc). The type-dependent web heuristic is intentionally off here
	// (type isn't known until categorization); the asset + topic layers, which
	// include explicit web topics, are type-independent and refresh every fetch.
	chosen := n.latestRelease()
	var assets []ReleaseAsset
	var latestRelease map[string]any
	if chosen >= 0 {
		nd := n.Releases.Edges[chosen].Node
		for _, a := range nd.ReleaseAssets.Nodes {
			assets = append(assets, ReleaseAsset{Name: a.Name, DownloadURL: a.DownloadURL, Size: a.Size})
		}
		latestRelease = map[string]any{"tag": nd.TagName, "publishedAt": nd.PublishedAt}
	}
	plat := DetectPlatforms(assets, topicNames, "", externalID, n.HomepageURL, language)

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
			"owner":          n.Owner.Login,
			"url":            n.URL,
			"homepageUrl":    n.HomepageURL,
			"license":        license,
			"pushedAt":       n.PushedAt,
			"topics":         topics,
			"topicNames":     topicNames,
			"releases":       releases,
			"readme":         readme,
			"platforms":      platformStrings(plat.Platforms),
			"platformSource": plat.Source,
			"releaseAssets":  plat.Assets,
			"latestRelease":  latestRelease,
		},
	}
}

// latestRelease is the index of the release a user would be pointed at — the
// one flagged latest, else the most recent non-draft (edges are CREATED_AT
// desc) — or -1 when there is none.
func (n repoNode) latestRelease() int {
	chosen := -1
	for i := range n.Releases.Edges {
		nd := n.Releases.Edges[i].Node
		if nd.IsDraft {
			continue
		}
		if nd.IsLatest {
			return i
		}
		if chosen == -1 {
			chosen = i
		}
	}
	return chosen
}

// mapRepoItem is mapRepo for the rebuilt model: the same facts, typed, with
// the README and the release list split off into the blob. Platform detection
// is identical (asset and topic layers; the type-dependent heuristic stays off).
func mapRepoItem(n repoNode, now time.Time) item.Fetched {
	externalID := n.Owner.Login + "/" + n.Name
	it := item.Item{
		Source:          item.SourceGitHub,
		ExternalID:      externalID,
		Name:            n.Name,
		Description:     n.Description,
		Homepage:        n.HomepageURL,
		Topics:          []string{},
		PrimaryMetric:   "stars",
		MetricDirection: string(domain.DirectionDescBetter),
		Metrics: map[string]float64{
			"stars":      float64(n.StargazerCount),
			"forks":      float64(n.ForkCount),
			"openIssues": float64(n.Issues.TotalCount),
		},
		Gh: &item.GitHub{Owner: n.Owner.Login},
	}
	if n.PrimaryLanguage != nil {
		it.Language = n.PrimaryLanguage.Name
	}
	if n.LicenseInfo != nil {
		it.License = n.LicenseInfo.Name
	}
	if n.PushedAt != nil {
		it.Gh.PushedAt = n.PushedAt.UTC()
	}
	for _, e := range n.RepositoryTopics.Edges {
		if name := e.Node.Topic.Name; name != "" {
			it.Topics = append(it.Topics, name)
		}
	}

	var releases []item.Release
	for _, e := range n.Releases.Edges {
		r := item.Release{
			Name: e.Node.Name, TagName: e.Node.TagName,
			IsPrerelease: e.Node.IsPrerelease, IsLatest: e.Node.IsLatest, IsDraft: e.Node.IsDraft,
		}
		if e.Node.PublishedAt != nil {
			r.PublishedAt = e.Node.PublishedAt.UTC()
		}
		releases = append(releases, r)
	}

	var assets []ReleaseAsset
	if chosen := n.latestRelease(); chosen >= 0 {
		nd := n.Releases.Edges[chosen].Node
		for _, a := range nd.ReleaseAssets.Nodes {
			assets = append(assets, ReleaseAsset{Name: a.Name, DownloadURL: a.DownloadURL, Size: a.Size})
		}
		if nd.TagName != "" {
			it.Gh.LatestRelease = &item.ReleaseRef{Tag: nd.TagName}
			if nd.PublishedAt != nil {
				it.Gh.LatestRelease.PublishedAt = nd.PublishedAt.UTC()
			}
		}
	}
	plat := DetectPlatforms(assets, it.Topics, "", externalID, n.HomepageURL, it.Language)
	for _, p := range plat.Platforms {
		it.Gh.Platforms = append(it.Gh.Platforms, string(p))
	}
	it.Gh.PlatformSource = plat.Source
	for _, a := range plat.Assets {
		if a.Name != "" {
			it.Gh.ReleaseAssets = append(it.Gh.ReleaseAssets, item.ReleaseAsset{
				Name: a.Name, Platform: string(a.Platform), URL: a.URL, Size: a.Size,
			})
		}
	}
	return item.Fetched{Item: it, Blob: item.NewBlob(it.Source, externalID, readmeOf(n), releases, now)}
}

func readmeOf(n repoNode) string {
	if n.Readme == nil {
		return ""
	}
	return n.Readme.Text
}

// platformStrings converts the detected platform set to plain strings for
// storage and querying.
func platformStrings(ps []Platform) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}
