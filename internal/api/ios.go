package api

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/appstore"
)

// The App Store pages: Apple's own charts per storefront × chart × genre,
// shown as Apple ranks them, and one page per app. Separate from /repos:
// a store app has no stars, owner or repo, but a price, a rating and a
// storefront.
func (a *API) registerIOS(r gin.IRouter) {
	r.GET("/ios/genres", a.iosGenres)
	r.GET("/ios/charts/:country/:chart", a.iosChart)
	r.GET("/ios/new/:country", a.iosNew)
	r.GET("/ios/apps/:id", a.iosApp)
	r.GET("/ios/apps/:id/history", a.iosHistory)
}

// IOSCountries are the storefronts the ios job charts (its IOS_COUNTRIES
// default), in the order a page lists them.
var IOSCountries = []string{"us", "cn", "jp", "gb", "de"}

// maxAgeStore: the charts change once a day, at 09:00 UTC.
const maxAgeStore = 3600

// listedWindow is how recently an app must have been on a chart for its
// page to be worth indexing: an app off every chart is not looked up again,
// so its listing goes stale.
const listedWindow = 7 * 24 * time.Hour

var trackID = regexp.MustCompile(`^[0-9]{1,15}$`)

// genreBySlug and genreByID translate between the public genre slugs and
// Apple's ids, which chart_snapshots and app.genreId store.
var genreBySlug, genreByID = func() (map[string]string, map[string]string) {
	bySlug, byID := map[string]string{}, map[string]string{}
	for _, g := range appstore.Genres {
		bySlug[g.Slug], byID[g.ID] = g.ID, g.Slug
	}
	return bySlug, byID
}()

// genreSlug is the public name of a stored genre key: "all", a slug, or ""
// for a genre outside the charted set.
func genreSlug(key string) string {
	if key == appstore.GenreAll {
		return key
	}
	return genreByID[key]
}

// IOSCard is one row of a store chart.
type IOSCard struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Developer     string   `json:"developer,omitempty"`
	IconURL       string   `json:"iconUrl,omitempty"`
	Genre         string   `json:"genre,omitempty"`
	Price         float64  `json:"price"`
	Currency      string   `json:"currency,omitempty"`
	Rating        float64  `json:"rating"`
	Ratings       int      `json:"ratings"`
	RatingsD7     *float64 `json:"ratingsD7"`
	HomeCountry   string   `json:"homeCountry,omitempty"`
	ContentRating string   `json:"contentRating,omitempty"`
}

func iosCard(it *item.Item) IOSCard {
	c := IOSCard{
		ID: it.ExternalID, Name: it.Name, IconURL: it.Assets.IconURL,
		Price: it.Metrics[appstore.MetricPrice], Rating: it.Metrics[appstore.MetricRating],
		Ratings: int(it.Metrics[appstore.MetricRatings]), RatingsD7: it.Growth.D7,
	}
	if s := it.App; s != nil {
		c.Developer, c.Genre, c.Currency = s.Developer, genreByID[s.GenreID], s.Currency
		c.HomeCountry, c.ContentRating = s.HomeCountry, s.ContentRating
	}
	return c
}

// ChartEntry is one place on a chart. Change is the places gained since
// the previous snapshot of the chart; New marks an app that was not on it.
// Neither is set when there is no previous snapshot.
type ChartEntry struct {
	Rank   int     `json:"rank"`
	Change *int    `json:"change,omitempty"`
	New    bool    `json:"new,omitempty"`
	App    IOSCard `json:"app"`
}

// ChartPage is one chart on its latest day.
type ChartPage struct {
	Country string       `json:"country"`
	Chart   string       `json:"chart"`
	Genre   string       `json:"genre"`
	Day     string       `json:"day"`
	PrevDay string       `json:"prevDay,omitempty"`
	Entries []ChartEntry `json:"entries"`
}

// move is the change between a previous and a current rank, for a chart
// that had a previous snapshot.
func move(prev, now int) (*int, bool) {
	if prev == 0 {
		return nil, true
	}
	d := prev - now
	return &d, false
}

// chartParams validates a chart route: storefront, chart kind and genre
// (?genre=, a slug or "all", default "all"), returning the stored genre key.
func chartParams(c *gin.Context) (country, chart, genre string, err error) {
	country, chart = c.Param("country"), c.Param("chart")
	if !slices.Contains(IOSCountries, country) {
		return "", "", "", errBadInput("country must be one of " + strings.Join(IOSCountries, ", "))
	}
	if !slices.Contains(appstore.Charts, chart) {
		return "", "", "", errBadInput("chart must be one of " + strings.Join(appstore.Charts, ", "))
	}
	switch g := c.DefaultQuery("genre", appstore.GenreAll); {
	case g == appstore.GenreAll:
		genre = g
	case genreBySlug[g] != "":
		genre = genreBySlug[g]
	default:
		return "", "", "", errBadInput("unknown genre " + g)
	}
	return country, chart, genre, nil
}

// GET /v1/ios/charts/{country}/{chart}?genre= — the latest snapshot of one
// App Store chart in Apple's order, each place with its move since the
// snapshot before. 404 when the chart was never read.
func (a *API) iosChart(c *gin.Context) {
	country, chart, genre, err := chartParams(c)
	if err != nil {
		a.fail(c, err)
		return
	}
	ctx := c.Request.Context()
	snaps, err := a.st.LatestCharts(ctx, item.SourceIOS, country, chart, genre, 2)
	if err != nil {
		a.fail(c, err)
		return
	}
	if len(snaps) == 0 {
		a.fail(c, errNotFound)
		return
	}
	cur := snaps[0]
	cards, err := a.st.StoreCards(ctx, item.SourceIOS, cur.Entries)
	if err != nil {
		a.fail(c, err)
		return
	}
	page := ChartPage{Country: country, Chart: chart, Genre: genreSlug(genre), Day: cur.Day, Entries: []ChartEntry{}}
	var prev map[string]int
	if len(snaps) > 1 {
		page.PrevDay = snaps[1].Day
		prev = make(map[string]int, len(snaps[1].Entries))
		for i, id := range snaps[1].Entries {
			prev[id] = i + 1
		}
	}
	for i, id := range cur.Entries {
		it, ok := cards[id]
		if !ok || it.Stale {
			continue // on the chart, but not looked up (yet)
		}
		e := ChartEntry{Rank: i + 1, App: iosCard(&it)}
		if prev != nil {
			e.Change, e.New = move(prev[id], e.Rank)
		}
		page.Entries = append(page.Entries, e)
	}
	a.cached(c, maxAgeStore, gin.H{"data": page})
}

// NewEntry is one app on the new-apps view. Rank is its place in this
// view, not on any Apple chart; Best is the Apple chart place it is
// ordered by, and Charts how many of the storefront's charts it is on.
type NewEntry struct {
	Rank       int        `json:"rank"`
	Best       ChartPlace `json:"best"`
	Charts     int        `json:"charts"`
	ReleasedAt time.Time  `json:"releasedAt"`
	App        IOSCard    `json:"app"`
}

// NewPage is a storefront's new-apps view on its latest day.
type NewPage struct {
	Country string `json:"country"`
	Day     string `json:"day"`
	// Synthetic marks a StarRank view rather than an Apple chart: it merges
	// every chart of the storefront. Sort says how it is ordered ("best":
	// by best chart place), WindowDays how recent a first release must be.
	Synthetic  bool       `json:"synthetic"`
	Sort       string     `json:"sort"`
	WindowDays int        `json:"windowDays"`
	Total      int        `json:"total"`
	Entries    []NewEntry `json:"entries"`
}

// GET /v1/ios/new/{country}?limit= — apps first released in the last 90
// days that are on any of the storefront's charts today, best chart place
// first (boards.IOSNew). limit is 1–500, default 100. 404 before the first
// ios-new run.
func (a *API) iosNew(c *gin.Context) {
	country := c.Param("country")
	if !slices.Contains(IOSCountries, country) {
		a.fail(c, errBadInput("country must be one of "+strings.Join(IOSCountries, ", ")))
		return
	}
	limit, err := intParam(c, "limit", 100, 1, 500)
	if err != nil {
		a.fail(c, err)
		return
	}
	ctx := c.Request.Context()
	key, err := a.st.LatestDatedBoard(ctx, boards.IOSNewPrefix+country+"|")
	if err != nil {
		a.fail(c, err)
		return
	}
	if key == "" {
		a.fail(c, errNotFound)
		return
	}
	var rows []boards.IOSNewRow
	if _, err := a.st.Board(ctx, key, &rows); err != nil {
		a.fail(c, err)
		return
	}
	page := NewPage{
		Country: country, Day: key[strings.LastIndex(key, "|")+1:], Synthetic: true, Sort: "best",
		WindowDays: int(boards.IOSNewWindow / (24 * time.Hour)), Total: len(rows), Entries: []NewEntry{},
	}
	head := rows[:min(limit, len(rows))]
	ids := make([]string, len(head))
	for i, r := range head {
		ids[i] = r.ID
	}
	cards, err := a.st.StoreCards(ctx, item.SourceIOS, ids)
	if err != nil {
		a.fail(c, err)
		return
	}
	for _, r := range head {
		it, ok := cards[r.ID]
		if !ok || it.Stale {
			continue
		}
		page.Entries = append(page.Entries, NewEntry{
			Rank: len(page.Entries) + 1, Charts: r.Charts, ReleasedAt: r.ReleasedAt, App: iosCard(&it),
			Best: ChartPlace{Country: r.Best.Country, Chart: r.Best.Chart, Genre: genreSlug(r.Best.Genre), Rank: r.Best.Rank},
		})
	}
	a.cached(c, maxAgeStore, gin.H{"data": page})
}

// ChartPlace is one chart an app is on today.
type ChartPlace struct {
	Country string `json:"country"`
	Chart   string `json:"chart"`
	Genre   string `json:"genre"`
	Rank    int    `json:"rank"`
	Change  *int   `json:"change,omitempty"`
	New     bool   `json:"new,omitempty"`
}

// IOSDetail is an app's page.
type IOSDetail struct {
	IOSCard
	Description string       `json:"description"`
	Homepage    string       `json:"homepage,omitempty"`
	BundleID    string       `json:"bundleId,omitempty"`
	Version     string       `json:"version,omitempty"`
	UpdatedAt   time.Time    `json:"updatedAt,omitzero"`
	ReleasedAt  time.Time    `json:"releasedAt,omitzero"`
	Screenshots []string     `json:"screenshots"`
	Genres      []string     `json:"genres"`
	Countries   []string     `json:"countries"`
	Charts      []ChartPlace `json:"charts"`
	ChartsDay   string       `json:"chartsDay,omitempty"`
	// Listed is whether the app was on a chart in the last week: an app
	// that is not has a frozen listing, and its page is not indexed.
	Listed    bool      `json:"listed"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// GET /v1/ios/apps/{id} — one App Store app by its track id, with the
// charts it is on today.
func (a *API) iosApp(c *gin.Context) {
	id := c.Param("id")
	if !trackID.MatchString(id) {
		a.fail(c, errNotFound)
		return
	}
	ctx := c.Request.Context()
	it, err := a.st.Item(ctx, item.SourceIOS, id)
	if err != nil {
		a.fail(c, notFound(err))
		return
	}
	if it.Stale || it.App == nil {
		a.fail(c, errNotFound)
		return
	}
	s := it.App
	d := IOSDetail{
		IOSCard: iosCard(&it), Description: strings.TrimSpace(it.Description), Homepage: it.Homepage,
		BundleID: s.BundleID, Version: s.Version, UpdatedAt: s.UpdatedAt, ReleasedAt: s.ReleasedAt,
		Screenshots: orEmpty(s.Screenshots), Genres: []string{}, Countries: orEmpty(s.Countries),
		Charts: []ChartPlace{}, FetchedAt: it.FetchedAt,
		Listed: time.Since(it.FetchedAt) < listedWindow,
	}
	for _, g := range s.GenreIDs {
		if slug := genreByID[g]; slug != "" && !slices.Contains(d.Genres, slug) {
			d.Genres = append(d.Genres, slug)
		}
	}
	// The whole store description, when the blob has it; the item keeps a cut.
	if full, err := a.st.Readme(ctx, item.SourceIOS, id); err != nil {
		a.log.Warn("api: ios description", "app", id, "err", err)
	} else if full = strings.TrimSpace(full); full != "" {
		d.Description = full
	}
	if err := a.placeCharts(c, &d, s); err != nil {
		a.log.Warn("api: ios charts", "app", id, "err", err)
	}
	a.cached(c, maxAgeStore, gin.H{"data": d})
}

// placeCharts fills an app's chart places from the positions the ios job
// keeps on it, when they are today's: positions left from an earlier day
// (a run that could not read every chart does not clear them) are stale.
func (a *API) placeCharts(c *gin.Context, d *IOSDetail, s *item.StoreApp) error {
	if len(s.Charts) == 0 {
		return nil
	}
	latest, err := a.st.LatestChartDay(c.Request.Context(), item.SourceIOS)
	if err != nil || s.ChartsDay != latest {
		return err
	}
	d.ChartsDay = s.ChartsDay
	for _, p := range s.Charts {
		g := genreSlug(p.Genre)
		if g == "" || p.Rank <= 0 {
			continue
		}
		pl := ChartPlace{Country: p.Country, Chart: p.Chart, Genre: g, Rank: p.Rank}
		pl.Change, pl.New = move(p.Prev, p.Rank)
		d.Charts = append(d.Charts, pl)
	}
	return nil
}

// IOSHistory is an app's two daily curves: the rating count of its home
// storefront, and its best rank there (only days it was on a chart).
type IOSHistory struct {
	Ratings  []Point `json:"ratings"`
	BestRank []Point `json:"bestRank"`
}

// GET /v1/ios/apps/{id}/history — every daily snapshot of the app.
func (a *API) iosHistory(c *gin.Context) {
	id := c.Param("id")
	if !trackID.MatchString(id) {
		a.fail(c, errNotFound)
		return
	}
	ctx := c.Request.Context()
	if _, err := a.st.Item(ctx, item.SourceIOS, id); err != nil {
		a.fail(c, notFound(err))
		return
	}
	h := IOSHistory{Ratings: []Point{}, BestRank: []Point{}}
	for metric, out := range map[string]*[]Point{appstore.MetricRatings: &h.Ratings, appstore.MetricBestRank: &h.BestRank} {
		rs, err := a.st.MetricSeries(ctx, item.SourceIOS, id, metric, time.Time{})
		if err != nil {
			a.fail(c, err)
			return
		}
		for _, r := range rs {
			if metric == appstore.MetricBestRank && r.V <= 0 {
				continue
			}
			*out = append(*out, Point{T: r.At.UnixMilli(), V: int(r.V)})
		}
	}
	a.cached(c, maxAgeStore, gin.H{"data": h})
}

// IOSGenre is one charted genre and how many tracked apps it has.
type IOSGenre struct {
	Slug  string `json:"slug"`
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// GET /v1/ios/genres — what the chart routes accept: storefronts, chart
// kinds and genres, in Apple's genre order.
func (a *API) iosGenres(c *gin.Context) {
	counts, err := a.st.DomainCounts(c.Request.Context(), item.SourceIOS)
	if err != nil {
		a.fail(c, err)
		return
	}
	genres := make([]IOSGenre, 0, len(appstore.Genres))
	for _, g := range appstore.Genres {
		genres = append(genres, IOSGenre{Slug: g.Slug, ID: g.ID, Count: counts[appstore.DomainPrefix+g.Slug]})
	}
	a.cached(c, maxAgeStore, gin.H{"data": gin.H{
		"countries": IOSCountries, "charts": appstore.Charts, "genres": genres,
	}})
}
