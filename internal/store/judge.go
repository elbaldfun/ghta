package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

// JudgeCandidate is an item waiting for the judge, with what the model is shown.
type JudgeCandidate struct {
	ID          primitive.ObjectID `bson:"_id"`
	Source      string             `bson:"source"`
	ExternalID  string             `bson:"externalId"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	Language    string             `bson:"language"`
	Topics      []string           `bson:"topics"`
	Gh          struct {
		Platforms []string `bson:"platforms"`
	} `bson:"gh"`
	Labels struct {
		ShelfOverride string `bson:"shelfOverride"`
	} `bson:"labels"`
}

// JudgeScope says which items are due.
type JudgeScope struct {
	TreeVersion   int // items labelled on an older tree are due again
	PromptVersion int // so are items judged by an older prompt
	// The long tail is judged only once it moves: an item below MinStars
	// qualifies when its primary metric grew by HotWeekly in seven days.
	MinStars  float64
	HotWeekly float64
}

// JudgeQueue returns GitHub items due for the judge, most popular first.
// HuggingFace models never queue: their labels come from the source. Items a
// person labelled, stale ones and ones that failed for good are left alone.
func (s *Store) JudgeQueue(ctx context.Context, scope JudgeScope, limit int) ([]JudgeCandidate, error) {
	due := bson.A{
		bson.M{"jobs.judge.status": bson.M{"$ne": item.StatusDone}},
		bson.M{"labels.version": bson.M{"$lt": scope.TreeVersion}},
	}
	if scope.PromptVersion > 1 {
		// An item with no recorded prompt version was judged by generation 1.
		due = append(due, bson.M{"jobs.judge.version": bson.M{"$not": bson.M{"$gte": scope.PromptVersion}}})
	}
	filter := bson.M{
		"source":            item.SourceGitHub,
		"stale":             bson.M{"$ne": true},
		"labels.by":         bson.M{"$ne": item.ByManual},
		"jobs.judge.status": bson.M{"$ne": item.StatusFailed},
		"$and": bson.A{
			bson.M{"$or": due},
			bson.M{"$or": bson.A{
				bson.M{"metrics.stars": bson.M{"$gte": scope.MinStars}},
				bson.M{"growth.d7": bson.M{"$gte": scope.HotWeekly}},
			}},
		},
	}
	cur, err := s.Items().Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).SetLimit(int64(limit)).
		SetProjection(bson.M{"source": 1, "externalId": 1, "name": 1, "description": 1, "language": 1,
			"topics": 1, "gh.platforms": 1, "labels.shelfOverride": 1}))
	if err != nil {
		return nil, fmt.Errorf("judge queue: %w", err)
	}
	out := []JudgeCandidate{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("judge queue: %w", err)
	}
	return out, nil
}

// Readmes returns the stored README of each listed item that has one.
func (s *Store) Readmes(ctx context.Context, source string, externalIDs []string) (map[string]string, error) {
	cur, err := s.Blobs().Find(ctx, bson.M{"source": source, "externalId": bson.M{"$in": externalIDs}},
		options.Find().SetProjection(bson.M{"externalId": 1, "readme": 1}))
	if err != nil {
		return nil, fmt.Errorf("read readmes: %w", err)
	}
	var rows []struct {
		ExternalID string `bson:"externalId"`
		Readme     string `bson:"readme"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read readmes: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Readme != "" {
			out[r.ExternalID] = r.Readme
		}
	}
	return out, nil
}

// SetVerdict stores the judge's decision on one item and marks it done.
// labels carries everything the judge owns; the shelf override, which a
// person owns, is not touched.
func (s *Store) SetVerdict(ctx context.Context, id primitive.ObjectID, labels item.Labels, promptVersion int, at time.Time) error {
	at = at.UTC()
	set := bson.M{
		"labels.domain": nonNil(labels.Domain), "labels.type": labels.Type, "labels.shelf": labels.Shelf,
		"labels.hasGui": labels.HasGui, "labels.taglineZh": labels.TaglineZh, "labels.taglineEn": labels.TaglineEn,
		"labels.alternatives": labels.Alternatives, "labels.tags": nonNil(labels.Tags),
		"labels.by": item.ByLLM, "labels.confidence": labels.Confidence, "labels.version": labels.Version, "labels.at": at,
		"jobs.judge": item.JobState{Status: item.StatusDone, Version: promptVersion, At: at},
		"updatedAt":  at,
	}
	if labels.Alternatives == nil {
		set["labels.alternatives"] = []item.Alternative{}
	}
	if _, err := s.Items().UpdateByID(ctx, id, bson.M{"$set": set}); err != nil {
		return fmt.Errorf("set verdict: %w", err)
	}
	return nil
}

// JudgeFailed counts one unusable answer for an item; at max the item is
// marked failed and stops queueing.
func (s *Store) JudgeFailed(ctx context.Context, id primitive.ObjectID, max int, at time.Time) error {
	_, err := s.Items().UpdateByID(ctx, id, bson.A{bson.M{"$set": bson.M{
		"jobs.judge.fail": bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$jobs.judge.fail", 0}}, 1}},
		"jobs.judge.at":   at.UTC(),
		"jobs.judge.status": bson.M{"$cond": bson.A{
			bson.M{"$gte": bson.A{bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$jobs.judge.fail", 0}}, 1}}, max}},
			item.StatusFailed, item.StatusPending}},
	}}})
	if err != nil {
		return fmt.Errorf("judge failed: %w", err)
	}
	return nil
}
