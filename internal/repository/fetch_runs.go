package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
)

// ShardStatus returns the status of a source+date+shard, or "" if not recorded.
func (s *Store) ShardStatus(ctx context.Context, src domain.Source, date, shard string) (domain.FetchRunStatus, error) {
	var run domain.FetchRun
	err := s.FetchRuns().FindOne(ctx, bson.M{"source": src, "date": date, "shard": shard}).Decode(&run)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return run.Status, nil
}

// SetShardStatus upserts the run record for a shard, stamping timing and error.
func (s *Store) SetShardStatus(ctx context.Context, src domain.Source, date, shard string, status domain.FetchRunStatus, errMsg string) error {
	now := time.Now().UTC()
	set := bson.M{"status": status, "error": errMsg}
	switch status {
	case domain.FetchRunning:
		set["startedAt"] = now
	case domain.FetchDone, domain.FetchFailed:
		set["endedAt"] = now
	}
	_, err := s.FetchRuns().UpdateOne(ctx,
		bson.M{"source": src, "date": date, "shard": shard},
		bson.M{"$set": set, "$setOnInsert": bson.M{"source": src, "date": date, "shard": shard}},
		options.Update().SetUpsert(true),
	)
	return err
}
