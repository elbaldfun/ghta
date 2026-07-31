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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/pkg/query"
)

// containsCJK reports whether s contains CJK characters (Chinese/Japanese
// kanji, kana, or Hangul) — queries that Mongo's text index cannot segment.
func containsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0x3040 && r <= 0x30FF) || (r >= 0xAC00 && r <= 0xD7AF) {
			return true
		}
	}
	return false
}

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

	filter := liveFilter(bson.M{"source": domain.SourceGitHub, "name": bson.M{"$gte": q, "$lt": q + "￿"}})
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

// liveFilter excludes reconciler-tombstoned records (deleted upstream or
// rename ghosts) from a query filter. Every user-facing ranking applies it.
// Equality on stale (not $ne): $ne can't use an index, so it forced a full
// COLLSCAN that read every document's blobs and pushed CountDocuments to
// 14–30s (1-vCPU CPU pegged at 100%). Every live doc carries stale:false
// (backfilled + set on insert), so equality is index-friendly.
func liveFilter(m bson.M) bson.M {
	m["stale"] = false
	return m
}

// liveMatch is liveFilter as a ready-made aggregation stage.
var liveMatch = bson.D{{Key: "$match", Value: bson.M{"stale": false}}}

// sortFields whitelists user-facing sort fields and maps them to stored paths.
// "stars" is the documented alias for the GitHub primary metric.
var sortFields = map[string]string{
	"stars":     "metrics.stars",
	"forks":     "metrics.forks",
	"issues":    "metrics.openIssues",
	"fetchedAt": "fetchedAt",
	"updated":   "fetchedAt",
	// Growth boards — the differentiated signal. Nulls (no snapshots yet)
	// sort last under desc, which is exactly what a growth board wants.
	"daily":   "dailyIncrease",
	"weekly":  "weeklyIncrease",
	"monthly": "monthlyIncrease",
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
	filter := liveFilter(bson.M{})
	// Default to the GitHub corpus: these are the repo boards. Other sources
	// (huggingface) have their own endpoints and are only included when asked
	// for explicitly.
	if q.Source == "" {
		q.Source = string(domain.SourceGitHub)
	}
	filter["source"] = q.Source
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
		if containsCJK(q.Q) {
			// Mongo's text index has no CJK segmentation — a Chinese tagline is
			// one giant token and $text can never match a substring of it. CJK
			// queries instead substring-match the store tagline (the zh search
			// corpus, change 15) plus name/description.
			re := primitive.Regex{Pattern: regexp.QuoteMeta(q.Q), Options: "i"}
			filter["$or"] = []bson.M{
				{"store.taglineZh": re},
				{"name": re},
				{"description": re},
			}
		} else {
			// Full-text search over the weighted text index (name > topics >
			// description), ranked by blended relevance below.
			filter["$text"] = bson.M{"$search": q.Q}
		}
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
	// CJK queries run as substring matches (no $text, see above), so there is no
	// textScore to blend — they skip the relevance pipeline and rank by stars.
	relevance := q.Q != "" && sortSpec == "" && !containsCJK(q.Q)

	sortField, sortOrder, err := parseSort(sortSpec)
	if err != nil {
		return nil, 0, err
	}
	if q.Q != "" && sortSpec == "" && !relevance {
		sortField, sortOrder = "metrics.stars", -1
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
		items, err := s.relevanceSearch(ctx, filter, q.Q, page, limit)
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
// boost and a name-match bonus. Raw textScore is term-frequency based, so
// low-star repos that repeat the query word across name/topics/description
// outscore the canonical repo (searching "react" buried react/react at #18).
// ln(stars) separates near-ties without letting a loosely-matching mega-repo
// hijack the query, and an exact name hit (the "find this project by its
// name" intent) is decisive.
func (s *TrendService) relevanceSearch(ctx context.Context, filter bson.M, rawQuery string, page, limit int) ([]domain.TrackedItem, error) {
	q := strings.ToLower(strings.TrimSpace(rawQuery))
	nameLower := bson.M{"$toLower": "$name"}
	// name == query -> 5x; name starts with query -> 2x; else 1x.
	nameBonus := bson.M{"$switch": bson.M{
		"branches": bson.A{
			bson.M{"case": bson.M{"$eq": bson.A{nameLower, q}}, "then": 5},
			bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$indexOfCP": bson.A{nameLower, q}}, 0}}, "then": 2},
		},
		"default": 1,
	}}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		// Shed the heavyweight blobs before the in-memory sort stage.
		{{Key: "$project", Value: bson.M{"sourceData.readme": 0, "sourceData.releases": 0}}},
		{{Key: "$addFields", Value: bson.M{
			"searchScore": bson.M{"$multiply": bson.A{
				bson.M{"$meta": "textScore"},
				// 1 + ln(stars+2): ~1.7x at 0 stars, ~14x at 250k.
				bson.M{"$add": bson.A{1, bson.M{"$ln": bson.M{"$add": bson.A{
					bson.M{"$ifNull": bson.A{"$metrics.stars", 0}}, 2,
				}}}}},
				nameBonus,
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

// RepoRank locates an item on one leaderboard scope — the detail page's
// "JavaScript #3 · web/frontend #1" line and the future README badge both
// read from this.
type RepoRank struct {
	Scope string `json:"scope"`         // "overall" | "language" | "category"
	Key   string `json:"key,omitempty"` // language name or category path
	Rank  int64  `json:"rank"`          // 1-based; ties share the best position
}

// Ranks computes the item's position on each board it belongs to: the overall
// corpus, its language board, and every domain path (capped at 2). Each rank
// is one indexed count of live repos with strictly more stars. Best-effort —
// a failing scope is skipped rather than failing the page.
func (s *TrendService) Ranks(ctx context.Context, item *domain.TrackedItem) []RepoRank {
	stars, ok := item.Metrics["stars"]
	if !ok {
		return nil
	}
	base := func() bson.M {
		return liveFilter(bson.M{"source": item.Source, "metrics.stars": bson.M{"$gt": stars}})
	}
	ranks := []RepoRank{}
	add := func(scope, key string, extra bson.M) {
		f := base()
		for k, v := range extra {
			f[k] = v
		}
		n, err := s.store.Items().CountDocuments(ctx, f)
		if err != nil {
			return
		}
		ranks = append(ranks, RepoRank{Scope: scope, Key: key, Rank: n + 1})
	}

	add("overall", "", nil)
	if item.Language != "" {
		add("language", item.Language, bson.M{"language": item.Language})
	}
	for i, path := range item.CategoryPath {
		if i >= 2 {
			break
		}
		add("category", path, bson.M{"categoryPath": path})
	}
	return ranks
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
	if q.Source == "" {
		q.Source = string(domain.SourceGitHub)
	}
	filter["source"] = q.Source
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
// stars descending — the order a ranking API's consumers expect (the old
// fetchedAt default returned an effectively arbitrary list).
func parseSort(sort string) (string, int, error) {
	if sort == "" {
		return "metrics.stars", -1, nil
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
