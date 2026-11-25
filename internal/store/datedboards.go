package store

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

	"github.com/elbaldfun/ghta/internal/item"
)

// Dated boards are keyed "<prefix><YYYY-MM-DD>": one board per day, the
// newest being the one a page shows (the ios-new boards).

func keyPrefix(prefix string) bson.M {
	return bson.M{"key": primitive.Regex{Pattern: "^" + regexp.QuoteMeta(prefix)}}
}

// LatestDatedBoard returns the key of the newest board under prefix, ""
// when there is none. Day keys sort as text.
func (s *Store) LatestDatedBoard(ctx context.Context, prefix string) (string, error) {
	var doc struct {
		Key string `bson:"key"`
	}
	err := s.Boards().FindOne(ctx, keyPrefix(prefix),
		options.FindOne().SetSort(bson.D{{Key: "key", Value: -1}}).SetProjection(bson.M{"key": 1})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("latest board %s: %w", prefix, err)
	}
	return doc.Key, nil
}

// DeleteDatedBoards removes the boards under prefix whose day is before
// day: "<prefix><country>|<day>" alike, the day being the last segment.
func (s *Store) DeleteDatedBoards(ctx context.Context, prefix, day string) (int64, error) {
	cur, err := s.Boards().Find(ctx, keyPrefix(prefix), options.Find().SetProjection(bson.M{"key": 1}))
	if err != nil {
		return 0, fmt.Errorf("dated boards %s: %w", prefix, err)
	}
	var docs []struct {
		Key string `bson:"key"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return 0, fmt.Errorf("dated boards %s: %w", prefix, err)
	}
	var old []string
	for _, d := range docs {
		if d.Key[strings.LastIndex(d.Key, "|")+1:] < day {
			old = append(old, d.Key)
		}
	}
	if len(old) == 0 {
		return 0, nil
	}
	res, err := s.Boards().DeleteMany(ctx, bson.M{"key": bson.M{"$in": old}})
	if err != nil {
		return 0, fmt.Errorf("delete dated boards %s: %w", prefix, err)
	}
	return res.DeletedCount, nil
}

// ChartedSince returns the store items of a source whose chart places are
// those of day and whose first release is at or after since: what the
// new-apps boards rank. Only the fields those boards read are filled.
func (s *Store) ChartedSince(ctx context.Context, source, day string, since time.Time) ([]item.Item, error) {
	cur, err := s.Items().Find(ctx, bson.M{
		"source": source, "stale": bson.M{"$ne": true},
		"app.chartsDay": day, "app.releasedAt": bson.M{"$gte": since.UTC()},
	}, options.Find().SetProjection(bson.M{
		"externalId": 1, "stale": 1, "app.charts": 1, "app.chartsDay": 1, "app.releasedAt": 1,
	}))
	if err != nil {
		return nil, fmt.Errorf("charted since: %w", err)
	}
	var out []item.Item
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("charted since: %w", err)
	}
	return out, nil
}
