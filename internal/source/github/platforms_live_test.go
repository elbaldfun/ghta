package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestDetectPlatformsLive runs the platform detector against a set of real,
// well-known open-source apps — fetching their latest release assets, topics,
// homepage, and language live from the GitHub REST API — and prints the result.
// Skipped unless PLATFORMS_LIVE=1 so CI stays offline. It is a demonstration /
// smoke check, not an assertion: `go test -tags ” -run Live -v` shows what the
// directory would actually detect.
func TestDetectPlatformsLive(t *testing.T) {
	if os.Getenv("PLATFORMS_LIVE") == "" {
		t.Skip("set PLATFORMS_LIVE=1 to run the live platform-detection demo")
	}
	repos := []string{
		"clash-verge-rev/clash-verge-rev", "localsend/localsend", "laurent22/joplin",
		"Genymobile/scrcpy", "microsoft/vscode", "obsidianmd/obsidian-releases",
		"logseq/logseq", "mjmlio/mjml", "signalapp/Signal-Desktop", "zen-browser/desktop",
		"mtkennerly/ludusavi", "jgraph/drawio-desktop", "AppFlowy-IO/AppFlowy",
		"gethomepage/homepage", "immich-app/immich",
	}
	tok := os.Getenv("GITHUB_API_TOKEN")
	hc := &http.Client{Timeout: 15 * time.Second}

	fmt.Printf("\n%-38s %-28s %s\n", "REPO", "PLATFORMS", "SOURCE")
	fmt.Println("---------------------------------------------------------------------------------------------")
	for _, repo := range repos {
		meta := ghGet(t, hc, tok, "https://api.github.com/repos/"+repo)
		rel := ghGet(t, hc, tok, "https://api.github.com/repos/"+repo+"/releases/latest")

		var assets []ReleaseAsset
		if raw, ok := rel["assets"].([]any); ok {
			for _, a := range raw {
				m, _ := a.(map[string]any)
				if m == nil {
					continue
				}
				name, _ := m["name"].(string)
				url, _ := m["browser_download_url"].(string)
				size, _ := m["size"].(float64)
				assets = append(assets, ReleaseAsset{Name: name, DownloadURL: url, Size: int64(size)})
			}
		}
		var topics []string
		if raw, ok := meta["topics"].([]any); ok {
			for _, tp := range raw {
				if s, ok := tp.(string); ok {
					topics = append(topics, s)
				}
			}
		}
		homepage, _ := meta["homepage"].(string)
		language, _ := meta["language"].(string)
		// The real classifier decides type; for the demo assume app-like.
		res := DetectPlatforms(assets, topics, "app", repo, homepage, language)

		src := res.Source
		if src == "" {
			src = "(none)"
		}
		fmt.Printf("%-38s %-28s %s  [%d assets, %d topics]\n",
			repo, joinPlatforms(res.Platforms), src, len(assets), len(topics))
	}
}

func joinPlatforms(ps []Platform) string {
	if len(ps) == 0 {
		return "—"
	}
	out := ""
	for i, p := range ps {
		if i > 0 {
			out += " "
		}
		out += string(p)
	}
	return out
}

func ghGet(t *testing.T, hc *http.Client, token, url string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}
