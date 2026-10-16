package store

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// ListSorts are the stored paths a list can be ordered by.
var ListSorts = []string{"metrics.stars", "metrics.forks", "metrics.openIssues", "growth.d1", "growth.d7", "growth.d30", "gh.pushedAt"}

// The read paths of the item lists (REBUILD M4). A list walks a {source,
// key, externalId} index in order, the external id breaking ties so pages
// never overlap, and stops at the page. A filter that keeps few items is
// answered from its own index and sorted in memory instead; the planner
// picks. Text search has the one text index.
var listIndexes = append(sortIndexes(),
	mongo.IndexModel{Keys: bson.D{{Key: "language", Value: 1}}},
	mongo.IndexModel{Keys: bson.D{{Key: "topics", Value: 1}}},
	mongo.IndexModel{
		Keys: bson.D{
			{Key: "name", Value: "text"}, {Key: "topics", Value: "text"}, {Key: "labels.tags", Value: "text"},
			{Key: "externalId", Value: "text"}, {Key: "description", Value: "text"},
		},
		// Our `language` field holds values like "TypeScript", which Mongo
		// would read as the text language and refuse: point the override at
		// a field no document has.
		Options: options.Index().SetName("items_text").SetLanguageOverride("searchLang").SetWeights(bson.D{
			{Key: "name", Value: 10}, {Key: "topics", Value: 6}, {Key: "labels.tags", Value: 6},
			{Key: "externalId", Value: 4}, {Key: "description", Value: 2},
		}),
	},
	// Prefix autocomplete, case-insensitive with the same collation.
	mongo.IndexModel{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetName("name_ci").SetCollation(nameCollation)},
)

func sortIndexes() []mongo.IndexModel {
	var out []mongo.IndexModel
	for _, k := range ListSorts {
		out = append(out, mongo.IndexModel{Keys: bson.D{{Key: "source", Value: 1}, {Key: k, Value: -1}, {Key: "externalId", Value: 1}}})
	}
	return out
}

// MaxListTotal is as far as a list counts: no page goes deeper, and counting
// every match of a broad filter would read the whole collection.
const MaxListTotal = 10000

var nameCollation = &options.Collation{Locale: "en", Strength: 2}

// ListQuery selects and orders a page of items. Empty fields do not filter.
type ListQuery struct {
	Source string
	// Owner is a GitHub account, as stored; it matches the id's prefix so the
	// unique (source, externalId) index serves it.
	Owner    string
	Language string
	// Domain is a leaf path ("ai/vision") or a top-level domain ("ai").
	Domain  string
	Type    string
	License string
	Topics  []string // every one must be present
	// Stars and Issues are range conditions on the metric (pkg/query).
	Stars, Issues bson.M
	// Q searches text. Latin text goes through the text index and, unless a
	// sort is given, orders by relevance; CJK text, which the index does not
	// split into words, matches name, description and the Chinese tagline.
	Q string
	// Sort is a stored path ("metrics.stars", "growth.d7"); "" is relevance
	// for a text search and stars otherwise.
	Sort        string
	Asc         bool
	Skip, Limit int
}

// ListFields is what a list row carries: the card, not the document.
var ListFields = bson.M{
	"source": 1, "externalId": 1, "name": 1, "description": 1, "language": 1, "license": 1, "homepage": 1,
	"topics": 1, "metrics": 1, "growth": 1, "labels.domain": 1, "labels.type": 1, "labels.tags": 1,
	"labels.taglineZh": 1, "labels.taglineEn": 1, "assets.iconUrl": 1, "gh.owner": 1, "gh.pushedAt": 1,
	"ranks.overall": 1, "fetchedAt": 1,
}

// ContainsCJK reports Chinese, Japanese or Korean script in s.
func ContainsCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF || r >= 0x3040 && r <= 0x30FF || r >= 0xAC00 && r <= 0xD7AF {
			return true
		}
	}
	return false
}

func (q ListQuery) filter() bson.M {
	f := bson.M{"source": q.Source, "stale": bson.M{"$ne": true}}
	if q.Owner != "" {
		f["externalId"] = bson.M{"$regex": "^" + regexp.QuoteMeta(q.Owner) + "/"}
	}
	if q.Language != "" {
		f["language"] = q.Language
	}
	switch {
	case q.Domain == "":
	case strings.Contains(q.Domain, "/"):
		f["labels.domain"] = q.Domain
	default:
		f["labels.domain"] = bson.M{"$regex": "^" + regexp.QuoteMeta(q.Domain) + "/"}
	}
	if q.Type != "" {
		f["labels.type"] = q.Type
	}
	if q.License != "" {
		f["license"] = q.License
	}
	if len(q.Topics) > 0 {
		f["topics"] = bson.M{"$all": q.Topics}
	}
	if q.Stars != nil {
		f["metrics.stars"] = q.Stars
	}
	if q.Issues != nil {
		f["metrics.openIssues"] = q.Issues
	}
	if q.Q != "" {
		if ContainsCJK(q.Q) {
			re := primitive.Regex{Pattern: regexp.QuoteMeta(q.Q), Options: "i"}
			f["$or"] = bson.A{bson.M{"labels.taglineZh": re}, bson.M{"name": re}, bson.M{"description": re}}
		} else {
			f["$text"] = bson.M{"$search": q.Q}
		}
	}
	return f
}

// ListItems returns one page of items and how many match, up to
// MaxListTotal.
func (s *Store) ListItems(ctx context.Context, q ListQuery) ([]item.Item, int64, error) {
	f := q.filter()
	total, err := s.Items().CountDocuments(ctx, f, options.Count().SetLimit(MaxListTotal))
	if err != nil {
		return nil, 0, fmt.Errorf("list count: %w", err)
	}
	out := []item.Item{}
	if q.Sort == "" && q.Q != "" && !ContainsCJK(q.Q) {
		cur, err := s.Items().Aggregate(ctx, relevance(f, q), options.Aggregate().SetAllowDiskUse(true))
		if err != nil {
			return nil, 0, fmt.Errorf("list search: %w", err)
		}
		if err := cur.All(ctx, &out); err != nil {
			return nil, 0, fmt.Errorf("list search: %w", err)
		}
		return out, total, nil
	}
	key, order := q.Sort, -1
	if key == "" {
		key = "metrics.stars"
	}
	if q.Asc {
		order = 1
	}
	// Ties in the order the index holds them, walked forward or backward.
	cur, err := s.Items().Find(ctx, f, options.Find().
		SetSort(bson.D{{Key: key, Value: order}, {Key: "externalId", Value: -order}}).
		SetSkip(int64(q.Skip)).SetLimit(int64(q.Limit)).SetProjection(ListFields))
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	return out, total, nil
}

// relevance orders a text search by the text score weighted toward
// established and exactly-named repos: an exact name match counts 5x, a name
// starting with the query 2x, and stars add a log term.
func relevance(f bson.M, q ListQuery) mongo.Pipeline {
	term := strings.ToLower(strings.TrimSpace(q.Q))
	name := bson.M{"$toLower": "$name"}
	bonus := bson.M{"$switch": bson.M{
		"branches": bson.A{
			bson.M{"case": bson.M{"$eq": bson.A{name, term}}, "then": 5},
			bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$indexOfCP": bson.A{name, term}}, 0}}, "then": 2},
		},
		"default": 1,
	}}
	return mongo.Pipeline{
		{{Key: "$match", Value: f}},
		{{Key: "$addFields", Value: bson.M{"score": bson.M{"$multiply": bson.A{
			bson.M{"$meta": "textScore"},
			bson.M{"$add": bson.A{1, bson.M{"$ln": bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$metrics.stars", 0}}, 2}}}}},
			bonus,
		}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}, {Key: "metrics.stars", Value: -1}, {Key: "externalId", Value: 1}}}},
		{{Key: "$skip", Value: int64(q.Skip)}},
		{{Key: "$limit", Value: int64(q.Limit)}},
		{{Key: "$project", Value: ListFields}},
	}
}

// Suggest returns up to limit live GitHub repos whose name starts with
// prefix, ignoring case, most-starred first.
func (s *Store) Suggest(ctx context.Context, prefix string, limit int) ([]item.Item, error) {
	// Up to 60 prefix matches through the collated index, then the most
	// starred of those: a common prefix has more matches than are worth
	// sorting.
	cur, err := s.Items().Find(ctx,
		bson.M{"source": item.SourceGitHub, "stale": bson.M{"$ne": true}, "name": bson.M{"$gte": prefix, "$lt": prefix + "￿"}},
		options.Find().SetCollation(nameCollation).SetLimit(60).
			SetProjection(bson.M{"externalId": 1, "name": 1, "language": 1, "metrics.stars": 1, "assets.iconUrl": 1}))
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	var out []item.Item
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Metrics["stars"] > out[j].Metrics["stars"] })
	return out[:min(limit, len(out))], nil
}
