// Package service holds the query/business logic sitting between HTTP handlers
// and the repository.
package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
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
	Category string   // categoryId
	Q        string   // case-insensitive match on externalId/name/description
	Topics   []string // every topic must be present in sourceData.topicNames
	License  string   // exact sourceData.license
	Sort     string   // "field:order"
	Limit    int
	Page     int // 1-based; combined with Limit for offset pagination
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
	if q.Category != "" {
		filter["categoryId"] = q.Category
	}
	if q.License != "" {
		filter["sourceData.license"] = q.License
	}
	if len(q.Topics) > 0 {
		filter["sourceData.topicNames"] = bson.M{"$all": q.Topics}
	}
	if q.Q != "" {
		re := primitive.Regex{Pattern: regexp.QuoteMeta(q.Q), Options: "i"}
		filter["$or"] = []bson.M{
			{"externalId": re},
			{"name": re},
			{"description": re},
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

	sortField, sortOrder, err := parseSort(q.Sort)
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
	if q.Category != "" {
		filter["categoryId"] = q.Category
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
