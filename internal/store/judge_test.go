package store

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/elbaldfun/ghta/internal/item"
)

func queued(t *testing.T, st *Store, scope JudgeScope) []string {
	t.Helper()
	got, err := st.JudgeQueue(context.Background(), scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, c := range got {
		ids = append(ids, c.ExternalID)
	}
	return ids
}

// A new prompt generation sends judged items back, including ones judged
// before versions were recorded; the first generation leaves those alone.
func TestJudgeQueuePromptVersion(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	judged := func(id string, stars float64, prompt int) item.Item {
		it := item.Item{Source: item.SourceGitHub, ExternalID: id, Topics: []string{}, History: []item.Point{},
			Metrics: map[string]float64{"stars": stars}, Labels: item.Labels{Domain: []string{}, Version: 3}}
		it.Jobs.Judge = item.JobState{Status: item.StatusDone, Version: prompt}
		return it
	}
	for _, it := range []item.Item{judged("o/imported", 3000, 0), judged("o/v1", 2000, 1), judged("o/v2", 1000, 2)} {
		if _, err := st.Items().InsertOne(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	scope := JudgeScope{TreeVersion: 3, PromptVersion: 1, MinStars: 1000}
	if got := queued(t, st, scope); len(got) != 0 {
		t.Errorf("generation 1 re-queued %v", got)
	}
	scope.PromptVersion = 2
	if got := queued(t, st, scope); len(got) != 2 || got[0] != "o/imported" || got[1] != "o/v1" {
		t.Errorf("generation 2 queued %v", got)
	}
}

func TestJudgeFailedRetiresAtMax(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	res, err := st.Items().InsertOne(ctx, bson.M{"source": item.SourceGitHub, "externalId": "o/x"})
	if err != nil {
		t.Fatal(err)
	}
	for want := 1; want <= 2; want++ {
		if err := st.JudgeFailed(ctx, res.InsertedID.(primitive.ObjectID), 2, time.Now()); err != nil {
			t.Fatal(err)
		}
		got := getItem(t, st, "o/x").Jobs.Judge
		status := item.StatusPending
		if want == 2 {
			status = item.StatusFailed
		}
		if got.Fail != want || got.Status != status {
			t.Errorf("after %d failures: %+v", want, got)
		}
	}
}
