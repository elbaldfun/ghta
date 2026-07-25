package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	trendingBaseURL = "https://github.com/trending"
	// A browser User-Agent; github.com/trending serves a stripped page (or 403) to
	// obviously-scripted clients with no UA.
	trendingUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
)

// trendingWindows is the set of periods GitHub's trending page accepts.
var trendingWindows = map[string]bool{"daily": true, "weekly": true, "monthly": true}

// TrendingRepo is one row of GitHub's trending page: a repository ranked by the
// stars it gained during the selected window. GitHub exposes this ranking only
// on the HTML page (github.com/trending) — there is no API for it — so we scrape.
type TrendingRepo struct {
	ExternalID      string `json:"externalId"` // "owner/name"
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Language        string `json:"language,omitempty"`
	URL             string `json:"url"`
	Stars           int    `json:"stars"`           // total stargazers
	Forks           int    `json:"forks"`           // total forks
	StarsThisPeriod int    `json:"starsThisPeriod"` // stars gained during the window
}

// FetchTrending scrapes github.com/trending for the given window (since =
// daily|weekly|monthly, default weekly) and optional programming language slug
// (e.g. "go", "python"; "" = all languages). It returns the repositories in the
// page's rank order.
func FetchTrending(ctx context.Context, hc *http.Client, since, language string) ([]TrendingRepo, error) {
	if since == "" {
		since = "weekly"
	}
	if !trendingWindows[since] {
		return nil, fmt.Errorf("trending: since must be daily|weekly|monthly, got %q", since)
	}

	target := trendingBaseURL
	if language != "" {
		target += "/" + url.PathEscape(language)
	}
	target += "?since=" + since

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", trendingUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("trending: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trending: unexpected status %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("trending: parse html: %w", err)
	}

	repos := parseTrending(doc)
	if len(repos) == 0 {
		// The page loaded but nothing matched our selectors: GitHub changed the
		// markup. Fail loudly rather than silently serving an empty list.
		return nil, fmt.Errorf("trending: parsed 0 repositories (page markup may have changed)")
	}
	return repos, nil
}

// starsPeriodRe pulls "17,401" out of "17,401 stars this week" / "... today".
var starsPeriodRe = regexp.MustCompile(`([\d,]+)\s+stars?\s+(?:today|this week|this month)`)

// parseTrending walks the parsed document and extracts one TrendingRepo per
// `article.Box-row`. Rows missing an owner/name link are skipped.
func parseTrending(doc *html.Node) []TrendingRepo {
	var repos []TrendingRepo
	for _, art := range findAll(doc, func(n *html.Node) bool {
		return n.Data == "article" && hasClass(n, "Box-row")
	}) {
		repo, ok := parseTrendingRow(art)
		if ok {
			repos = append(repos, repo)
		}
	}
	return repos
}

// parseTrendingRow extracts one repository from a Box-row article node.
func parseTrendingRow(art *html.Node) (TrendingRepo, bool) {
	// Owner/name: the <h2>'s anchor href is exactly "/owner/name".
	h2 := findFirst(art, func(n *html.Node) bool { return n.Data == "h2" })
	if h2 == nil {
		return TrendingRepo{}, false
	}
	anchor := findFirst(h2, func(n *html.Node) bool { return n.Data == "a" && attr(n, "href") != "" })
	if anchor == nil {
		return TrendingRepo{}, false
	}
	slug := strings.Trim(attr(anchor, "href"), "/")
	owner, name, found := strings.Cut(slug, "/")
	if !found || owner == "" || name == "" {
		return TrendingRepo{}, false
	}

	repo := TrendingRepo{
		ExternalID: owner + "/" + name,
		Owner:      owner,
		Name:       name,
		URL:        "https://github.com/" + owner + "/" + name,
	}

	if p := findFirst(art, func(n *html.Node) bool { return n.Data == "p" }); p != nil {
		repo.Description = squish(text(p))
	}
	if langNode := findFirst(art, func(n *html.Node) bool {
		return n.Data == "span" && attr(n, "itemprop") == "programmingLanguage"
	}); langNode != nil {
		repo.Language = squish(text(langNode))
	}
	if a := findFirst(art, func(n *html.Node) bool {
		return n.Data == "a" && strings.HasSuffix(attr(n, "href"), "/stargazers")
	}); a != nil {
		repo.Stars = parseCount(text(a))
	}
	if a := findFirst(art, func(n *html.Node) bool {
		return n.Data == "a" && strings.HasSuffix(attr(n, "href"), "/forks")
	}); a != nil {
		repo.Forks = parseCount(text(a))
	}
	if m := starsPeriodRe.FindStringSubmatch(text(art)); m != nil {
		repo.StarsThisPeriod = parseCount(m[1])
	}
	return repo, true
}

// --- tiny html helpers (avoid a goquery dependency for one page) ---

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func findFirst(root *html.Node, pred func(*html.Node) bool) *html.Node {
	var out *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if out != nil {
			return
		}
		if n.Type == html.ElementNode && pred(n) {
			out = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func findAll(root *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && pred(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

// text returns the concatenated text content of a node's subtree.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// squish collapses internal whitespace runs to single spaces and trims.
func squish(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// parseCount turns "19,133" (or "1.2k"-free plain counts GitHub uses) into an int.
func parseCount(s string) int {
	var digits strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	n, _ := strconv.Atoi(digits.String())
	return n
}

// NewTrendingClient returns an http.Client suited to scraping the trending page:
// a modest timeout, no auth. Kept separate from the GraphQL client so trending
// never contends with the API rate limiter.
func NewTrendingClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}
