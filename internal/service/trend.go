// Package service holds the query/business logic sitting between HTTP handlers
// and the repository.
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/pkg/query"
)

// SuggestItem is one autocomplete row for the search-as-you-type dropdown.
type SuggestItem struct {
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
	Stars      int    `json:"stars"`
	Language   string `json:"language,omitempty"`
	IconURL    string `json:"iconUrl,omitempty"`
}

// suggestCollation makes the name prefix match case-insensitively via the
// matching collated index (name_ci), so autocomplete stays index-backed.
var suggestCollation = &options.Collation{Locale: "en", Strength: 2}

// Suggest returns up to `limit` name-prefix matches for autocomplete, ranked by
// stars. Uses a case-insensitive prefix range on the collated name index (fast,
// index-backed), pulling a bounded window and ranking it in Go.
func (s *TrendService) Suggest(ctx context.Context, q string, limit int) ([]SuggestItem, error) {
	q = strings.TrimSpace(q)
	if len([]rune(q)) < 2 {
		return []SuggestItem{}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}

	filter := bson.M{"name": bson.M{"$gte": q, "$lt": q + "￿"}}
	cur, err := s.store.Items().Find(ctx, filter,
		options.Find().
			SetCollation(suggestCollation).
			SetLimit(60).
			SetProjection(bson.M{"externalId": 1, "name": 1, "metrics.stars": 1, "language": 1, "iconUrl": 1}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ExternalID string `bson:"externalId"`
		Name       string `bson:"name"`
		Language   string `bson:"language"`
		IconURL    string `bson:"iconUrl"`
		Metrics    struct {
			Stars float64 `bson:"stars"`
		} `bson:"metrics"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Metrics.Stars > rows[j].Metrics.Stars })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]SuggestItem, len(rows))
	for i, r := range rows {
		out[i] = SuggestItem{ExternalID: r.ExternalID, Name: r.Name, Stars: int(r.Metrics.Stars), Language: r.Language, IconURL: r.IconURL}
	}
	return out, nil
}

// InputError marks a client input problem (mapped to HTTP 400).
type InputError struct{ msg string }

func (e InputError) Error() string { return e.msg }

func badInput(format string, a ...any) InputError { return InputError{msg: fmt.Sprintf(format, a...)} }

const defaultLimit = 50
const maxLimit = 50

// sortFields whitelists user-facing sort fields and maps them to stored paths.
// "stars" is the documented alias for the GitHub primary metric.
var sortFields = map[string]string{
	"stars":     "metrics.stars",
	"forks":     "metrics.forks",
	"issues":    "metrics.openIssues",
	"fetchedAt": "fetchedAt",
	"updated":   "fetchedAt",
}

type TrendQuery struct {
	Source   string
	Stars    string
	Issues   string
	Language string
	Category string   // categoryId (hex) or a category path (contains "/")
	Type     string   // form facet: cli|app|library|software|tutorial|awesome|interview|skill
	Q        string   // case-insensitive match on externalId/name/description
	Topics   []string // every topic must be present in sourceData.topicNames
	License  string   // exact sourceData.license
	Sort     string   // "field:order"
	Limit    int
	Page     int // 1-based; combined with Limit for offset pagination
}

var hex24 = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// categoryFilter matches a category argument three ways:
//   - a 24-char hex id      -> categoryId membership
//   - a leaf path "a/b"     -> exact categoryPath membership
//   - a bare parent "a"     -> any leaf under it (categoryPath prefix "a/")
//
// categoryId/categoryPath are arrays (multi-label), so equality matches membership.
func categoryFilter(filter bson.M, category string) {
	switch {
	case category == "":
		return
	case hex24.MatchString(category):
		filter["categoryId"] = category
	case strings.Contains(category, "/"):
		filter["categoryPath"] = category
	default:
		filter["categoryPath"] = bson.M{"$regex": "^" + regexp.QuoteMeta(category) + "/"}
	}
}

type TrendService struct {
	store   *repository.Store
	history *StarHistoryService // optional lazy star-history backfill
}

func NewTrendService(store *repository.Store, history *StarHistoryService) *TrendService {
	return &TrendService{store: store, history: history}
}

// List returns tracked items matching the query. Invalid filters/sort/limit
// return an InputError. The returned total is the full match count (for
// pagination), independent of limit/page.
func (s *TrendService) List(ctx context.Context, q TrendQuery) ([]domain.TrackedItem, int64, error) {
	filter := bson.M{}
	if q.Source != "" {
		filter["source"] = q.Source
	}
	if q.Language != "" {
		filter["language"] = q.Language
	}
	categoryFilter(filter, q.Category)
	if q.Type != "" {
		filter["type"] = q.Type
	}
	if q.License != "" {
		filter["sourceData.license"] = q.License
	}
	if len(q.Topics) > 0 {
		filter["sourceData.topicNames"] = bson.M{"$all": q.Topics}
	}
	if q.Q != "" {
		// Full-text search over the weighted text index (name > topics >
		// description). Replaces the old unanchored $or regex, which scanned the
		// whole collection and ranked by stars instead of relevance.
		filter["$text"] = bson.M{"$search": q.Q}
	}
	if q.Stars != "" {
		cond, err := query.ParseRange(q.Stars)
		if err != nil {
			return nil, 0, badInput("stars: %v", err)
		}
		filter["metrics.stars"] = cond
	}
	if q.Issues != "" {
		cond, err := query.ParseRange(q.Issues)
		if err != nil {
			return nil, 0, badInput("issues: %v", err)
		}
		filter["metrics.openIssues"] = cond
	}

	// A search ranks by blended relevance unless the caller explicitly asks for
	// a concrete field ("relevance" without a query degrades to the default).
	sortSpec := q.Sort
	if sortName(sortSpec) == "relevance" {
		sortSpec = ""
	}
	relevance := q.Q != "" && sortSpec == ""

	sortField, sortOrder, err := parseSort(sortSpec)
	if err != nil {
		return nil, 0, err
	}

	limit := q.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 0 || limit > maxLimit {
		return nil, 0, badInput("limit must be between 1 and %d", maxLimit)
	}
	page := q.Page
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return nil, 0, badInput("page must be >= 1")
	}

	total, err := s.store.Items().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	if relevance {
		items, err := s.relevanceSearch(ctx, filter, page, limit)
		if err != nil {
			return nil, 0, err
		}
		return items, total, nil
	}

	opts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortOrder}}).
		SetSkip(int64(page-1) * int64(limit)).
		SetLimit(int64(limit)).
		// The list view never needs the heavyweight sourceData blobs.
		SetProjection(bson.M{"sourceData.readme": 0, "sourceData.releases": 0})

	cur, err := s.store.Items().Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	items := []domain.TrackedItem{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// relevanceSearch ranks $text matches by textScore blended with a popularity
// boost. Raw textScore is term-frequency based, so a low-star repo that stuffs
// the query word into its name/topics/description outscores the canonical repo
// (searching "react" buried facebook/react below tutorial forks). Multiplying
// by ln(stars) keeps relevance primary while letting popularity break the
// near-ties that dominate name searches.
func (s *TrendService) relevanceSearch(ctx context.Context, filter bson.M, page, limit int) ([]domain.TrackedItem, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		// Shed the heavyweight blobs before the in-memory sort stage.
		{{Key: "$project", Value: bson.M{"sourceData.readme": 0, "sourceData.releases": 0}}},
		{{Key: "$addFields", Value: bson.M{
			"searchScore": bson.M{"$multiply": bson.A{
				bson.M{"$meta": "textScore"},
				// 1 + ln(stars+2): ~1.7x at 0 stars, ~14x at 250k. Log-scaled so
				// popularity separates near-equal text scores without letting a
				// mega-repo hijack unrelated queries.
				bson.M{"$add": bson.A{1, bson.M{"$ln": bson.M{"$add": bson.A{
					bson.M{"$ifNull": bson.A{"$metrics.stars", 0}}, 2,
				}}}}},
			}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "searchScore", Value: -1}, {Key: "metrics.stars", Value: -1}}}},
		{{Key: "$skip", Value: int64(page-1) * int64(limit)}},
		{{Key: "$limit", Value: int64(limit)}},
		{{Key: "$unset", Value: "searchScore"}},
	}
	cur, err := s.store.Items().Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	items := []domain.TrackedItem{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// sortName extracts the field part of a "field:order" sort spec.
func sortName(sort string) string {
	if i := strings.IndexByte(sort, ':'); i >= 0 {
		return sort[:i]
	}
	return sort
}

// Item returns a single tracked item and its recent snapshot history (for the
// detail page's metric-history chart).
func (s *TrendService) Item(ctx context.Context, source, externalID string) (*domain.TrackedItem, []domain.MetricSnapshot, error) {
	if source == "" || externalID == "" {
		return nil, nil, badInput("source and externalId are required")
	}
	var item domain.TrackedItem
	err := s.store.Items().FindOne(ctx, bson.M{"source": source, "externalId": externalID}).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	cur, err := s.store.Snapshots().Find(ctx,
		bson.M{"meta.source": source, "meta.externalId": externalID},
		options.Find().SetSort(bson.D{{Key: "capturedAt", Value: 1}}),
	)
	if err != nil {
		return nil, nil, err
	}
	snapshots := []domain.MetricSnapshot{}
	if err := cur.All(ctx, &snapshots); err != nil {
		return nil, nil, err
	}

	// Prepend the backfilled long-term curve (lazy; nil on failure) so the
	// chart covers the repo's full life, not just our own snapshot window.
	var backfill []domain.StarPoint
	if s.history != nil {
		backfill, _ = s.history.Ensure(ctx, item.Source, item.ExternalID)
	}
	return &item, mergeHistory(&item, backfill, snapshots), nil
}

// mergeHistory splices the backfilled monthly curve onto our daily snapshots.
// GH Archive counts star events but never unstars, so the backfill tail
// overshoots reality — scale the whole curve so its last point matches the
// first real observation, keeping the seam continuous.
func mergeHistory(item *domain.TrackedItem, backfill []domain.StarPoint, snapshots []domain.MetricSnapshot) []domain.MetricSnapshot {
	if len(backfill) == 0 {
		return snapshots
	}

	anchorV := item.Metrics["stars"]
	cutoff := time.Now().UTC()
	if len(snapshots) > 0 {
		if v, ok := snapshots[0].Metrics["stars"]; ok {
			anchorV = v
		}
		cutoff = snapshots[0].CapturedAt
	}

	kept := backfill[:0:0]
	for _, p := range backfill {
		if p.T.Before(cutoff) {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return snapshots
	}

	factor := 1.0
	if last := kept[len(kept)-1].V; last > 0 && anchorV > 0 {
		factor = anchorV / last
	}

	meta := domain.SnapshotMeta{Source: item.Source, ExternalID: item.ExternalID}
	merged := make([]domain.MetricSnapshot, 0, len(kept)+len(snapshots))
	for _, p := range kept {
		merged = append(merged, domain.MetricSnapshot{
			Meta:       meta,
			Metrics:    map[string]float64{"stars": math.Round(p.V * factor)},
			CapturedAt: p.T,
		})
	}
	return append(merged, snapshots...)
}

// risingWindows maps a window name to the stored increase field.
var risingWindows = map[string]string{
	"daily":   "dailyIncrease",
	"weekly":  "weeklyIncrease",
	"monthly": "monthlyIncrease",
}

type RisingQuery struct {
	Window   string // daily | weekly | monthly (default weekly)
	Source   string
	Category string
	Type     string
	Language string
	Limit    int
}

// Rising returns items ranked by their growth in the requested window, highest
// first, excluding items whose increase for that window is null (no baseline).
func (s *TrendService) Rising(ctx context.Context, q RisingQuery) ([]domain.TrackedItem, error) {
	window := q.Window
	if window == "" {
		window = "weekly"
	}
	field, ok := risingWindows[window]
	if !ok {
		return nil, badInput("window must be one of: daily, weekly, monthly")
	}

	filter := bson.M{field: bson.M{"$ne": nil}}
	if q.Source != "" {
		filter["source"] = q.Source
	}
	if q.Language != "" {
		filter["language"] = q.Language
	}
	categoryFilter(filter, q.Category)
	if q.Type != "" {
		filter["type"] = q.Type
	}

	limit := q.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 0 || limit > maxLimit {
		return nil, badInput("limit must be between 1 and %d", maxLimit)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: field, Value: -1}}).
		SetLimit(int64(limit))

	cur, err := s.store.Items().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	items := []domain.TrackedItem{}
	if err := cur.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// parseSort validates "field:order" against the whitelist, defaulting to
// fetchedAt descending.
func parseSort(sort string) (string, int, error) {
	if sort == "" {
		return "fetchedAt", -1, nil
	}
	field := sort
	order := -1
	if i := strings.IndexByte(sort, ':'); i >= 0 {
		field = sort[:i]
		if strings.EqualFold(sort[i+1:], "asc") {
			order = 1
		}
	}
	mapped, ok := sortFields[field]
	if !ok {
		return "", 0, badInput("unsupported sort field %q", field)
	}
	return mapped, order, nil
}
