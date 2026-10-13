package store

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("ghta_store_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Drop(ctx); _ = client.Disconnect(ctx) })
	st := New(db)
	if err := st.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestJobRunLifecycle(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 13, 3, 30, 0, 0, time.UTC)

	id, err := st.StartRun(ctx, "rank", t0)
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := st.StartRun(ctx, "fetch", t0.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	err = st.FinishRun(ctx, id, JobRun{
		Status: RunFailed, FinishedAt: t0.Add(1500 * time.Millisecond), Processed: 10, Failed: 2,
		// "$" and dots must be stored as data, not evaluated by the pipeline update.
		Notes: map[string]any{"written": 8, "odd": "$startedAt"}, Error: "boom: $x",
	})
	if err != nil {
		t.Fatal(err)
	}

	runs, err := st.Runs(ctx, "rank", 5)
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %v, %v", runs, err)
	}
	r := runs[0]
	if r.Status != RunFailed || r.DurationMs != 1500 || r.Processed != 10 || r.Failed != 2 ||
		r.Error != "boom: $x" || r.Notes["odd"] != "$startedAt" || !r.StartedAt.Equal(t0) {
		t.Fatalf("stored run = %+v", r)
	}

	n, err := st.CloseInterruptedRuns(ctx, t0.Add(time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("closed %d runs, %v; want the one orphan", n, err)
	}
	all, err := st.Runs(ctx, "", 5)
	if err != nil || len(all) != 2 || all[0].ID != id || all[1].ID != orphan || all[1].Status != RunInterrupted {
		t.Fatalf("all runs (newest first) = %+v, %v", all, err)
	}
}

func TestJobRunsExpire(t *testing.T) {
	st := testStore(t)
	cur, err := st.JobRuns().Indexes().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var idx []bson.M
	if err := cur.All(context.Background(), &idx); err != nil {
		t.Fatal(err)
	}
	for _, i := range idx {
		if ttl, ok := i["expireAfterSeconds"].(int32); ok && ttl == 90*24*3600 {
			return
		}
	}
	t.Fatalf("no 90-day TTL index on job_runs: %v", idx)
}
