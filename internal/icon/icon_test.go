package icon

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parse(t *testing.T, s string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

func TestBestIconHref(t *testing.T) {
	// apple-touch-icon wins over a plain favicon.
	doc := parse(t, `<head>
		<link rel="icon" href="/favicon.ico">
		<link rel="apple-touch-icon" href="/apple-touch-icon.png">
	</head>`)
	if got := bestIconHref(doc); got != "/apple-touch-icon.png" {
		t.Errorf("got %q, want apple-touch-icon", got)
	}

	// Among plain icons, the largest declared size wins.
	doc = parse(t, `<head>
		<link rel="icon" sizes="16x16" href="/small.png">
		<link rel="icon" sizes="512x512" href="/large.png">
	</head>`)
	if got := bestIconHref(doc); got != "/large.png" {
		t.Errorf("got %q, want /large.png", got)
	}

	// No icon links → empty.
	if got := bestIconHref(parse(t, `<head><title>x</title></head>`)); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSafeURL(t *testing.T) {
	ok := []string{"https://obsidian.md", "http://example.com/app", "logseq.com"}
	for _, u := range ok {
		if _, valid := safeURL(u); !valid {
			t.Errorf("safeURL(%q) = invalid, want valid", u)
		}
	}
	// SSRF / junk must be rejected.
	bad := []string{
		"", "ftp://example.com", "http://localhost/x", "http://127.0.0.1",
		"http://169.254.169.254/latest/meta-data", "http://10.0.0.5", "http://192.168.1.1",
		"http://example.com:8080", "javascript:alert(1)",
	}
	for _, u := range bad {
		if _, valid := safeURL(u); valid {
			t.Errorf("safeURL(%q) = valid, want rejected", u)
		}
	}
}
