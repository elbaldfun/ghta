package appstore

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/elbaldfun/ghta/internal/item"
)

// Genre is one top-level App Store genre.
type Genre struct {
	ID   string
	Slug string // labels.domain is "ios/<slug>"
}

// Genres are the top-level App Store genres we chart, from
// itunes.apple.com/WebObjects/MZStoreServices.woa/ws/genres?id=36 (read
// 2026-11-21). Left out: Stickers (6025), which are not apps, and Catalogs
// (6022), whose charts were empty in the US on 2026-11-22.
var Genres = []Genre{
	{"6018", "books"}, {"6000", "business"}, {"6026", "developer-tools"},
	{"6017", "education"}, {"6016", "entertainment"}, {"6015", "finance"}, {"6023", "food-drink"},
	{"6014", "games"}, {"6027", "graphics-design"}, {"6013", "health-fitness"}, {"6012", "lifestyle"},
	{"6021", "magazines-newspapers"}, {"6020", "medical"}, {"6011", "music"}, {"6010", "navigation"},
	{"6009", "news"}, {"6008", "photo-video"}, {"6007", "productivity"}, {"6006", "reference"},
	{"6024", "shopping"}, {"6005", "social-networking"}, {"6004", "sports"}, {"6003", "travel"},
	{"6002", "utilities"}, {"6001", "weather"},
}

var genreSlug = func() map[string]string {
	m := make(map[string]string, len(Genres))
	for _, g := range Genres {
		m[g.ID] = g.Slug
	}
	return m
}()

// DomainPrefix namespaces App Store genres in labels.domain, the way "hf/"
// does HuggingFace tasks.
const DomainPrefix = "ios/"

// Metric names of an App Store item.
const (
	MetricRatings  = "ratings"  // rating count of the home storefront: the primary metric
	MetricRating   = "rating"   // average rating, 0–5
	MetricPrice    = "price"    // in the home storefront's currency
	MetricBestRank = "bestRank" // best position on any of today's home-storefront charts
)

// descriptionMax is how much of the store description the item keeps; the
// whole text goes to the blob.
const descriptionMax = 1000

// Shelver maps a listing to a store shelf slug, or "" for none: NewShelver,
// by taxonomy/store-genres.yaml.
type Shelver func(a App) string

// Listing is what the job knows about an app from today's charts.
type Listing struct {
	HomeCountry string
	Countries   []string
	BestRank    int             // in the home storefront; 0 when unknown
	Charts      []item.ChartPos // today's places, every storefront
	Day         string          // the day of Charts
}

// ToFetched normalizes one Lookup result read in l.HomeCountry.
func ToFetched(a App, l Listing, shelve Shelver) item.Fetched {
	id := strconv.FormatInt(a.TrackID, 10)
	genre := strconv.Itoa(a.PrimaryGenreID)
	metrics := map[string]float64{
		MetricRatings: a.UserRatingCount,
		MetricRating:  a.AverageUserRating,
		MetricPrice:   a.Price,
	}
	if l.BestRank > 0 {
		metrics[MetricBestRank] = float64(l.BestRank)
	}
	app := &item.StoreApp{
		BundleID: a.BundleID, Developer: a.ArtistName, GenreID: genre, Genre: a.PrimaryGenreName,
		GenreIDs: a.GenreIDs, Version: a.Version, Currency: a.Currency,
		ContentRating: a.ContentAdvisoryRating, Screenshots: firstN(a.ScreenshotURLs, 3),
		UpdatedAt: parseTime(a.CurrentVersionReleaseDate), ReleasedAt: parseTime(a.ReleaseDate),
		HomeCountry: l.HomeCountry, Countries: l.Countries,
		Charts: l.Charts,
	}
	if len(l.Charts) > 0 {
		app.ChartsDay = l.Day
	}
	if len(app.Screenshots) == 0 {
		app.Screenshots = firstN(a.IPadScreenshotURLs, 3)
	}
	if a.ArtistID != 0 {
		app.DeveloperID = strconv.FormatInt(a.ArtistID, 10)
	}
	it := item.Item{
		Source: item.SourceIOS, ExternalID: id, Name: a.TrackName,
		Description: truncate(a.Description, descriptionMax), Homepage: a.SellerURL, Topics: []string{},
		PrimaryMetric: MetricRatings, MetricDirection: "desc-better", Metrics: metrics,
		App: app,
	}
	if slug, ok := genreSlug[genre]; ok {
		it.Labels = item.Labels{Domain: []string{DomainPrefix + slug}, By: item.BySource}
	} else {
		// A genre outside the charted set (Stickers): still the store's own.
		it.Labels = item.Labels{Domain: []string{DomainPrefix + "genre-" + genre}, By: item.BySource}
	}
	if shelve != nil {
		it.Labels.Shelf = shelve(a)
	}
	it.Assets.IconURL = a.ArtworkURL512
	if it.Assets.IconURL == "" {
		it.Assets.IconURL = a.ArtworkURL100
	}
	if len(app.Screenshots) > 0 {
		it.Assets.ScreenshotURL = app.Screenshots[0]
	}
	f := item.Fetched{Item: it}
	if a.Description != "" {
		f.Blob = item.NewBlob(item.SourceIOS, id, a.Description, nil, time.Time{})
	}
	return f
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// truncate cuts s to at most n bytes on a rune boundary.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut]) + "…"
}
