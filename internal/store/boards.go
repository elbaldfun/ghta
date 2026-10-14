package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Materialised rankings (docs/REDESIGN.md 5.2). A board is written whole by
// the nightly boards job and read whole by the api; nothing aggregates on a
// request.
const (
	CollBoards        = "boards"
	CollBoardsHistory = "boards_history"
)

// boardsHistoryTTL keeps a year of daily heads: enough for "up 3 this week"
// and for a yearly review.
const boardsHistoryTTL = 400 * 24 * time.Hour

// ErrNoBoard is returned for a board the job has not written.
var ErrNoBoard = errors.New("board not computed")

func (s *Store) Boards() *mongo.Collection { return s.DB.Collection(CollBoards) }

func (s *Store) BoardsHistory() *mongo.Collection { return s.DB.Collection(CollBoardsHistory) }

func (s *Store) ensureBoardIndexes(ctx context.Context) error {
	if _, err := s.Boards().Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("boards indexes: %w", err)
	}
	if _, err := s.BoardsHistory().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "key", Value: 1}, {Key: "d", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(int32(boardsHistoryTTL.Seconds()))},
	}); err != nil {
		return fmt.Errorf("boards_history indexes: %w", err)
	}
	return nil
}

// PutBoard replaces one board. rows is a slice of the board's row type.
func (s *Store) PutBoard(ctx context.Context, key string, rows any, at time.Time) error {
	_, err := s.Boards().UpdateOne(ctx, bson.M{"key": key},
		bson.M{"$set": bson.M{"rows": rows, "computedAt": at.UTC()}}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("put board %s: %w", key, err)
	}
	return nil
}

// Board decodes one board's rows into rows, a pointer to a slice of the
// board's row type, and returns when it was computed.
func (s *Store) Board(ctx context.Context, key string, rows any) (time.Time, error) {
	var doc struct {
		Rows       bson.RawValue `bson:"rows"`
		ComputedAt time.Time     `bson:"computedAt"`
	}
	err := s.Boards().FindOne(ctx, bson.M{"key": key}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, ErrNoBoard
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("board %s: %w", key, err)
	}
	if err := doc.Rows.Unmarshal(rows); err != nil {
		return time.Time{}, fmt.Errorf("board %s: %w", key, err)
	}
	return doc.ComputedAt, nil
}

// PutBoardHead records a board's head for one UTC day: the ids in order, the
// position being the index plus one. A rerun the same day replaces it.
func (s *Store) PutBoardHead(ctx context.Context, key, day string, ids []string, at time.Time) error {
	_, err := s.BoardsHistory().UpdateOne(ctx, bson.M{"key": key, "d": day},
		bson.M{"$set": bson.M{"ids": ids, "at": at.UTC()}}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("put board head %s %s: %w", key, day, err)
	}
	return nil
}

// BoardHead returns the ids a board's head held on a day, or nil.
func (s *Store) BoardHead(ctx context.Context, key, day string) ([]string, error) {
	var doc struct {
		IDs []string `bson:"ids"`
	}
	err := s.BoardsHistory().FindOne(ctx, bson.M{"key": key, "d": day}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("board head %s %s: %w", key, day, err)
	}
	return doc.IDs, nil
}
