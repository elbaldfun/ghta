package github

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/importer"
	"github.com/elbaldfun/ghta/internal/item"
)

// viaOldStore is what the importer makes of mapRepo's output after it has been
// stored and read back — the shape every imported item has at the cutover.
func viaOldStore(t *testing.T, n repoNode, at time.Time) (item.Item, *item.Blob) {
	t.Helper()
	old := mapRepo(n)
	old.FetchedAt = at
	raw, err := bson.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var stored domain.TrackedItem
	if err := bson.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	it, blob, notes := importer.Convert(stored, importer.Rules{
		ValidPath: func(string) bool { return true }, ShelfPath: func(s string) string { return s },
	})
	if len(notes.UnknownKeys) > 0 {
		t.Fatalf("the importer does not know sourceData keys %v", notes.UnknownKeys)
	}
	return it, blob
}

// fetcherOwned keeps the fields the fetcher writes, so the two paths can be
// compared without the parts other jobs own.
func fetcherOwned(it item.Item) item.Item {
	return item.Item{
		Source: it.Source, ExternalID: it.ExternalID, Name: it.Name, Description: it.Description,
		Language: it.Language, Homepage: it.Homepage, License: it.License, Topics: it.Topics,
		PrimaryMetric: it.PrimaryMetric, MetricDirection: it.MetricDirection, Metrics: it.Metrics,
		Gh: it.Gh,
	}
}

// The typed output must be what the importer produces from the old output, or
// the first fetch after the cutover would rewrite every item differently.
func TestMapRepoItemMatchesImportedShape(t *testing.T) {
	at := time.Date(2026, 10, 13, 3, 30, 0, 0, time.UTC)
	pushed := time.Date(2026, 10, 12, 8, 0, 0, 0, time.FixedZone("x", 8*3600))
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	var full repoNode
	full.Name, full.Owner.Login = "app", "acme"
	full.Description, full.URL, full.HomepageURL = "an app", "https://github.com/acme/app", "https://acme.dev"
	full.StargazerCount, full.ForkCount, full.Issues.TotalCount = 4200, 12, 3
	full.PushedAt = &pushed
	full.PrimaryLanguage = &struct{ Name string }{"Rust"}
	full.LicenseInfo = &struct{ Name string }{"MIT License"}
	full.Readme = &struct{ Text string }{"# App"}
	full.RepositoryTopics.Edges = make([]struct {
		Node struct {
			Topic struct{ Name string } `json:"topic"`
			URL   string                `json:"url"`
		} `json:"node"`
	}, 2)
	full.RepositoryTopics.Edges[0].Node.Topic.Name = "desktop-app"
	full.RepositoryTopics.Edges[1].Node.Topic.Name = "macos"
	full.Releases.Edges = make([]struct {
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
	}, 2)
	full.Releases.Edges[0].Node.Name, full.Releases.Edges[0].Node.TagName = "next", "v2.0.0-rc1"
	full.Releases.Edges[0].Node.IsDraft = true
	rel := &full.Releases.Edges[1].Node
	rel.Name, rel.TagName, rel.IsLatest, rel.PublishedAt = "1.4", "v1.4.0", true, &published
	rel.ReleaseAssets.Nodes = []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"downloadUrl"`
		Size        int64  `json:"size"`
	}{
		{"app-1.4.0-arm64.dmg", "https://example.com/a.dmg", 1 << 20},
		{"app-1.4.0-setup.exe", "https://example.com/a.exe", 2 << 20},
		{"checksums.sha256", "https://example.com/sums", 100},
	}

	var bare repoNode // no language, license, readme, releases or topics
	bare.Name, bare.Owner.Login, bare.StargazerCount = "lib", "solo", 1500

	for name, n := range map[string]repoNode{"full": full, "bare": bare} {
		t.Run(name, func(t *testing.T) {
			wantItem, wantBlob := viaOldStore(t, n, at)
			got := mapRepoItem(n, at)

			if w, g := fetcherOwned(wantItem), fetcherOwned(got.Item); !reflect.DeepEqual(w, g) {
				t.Errorf("item differs\n imported: %+v gh=%+v\n fetched:  %+v gh=%+v", w, w.Gh, g, g.Gh)
			}
			if !reflect.DeepEqual(wantBlob, got.Blob) {
				t.Errorf("blob differs\n imported: %+v\n fetched:  %+v", wantBlob, got.Blob)
			}
		})
	}

	got := mapRepoItem(full, at)
	if len(got.Item.Gh.Platforms) == 0 || got.Item.Gh.LatestRelease == nil || got.Item.Gh.LatestRelease.Tag != "v1.4.0" ||
		len(got.Item.Gh.ReleaseAssets) != 2 || got.Blob == nil || len(got.Blob.Releases) != 2 {
		t.Errorf("the sample does not exercise releases and platforms: gh=%+v blob=%+v", got.Item.Gh, got.Blob)
	}
	if mapRepoItem(bare, at).Blob != nil {
		t.Error("a repo without README or releases must have no blob")
	}
}
