// Package service holds the query/business logic sitting between HTTP handlers
// and the repository.
package service

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
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
}

type TrendQuery struct {
	Source   string
	Stars    string
	Issues   string
	Language string
	Category string // categoryId
	Sort     string // "field:order"
	Limit    int
}

type TrendService struct {
	store *repository.Store
}

func NewTrendService(store *repository.Store) *TrendService {
	return &TrendService{store: store}
}

// List returns tracked items matching the query. Invalid filters/sort/limit
// return an InputError.
func (s *TrendService) List(ctx context.Context, q TrendQuery) ([]domain.TrackedItem, error) {
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
	if q.Stars != "" {
		cond, err := query.ParseRange(q.Stars)
		if err != nil {
			return nil, badInput("stars: %v", err)
		}
		filter["metrics.stars"] = cond
	}
	if q.Issues != "" {
		cond, err := query.ParseRange(q.Issues)
		if err != nil {
			return nil, badInput("issues: %v", err)
		}
		filter["metrics.openIssues"] = cond
	}

	sortField, sortOrder, err := parseSort(q.Sort)
	if err != nil {
		return nil, err
	}

	limit := q.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 0 || limit > maxLimit {
		return nil, badInput("limit must be between 1 and %d", maxLimit)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: sortField, Value: sortOrder}}).
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
