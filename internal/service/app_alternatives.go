package service

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const altTargetsTTL = time.Hour

// AltTarget is one commercial product with the count of open-source apps that
// replace it — a row on the /alternatives index.
type AltTarget struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Kind  string `json:"kind,omitempty"`
	Count int    `json:"count"`
}

// AltTargets returns the paid products with the most open-source alternatives,
// most-covered first. Cached.
func (s *AppService) AltTargets(ctx context.Context) ([]AltTarget, error) {
	return s.targets.get(ctx, "all", func(ctx context.Context) ([]AltTarget, error) {
		pipeline := mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.M{"alternativeTo.0": bson.M{"$exists": true}}}},
			bson.D{{Key: "$unwind", Value: "$alternativeTo"}},
			bson.D{{Key: "$group", Value: bson.M{
				"_id":   "$alternativeTo.slug",
				"name":  bson.M{"$first": "$alternativeTo.name"},
				"kind":  bson.M{"$first": "$alternativeTo.kind"},
				"count": bson.M{"$sum": 1},
			}}},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
			bson.D{{Key: "$limit", Value: 200}},
		}
		cur, err := s.store.Items().Aggregate(ctx, pipeline)
		if err != nil {
			return nil, fmt.Errorf("alt targets aggregate: %w", err)
		}
		var raw []struct {
			Slug  string `bson:"_id"`
			Name  string `bson:"name"`
			Kind  string `bson:"kind"`
			Count int    `bson:"count"`
		}
		if err := cur.All(ctx, &raw); err != nil {
			return nil, fmt.Errorf("alt targets decode: %w", err)
		}
		out := make([]AltTarget, 0, len(raw))
		for _, r := range raw {
			out = append(out, AltTarget{Name: r.Name, Slug: r.Slug, Kind: r.Kind, Count: r.Count})
		}
		return out, nil
	})
}

// ByAlternative returns the open-source apps that replace one product (by slug),
// most-popular first, plus the product's display name. Cached per slug.
func (s *AppService) ByAlternative(ctx context.Context, slug string) ([]AppItem, string, error) {
	rows, err := s.byAlt.get(ctx, slug, func(ctx context.Context) ([]AppItem, error) {
		pipeline := mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.M{"alternativeTo.slug": slug}}},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "metrics.stars", Value: -1}}}},
			bson.D{{Key: "$limit", Value: appTopN}},
			bson.D{{Key: "$project", Value: appProjection()}},
		}
		cur, err := s.store.Items().Aggregate(ctx, pipeline)
		if err != nil {
			return nil, fmt.Errorf("by-alternative aggregate: %w", err)
		}
		var raw []appRow
		if err := cur.All(ctx, &raw); err != nil {
			return nil, fmt.Errorf("by-alternative decode: %w", err)
		}
		return appItemsFromRows(raw), nil
	})
	if err != nil {
		return nil, "", err
	}
	// The display name comes from the matched apps' own alternativeTo entries.
	name := slug
	for _, a := range rows {
		for _, alt := range a.AlternativeTo {
			if alt.Slug == slug && alt.Name != "" {
				name = alt.Name
				break
			}
		}
		if name != slug {
			break
		}
	}
	return rows, name, nil
}
