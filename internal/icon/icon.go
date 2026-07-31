// Package icon extracts an app's brand icon from its homepage — the
// apple-touch-icon or a high-res favicon — so the directory shows the app's own
// logo rather than the repo owner's avatar.
package icon

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const maxHTMLBytes = 512 * 1024

// Extract fetches homepage and returns the best absolute icon URL, or "" when
// none is found or the URL is unsafe to fetch. It never validates that the icon
// loads — the frontend falls back to the owner avatar on error — so this stays a
// single request.
func Extract(ctx context.Context, hc *http.Client, homepage string) string {
	base, ok := SafeURL(homepage)
	if !ok {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "ghta-icon/1.0")
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, maxHTMLBytes))
	if err != nil {
		return ""
	}
	// Resolve against the FINAL url after redirects, so relative hrefs are right.
	final := resp.Request.URL
	if href := bestIconHref(doc); href != "" {
		if u, err := final.Parse(href); err == nil {
			return u.String()
		}
	}
	return ""
}

// iconCandidate scores a <link rel=...icon...> by how good an app icon it is.
type iconCandidate struct {
	href  string
	score int
}

// bestIconHref walks the <head> links and returns the highest-scored icon href.
func bestIconHref(doc *html.Node) string {
	best := iconCandidate{score: -1}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			var rel, href, sizes string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "rel":
					rel = strings.ToLower(a.Val)
				case "href":
					href = strings.TrimSpace(a.Val)
				case "sizes":
					sizes = a.Val
				}
			}
			if href != "" && strings.Contains(rel, "icon") {
				s := scoreIcon(rel, sizes)
				if s > best.score {
					best = iconCandidate{href: href, score: s}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return best.href
}

// scoreIcon prefers apple-touch-icon (typically 180px, crisp) and larger sizes.
func scoreIcon(rel, sizes string) int {
	score := 0
	switch {
	case strings.Contains(rel, "apple-touch-icon"):
		score += 1000
	case strings.Contains(rel, "mask-icon"):
		score += 100 // monochrome svg, last resort
	default: // "icon" / "shortcut icon"
		score += 500
	}
	// Add the largest declared dimension (e.g. sizes="512x512").
	for _, part := range strings.FieldsFunc(sizes, func(r rune) bool { return r == ' ' }) {
		if i := strings.IndexAny(part, "xX"); i > 0 {
			if n, err := strconv.Atoi(part[:i]); err == nil && n > 0 {
				score += n
			}
		}
	}
	return score
}

// safeURL parses homepage and rejects anything that isn't a plain public
// http(s) URL — a basic SSRF guard so a repo's homepage field can't point the
// fetcher at cloud metadata (169.254.169.254) or internal hosts.
// SafeURL is exported for sibling fetchers (screenshots) that hit user-supplied
// homepages and need the same SSRF guard.
func SafeURL(homepage string) (*url.URL, bool) {
	homepage = strings.TrimSpace(homepage)
	if homepage == "" {
		return nil, false
	}
	if !strings.Contains(homepage, "://") {
		homepage = "https://" + homepage
	}
	u, err := url.Parse(homepage)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return nil, false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return nil, false
		}
	}
	return u, true
}
