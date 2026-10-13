package importer

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
)

func testDB(t *testing.T) *mongo.Database {
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
	db := client.Database("ghta_importer_test")
	if err := db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Drop(ctx); _ = client.Disconnect(ctx) })
	return db
}

func TestRunAndVerify(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 10, 12, 3, 30, 0, 0, time.UTC)

	week := 600.0
	docs := []any{
		bson.M{"source": "github", "externalId": "o/notes", "name": "notes", "categoryPath": bson.A{"web/frontend"},
			"primaryMetric": "stars", "metricDirection": "desc-better", "metrics": bson.M{"stars": 1000.0},
			"dailyIncrease": 100.0, "weeklyIncrease": week, "monthlyIncrease": nil,
			"analysisStatus": "done", "store": bson.M{"category": "productivity/notes", "status": "done", "version": 2},
			"sourceData": bson.M{"owner": "o", "readme": "# Notes", "topicNames": bson.A{"notes"},
				"releases": bson.A{bson.M{"tagName": "v1"}}, "mystery": 1},
			"fetchedAt": fetched, "createdAt": fetched, "updatedAt": fetched},
		// Legacy string path, a shelf that left the tree, no README.
		bson.M{"source": "github", "externalId": "o/old", "name": "old", "categoryPath": "ai/llm",
			"primaryMetric": "stars", "metrics": bson.M{"stars": 5.0}, "analysisStatus": "done",
			"store":      bson.M{"category": "devtools/devops", "status": "done"},
			"sourceData": bson.M{"owner": "o", "readme": ""}, "fetchedAt": fetched},
		bson.M{"source": "huggingface", "externalId": "a/m", "name": "m", "categoryPath": bson.A{"hf/text-generation"},
			"primaryMetric": "downloads", "metrics": bson.M{"downloads": 50.0}, "analysisStatus": "done",
			"sourceData": bson.M{"author": "a", "task": "text-generation"}, "fetchedAt": fetched},
	}
	if _, err := db.Collection(oldItems).InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	snap := func(id string, day int, hour int, stars float64) any {
		return bson.M{"meta": bson.M{"source": "github", "externalId": id},
			"metrics": bson.M{"stars": stars}, "capturedAt": time.Date(2026, 10, day, hour, 30, 0, 0, time.UTC)}
	}
	if _, err := db.Collection(oldSnapshots).InsertMany(ctx, []any{
		snap("o/notes", 5, 3, 400), snap("o/notes", 11, 3, 880), snap("o/notes", 11, 15, 900), snap("o/notes", 12, 3, 1000),
		// Outside the window, and another item's: neither may leak in.
		bson.M{"meta": bson.M{"source": "github", "externalId": "o/notes"}, "metrics": bson.M{"stars": 1.0},
			"capturedAt": time.Date(2026, 8, 1, 3, 30, 0, 0, time.UTC)},
		snap("o/other", 12, 3, 7),
	}); err != nil {
		t.Fatal(err)
	}

	rep, err := Run(ctx, db, testRules, Options{Now: now, Batch: 2})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep.Items["github"] != 2 || rep.Items["huggingface"] != 1 || rep.Blobs != 1 || rep.WithHistory != 1 {
		t.Errorf("report counts: %+v", rep)
	}
	if rep.DroppedPaths["ai/llm"] != 1 || rep.DroppedPaths["apps/devtools/devops"] != 1 || rep.UnknownKeys["mystery"] != 1 {
		t.Errorf("report notes: paths=%v keys=%v", rep.DroppedPaths, rep.UnknownKeys)
	}
	if d := rep.Growth["d7"]; d.Same != 1 || d.BothMissing != 2 {
		t.Errorf("d7 comparison: %+v", d)
	}
	if d := rep.Growth["d30"]; d.BothMissing != 3 {
		t.Errorf("d30 comparison: %+v", d)
	}

	var notes item.Item
	if err := db.Collection(item.CollItems).FindOne(ctx, bson.M{"externalId": "o/notes"}).Decode(&notes); err != nil {
		t.Fatal(err)
	}
	wantHistory := []item.Point{{D: "2026-10-05", V: 400}, {D: "2026-10-11", V: 900}, {D: "2026-10-12", V: 1000}}
	if len(notes.History) != 3 || notes.History[0] != wantHistory[0] || notes.History[1] != wantHistory[1] || notes.History[2] != wantHistory[2] {
		t.Errorf("history is one point per day, the day's last capture: %v", notes.History)
	}
	if g := notes.Growth; g.D1 == nil || *g.D1 != 100 || g.D7 == nil || *g.D7 != 600 || g.D30 != nil {
		t.Errorf("growth recomputed from the ring: %+v", g)
	}

	checks, err := Verify(ctx, db, testRules, 0)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, c := range checks {
		if !c.OK {
			t.Errorf("check %q failed: %s", c.Name, c.Detail)
		}
	}
	if len(checks) != 6 {
		t.Errorf("expected 6 checks, got %d", len(checks))
	}

	// Repeatable: a second run leaves the same result, not duplicates.
	if _, err := Run(ctx, db, testRules, Options{Now: now}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if n, _ := db.Collection(item.CollItems).CountDocuments(ctx, bson.M{}); n != 3 {
		t.Errorf("after a second run: %d items", n)
	}

	// Verify notices a stored item that no longer matches its source.
	if _, err := db.Collection(item.CollItems).UpdateOne(ctx, bson.M{"externalId": "o/notes"},
		bson.M{"$set": bson.M{"labels.shelf": "ai/assistant"}}); err != nil {
		t.Fatal(err)
	}
	checks, err = Verify(ctx, db, testRules, 0)
	if err != nil {
		t.Fatal(err)
	}
	if Passed(checks) {
		t.Error("a tampered shelf must fail verification")
	}
}
