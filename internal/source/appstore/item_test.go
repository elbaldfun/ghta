package appstore

import (
	"strings"
	"testing"

	"github.com/elbaldfun/ghta/internal/item"
)

func TestToFetched(t *testing.T) {
	a := App{
		Kind: "software", TrackID: 1440969370, BundleID: "md.obsidian", TrackName: "Obsidian",
		Description: strings.Repeat("笔记", 400), ArtistID: 1440969369, ArtistName: "Dynalist Inc.",
		SellerURL: "https://obsidian.md", PrimaryGenreID: 6007, PrimaryGenreName: "Productivity",
		GenreIDs: []string{"6007", "6000"}, Price: 0, Currency: "USD",
		AverageUserRating: 4.6, UserRatingCount: 2831, Version: "1.10.3",
		CurrentVersionReleaseDate: "2026-11-18T03:04:27Z", ReleaseDate: "2020-03-01T08:00:00Z",
		ArtworkURL512:  "https://is1-ssl.mzstatic.com/icon512.png",
		ScreenshotURLs: []string{"s1", "s2", "s3", "s4"},
	}
	f := ToFetched(a, Listing{HomeCountry: "us", Countries: []string{"us", "jp"}, BestRank: 12}, nil)
	it := f.Item
	if it.Source != item.SourceIOS || it.ExternalID != "1440969370" || it.PrimaryMetric != MetricRatings {
		t.Fatalf("identity %+v", it)
	}
	if it.Metrics[MetricRatings] != 2831 || it.Metrics[MetricBestRank] != 12 || it.Metrics[MetricRating] != 4.6 {
		t.Errorf("metrics %v", it.Metrics)
	}
	if len(it.Labels.Domain) != 1 || it.Labels.Domain[0] != "ios/productivity" || it.Labels.By != item.BySource || it.Labels.Shelf != "" {
		t.Errorf("labels %+v", it.Labels)
	}
	if len(it.Description) > descriptionMax+len("…") || !strings.HasSuffix(it.Description, "…") {
		t.Errorf("description of %d bytes", len(it.Description))
	}
	if f.Blob == nil || f.Blob.Readme != a.Description {
		t.Error("full description not in the blob")
	}
	if it.App.HomeCountry != "us" || len(it.App.Screenshots) != 3 || it.App.DeveloperID != "1440969369" || it.App.UpdatedAt.IsZero() {
		t.Errorf("app %+v", it.App)
	}
	if it.Assets.IconURL != a.ArtworkURL512 || it.Assets.ScreenshotURL != "s1" {
		t.Errorf("assets %+v", it.Assets)
	}

	shelved := ToFetched(a, Listing{HomeCountry: "us"}, func(App) string { return "productivity/notes" })
	if shelved.Item.Labels.Shelf != "productivity/notes" {
		t.Errorf("shelver ignored: %q", shelved.Item.Labels.Shelf)
	}
	if _, ok := shelved.Item.Metrics[MetricBestRank]; ok {
		t.Error("unknown best rank stored as a metric")
	}
}

func TestGenres(t *testing.T) {
	if len(Genres) != 25 {
		t.Fatalf("%d genres, want 25 (27 less Stickers and Catalogs)", len(Genres))
	}
	slugs := map[string]bool{}
	for _, g := range Genres {
		if g.ID == "6025" || g.ID == "6022" || slugs[g.Slug] || g.Slug == "" {
			t.Fatalf("bad genre %+v", g)
		}
		slugs[g.Slug] = true
	}
}
