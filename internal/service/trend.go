// Package service holds the query/business logic sitting between HTTP handlers
// and the repository.
package service

import (
	"context"
	"errors"
	"fmt"
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
	store *repository.Store
}

func NewTrendService(store *repository.Store) *TrendService {
	return &TrendService{store: store}
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

	from := time.Now().UTC().AddDate(0, 0, -90)
	cur, err := s.store.Snapshots().Find(ctx,
		bson.M{"meta.source": source, "meta.externalId": externalID, "capturedAt": bson.M{"$gte": from}},
		options.Find().SetSort(bson.D{{Key: "capturedAt", Value: 1}}),
	)
	if err != nil {
		return nil, nil, err
	}
	history := []domain.MetricSnapshot{}
	if err := cur.All(ctx, &history); err != nil {
		return nil, nil, err
	}
	return &item, history, nil
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
