package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CollJobRuns holds one document per job run.
const CollJobRuns = "job_runs"

// jobRunTTL is how long a run record is kept.
const jobRunTTL = 90 * 24 * time.Hour

// Run status values.
const (
	RunRunning     = "running"
	RunDone        = "done"
	RunFailed      = "failed"
	RunInterrupted = "interrupted" // the worker died or was stopped mid-run
)

// JobRun is the record of one run of one job.
type JobRun struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Job        string             `bson:"job" json:"job"`
	Worker     string             `bson:"worker,omitempty" json:"worker,omitempty"`
	Status     string             `bson:"status" json:"status"`
	StartedAt  time.Time          `bson:"startedAt" json:"startedAt"`
	FinishedAt time.Time          `bson:"finishedAt,omitempty" json:"finishedAt,omitempty"`
	DurationMs int64              `bson:"durationMs,omitempty" json:"durationMs,omitempty"`
	Processed  int                `bson:"processed" json:"processed"`
	Failed     int                `bson:"failed" json:"failed"`
	Notes      map[string]any     `bson:"notes,omitempty" json:"notes,omitempty"`
	Error      string             `bson:"error,omitempty" json:"error,omitempty"`
}

func (s *Store) JobRuns() *mongo.Collection { return s.DB.Collection(CollJobRuns) }

func (s *Store) ensureJobRunIndexes(ctx context.Context) error {
	_, err := s.JobRuns().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "job", Value: 1}, {Key: "startedAt", Value: -1}}},
		{Keys: bson.D{{Key: "startedAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(int32(jobRunTTL / time.Second))},
	})
	if err != nil {
		return fmt.Errorf("job_runs indexes: %w", err)
	}
	return nil
}

// StartRun records that a job began and returns the record's id.
func (s *Store) StartRun(ctx context.Context, job string, at time.Time) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	_, err := s.JobRuns().InsertOne(ctx, JobRun{ID: id, Job: job, Worker: s.Worker, Status: RunRunning, StartedAt: at.UTC()})
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("start run %s: %w", job, err)
	}
	return id, nil
}

// FinishRun closes a run record. Only Status, FinishedAt, Processed, Failed,
// Notes and Error of end are used; the duration is derived from the stored
// start time.
func (s *Store) FinishRun(ctx context.Context, id primitive.ObjectID, end JobRun) error {
	finished := end.FinishedAt.UTC()
	set := bson.M{
		"status":     end.Status,
		"finishedAt": finished,
		"processed":  end.Processed,
		"failed":     end.Failed,
		"durationMs": bson.M{"$dateDiff": bson.M{"startDate": "$startedAt", "endDate": finished, "unit": "millisecond"}},
	}
	if len(end.Notes) > 0 {
		set["notes"] = bson.M{"$literal": end.Notes}
	}
	if end.Error != "" {
		set["error"] = bson.M{"$literal": end.Error}
	}
	// A pipeline update, so the duration can be computed from the stored start.
	_, err := s.JobRuns().UpdateByID(ctx, id, mongo.Pipeline{{{Key: "$set", Value: set}}})
	if err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return nil
}

// CloseInterruptedRuns marks this worker's runs still recorded as running as
// interrupted. A worker calls it on startup: one process runs under a worker
// name at a time, so such a run belongs to a process that no longer exists.
// Another worker's runs are not touched.
func (s *Store) CloseInterruptedRuns(ctx context.Context, at time.Time) (int, error) {
	mine := bson.M{"worker": s.Worker}
	if s.Worker == "" {
		mine = bson.M{"worker": bson.M{"$exists": false}}
	}
	mine["status"] = RunRunning
	res, err := s.JobRuns().UpdateMany(ctx, mine,
		bson.M{"$set": bson.M{"status": RunInterrupted, "finishedAt": at.UTC()}})
	if err != nil {
		return 0, fmt.Errorf("close interrupted runs: %w", err)
	}
	return int(res.ModifiedCount), nil
}

// Runs returns the latest runs, newest first. job "" means every job.
func (s *Store) Runs(ctx context.Context, job string, limit int) ([]JobRun, error) {
	filter := bson.M{}
	if job != "" {
		filter["job"] = job
	}
	cur, err := s.JobRuns().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "startedAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	runs := []JobRun{}
	if err := cur.All(ctx, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// LatestRuns returns each job's latest run, by job name.
func (s *Store) LatestRuns(ctx context.Context) ([]JobRun, error) {
	cur, err := s.JobRuns().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$sort", Value: bson.D{{Key: "job", Value: 1}, {Key: "startedAt", Value: -1}}}},
		{{Key: "$group", Value: bson.M{"_id": "$job", "run": bson.M{"$first": "$$ROOT"}}}},
		{{Key: "$replaceWith", Value: "$run"}},
		{{Key: "$sort", Value: bson.D{{Key: "job", Value: 1}}}},
	})
	if err != nil {
		return nil, fmt.Errorf("latest runs: %w", err)
	}
	runs := []JobRun{}
	if err := cur.All(ctx, &runs); err != nil {
		return nil, fmt.Errorf("latest runs: %w", err)
	}
	return runs, nil
}

// LastDone returns when job last finished a run without failing; ok is false
// if it never has.
func (s *Store) LastDone(ctx context.Context, job string) (at time.Time, ok bool, err error) {
	var run JobRun
	err = s.JobRuns().FindOne(ctx, bson.M{"job": job, "status": RunDone},
		options.FindOne().SetSort(bson.D{{Key: "startedAt", Value: -1}})).Decode(&run)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("last %s run: %w", job, err)
	}
	return run.FinishedAt, true, nil
}
