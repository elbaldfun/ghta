// Package shot extracts an app's screenshot: the first real UI image in its
// README (the main path — apps almost always lead with one), with the
// homepage's og:image as fallback. Heuristics err toward rejecting — a wrong
// "screenshot" (badge wall, sponsor banner, star chart) is worse than none.
package shot

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/elbaldfun/ghta/internal/icon"
)

// candidate limits: only the first few images matter — a real screenshot sits
// near the top; deep images are gallery/footer noise.
const maxCandidates = 6

// noiseDomains host badges, charts, and counters — never screenshots.
var noiseDomains = []string{
	"shields.io", "badgen.net", "badge.fury.io", "travis-ci", "circleci.com",
	"codecov.io", "coveralls.io", "opencollective.com", "star-history.com",
	"github-readme-stats", "visitor-badge", "hits.seeyoufarm", "sonarcloud.io",
	"snyk.io", "deepsource.io", "codacy.com", "codeclimate.com", "gitpod.io",
	"vercel.com/button", "heroku.com/deploy", "colab.research.google.com",
	"discord.com/api", "discordapp.com/api", "img.badgesize",
}

// noiseWords in a URL path mark logos, badges, and decorations.
var noiseWords = []string{
	"badge", "logo", "icon", "sponsor", "banner", "shield", "button",
	"download-on", "app-store", "google-play", "playstore", "f-droid",
	"contributors", "stargazers", "forkers", "trend",
	// Round-1 audit failures: benchmark charts, social/OG marketing cards,
	// sponsor walls, architecture diagrams — images, not screenshots.
	"graph", "chart", "benchmark", "social", "opengraph", "og-image",
	"companies", "sponsors", "backers", "diagram", "architecture",
}

var mdImg = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)
var htmlImg = regexp.MustCompile(`<img[^>]+src=["']?([^"'>\s]+)`)

// Candidates returns, in document order, the README image URLs that survive the
// URL-level filters, resolved to absolute raw URLs.
func Candidates(readme, externalID string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(raw string) {
		if len(out) >= maxCandidates {
			return
		}
		u := resolve(strings.TrimSpace(raw), externalID)
		if u == "" || seen[u] || !urlLooksLikeShot(u) {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	// Preserve document order across both syntaxes by scanning line by line.
	for _, line := range strings.Split(readme, "\n") {
		for _, m := range mdImg.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
		for _, m := range htmlImg.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
		if len(out) >= maxCandidates {
			break
		}
	}
	return out
}

// resolve makes a README image reference absolute. Relative paths point into
// the repo at HEAD (branch-agnostic).
func resolve(raw, externalID string) string {
	raw = strings.TrimPrefix(raw, "<")
	raw = strings.TrimSuffix(raw, ">")
	if raw == "" || strings.HasPrefix(raw, "data:") {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	raw = strings.TrimPrefix(raw, "./")
	raw = strings.TrimPrefix(raw, "/")
	return "https://raw.githubusercontent.com/" + externalID + "/HEAD/" + raw
}

// urlLooksLikeShot rejects candidates on URL evidence alone: badge hosts, noise
// words, and svg (logos/badges; real screenshots are raster).
func urlLooksLikeShot(u string) bool {
	lower := strings.ToLower(u)
	if strings.Contains(lower, ".svg") {
		return false
	}
	for _, d := range noiseDomains {
		if strings.Contains(lower, d) {
			return false
		}
	}
	// Only inspect the path, not the query, to avoid false hits in tokens.
	path := lower
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	for _, w := range noiseWords {
		if strings.Contains(path, w) {
			return false
		}
	}
	return true
}

// dims holds a verified image's pixel size.
type dims struct{ w, h int }

// Verify downloads just enough of the image to decode its dimensions and
// applies the shape filters: wide banners (w/h > 3) and small graphics
// (w < 400 or h < 220) are not screenshots. Undecodable data is rejected —
// conservative by design (review M1).
func Verify(ctx context.Context, hc *http.Client, url string) bool {
	d, err := fetchDims(ctx, hc, url)
	if err != nil {
		return false
	}
	if d.w < 400 || d.h < 220 {
		return false
	}
	if float64(d.w)/float64(d.h) > 3.0 {
		return false
	}
	return true
}

func fetchDims(ctx context.Context, hc *http.Client, url string) (dims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return dims{}, err
	}
	req.Header.Set("User-Agent", "ghta-shot/1.0")
	resp, err := hc.Do(req)
	if err != nil {
		return dims{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dims{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	// Image headers carry the dimensions in the first bytes; 256KB covers even
	// progressive JPEGs while keeping the fetch bounded.
	cfg, _, err := image.DecodeConfig(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return dims{}, err
	}
	return dims{w: cfg.Width, h: cfg.Height}, nil
}

var ogImg = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:image(?::secure_url)?["'][^>]+content=["']([^"']+)`)
var ogImgRev = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image`)

// OGImage fetches homepage and returns its og:image URL, or "".
func OGImage(ctx context.Context, hc *http.Client, homepage string) string {
	base, ok := icon.SafeURL(homepage)
	if !ok {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "ghta-shot/1.0")
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	head, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if m := ogImg.FindSubmatch(head); m != nil {
		if u := absolutize(string(m[1]), base.Scheme, base.Host); urlLooksLikeShot(u) {
			return u
		}
	}
	if m := ogImgRev.FindSubmatch(head); m != nil {
		if u := absolutize(string(m[1]), base.Scheme, base.Host); urlLooksLikeShot(u) {
			return u
		}
	}
	return ""
}

func absolutize(u, scheme, host string) string {
	switch {
	case strings.HasPrefix(u, "http://"):
		// Mixed content: an http image on an https page never renders (L2).
		return ""
	case strings.HasPrefix(u, "https://"):
		return u
	case strings.HasPrefix(u, "//"):
		return "https:" + u
	case strings.HasPrefix(u, "/"):
		return scheme + "://" + host + u
	}
	return ""
}
