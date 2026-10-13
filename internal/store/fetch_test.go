package store

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
)

func repo(id string, stars float64, readme string) item.Fetched {
	it := item.Item{
		Source: item.SourceGitHub, ExternalID: id, Name: id, Description: "d", Language: "Go",
		Homepage: "https://example.com", License: "MIT", Topics: []string{"cli"},
		PrimaryMetric: "stars", MetricDirection: "desc-better",
		Metrics: map[string]float64{"stars": stars, "forks": 1},
		Gh:      &item.GitHub{Owner: "o", Platforms: []string{"macos"}},
	}
	return item.Fetched{Item: it, Blob: item.NewBlob(it.Source, id, readme, nil, time.Time{})}
}

func getItem(t *testing.T, st *Store, id string) item.Item {
	t.Helper()
	var it item.Item
	if err := st.Items().FindOne(context.Background(), bson.M{"externalId": id}).Decode(&it); err != nil {
		t.Fatalf("item %s: %v", id, err)
	}
	return it
}

func count(t *testing.T, st *Store, coll string, filter bson.M) int64 {
	t.Helper()
	n, err := st.DB.Collection(coll).CountDocuments(context.Background(), filter)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWriteFetchedOwnsOnlyItsFields(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	day1 := time.Date(2026, 10, 12, 3, 30, 0, 0, time.UTC)

	// Day 1: a new item.
	stats, err := st.WriteFetched(ctx, []item.Fetched{repo("o/a", 1000, "# A")}, day1)
	if err != nil || stats != (FetchStats{Items: 1, New: 1, Snapshots: 1, Blobs: 1}) {
		t.Fatalf("first write = %+v, %v", stats, err)
	}
	it := getItem(t, st, "o/a")
	if it.Labels.Domain == nil || len(it.Labels.Domain) != 0 || it.Jobs.Judge.Status != item.StatusPending ||
		!it.CreatedAt.Equal(day1) || !it.FetchedAt.Equal(day1) {
		t.Fatalf("new item defaults: labels=%+v jobs=%+v created=%v", it.Labels, it.Jobs, it.CreatedAt)
	}
	if len(it.History) != 1 || it.History[0] != (item.Point{D: "2026-10-12", V: 1000}) || it.Growth.D1 != nil {
		t.Fatalf("history=%v growth=%+v; a first observation has no growth", it.History, it.Growth)
	}

	// Other jobs do their work on the item.
	gui := true
	_, err = st.Items().UpdateOne(ctx, bson.M{"externalId": "o/a"}, bson.M{"$set": bson.M{
		"labels": item.Labels{Domain: []string{"devtools/cli"}, Shelf: "dev/cli", HasGui: &gui, By: item.ByLLM, Version: 3},
		"assets": item.Assets{IconURL: "https://example.com/i.png"},
		"ranks":  item.Ranks{Overall: 7},
		"jobs":   item.Jobs{Judge: item.JobState{Status: item.StatusDone, Fail: 1}, Icon: item.JobState{Status: item.StatusDone}},
		"stale":  true,
	}})
	if err != nil {
		t.Fatal(err)
	}

	// Day 2: more stars, the homepage is gone, the README is unchanged.
	day2 := day1.Add(24 * time.Hour)
	next := repo("o/a", 1250, "# A")
	next.Item.Homepage, next.Item.Description = "", "new description"
	stats, err = st.WriteFetched(ctx, []item.Fetched{next}, day2)
	if err != nil || stats != (FetchStats{Items: 1, Snapshots: 1}) {
		t.Fatalf("second write = %+v, %v; an unchanged README must not be rewritten", stats, err)
	}
	it = getItem(t, st, "o/a")
	if it.Description != "new description" || it.Homepage != "" || it.Metrics["stars"] != 1250 || !it.FetchedAt.Equal(day2) {
		t.Errorf("fetcher fields not refreshed: %+v", it)
	}
	if n := count(t, st, item.CollItems, bson.M{"externalId": "o/a", "homepage": bson.M{"$exists": true}}); n != 0 {
		t.Error("an emptied optional field must be removed, not stored as \"\"")
	}
	if len(it.Labels.Domain) != 1 || it.Labels.Shelf != "dev/cli" || it.Labels.HasGui == nil || it.Labels.Version != 3 ||
		it.Assets.IconURL == "" || it.Ranks == nil || it.Ranks.Overall != 7 ||
		it.Jobs.Judge.Status != item.StatusDone || it.Jobs.Judge.Fail != 1 || it.Jobs.Icon.Status != item.StatusDone ||
		!it.Stale || !it.CreatedAt.Equal(day1) {
		t.Errorf("fields owned by other jobs were touched: labels=%+v assets=%+v ranks=%+v jobs=%+v stale=%v created=%v",
			it.Labels, it.Assets, it.Ranks, it.Jobs, it.Stale, it.CreatedAt)
	}
	if len(it.History) != 2 || it.Growth.D1 == nil || *it.Growth.D1 != 250 || it.Growth.D7 != nil {
		t.Errorf("history=%v growth.d1=%v d7=%v", it.History, it.Growth.D1, it.Growth.D7)
	}

	// Later the same day, from an overlapping shard: the item is refreshed, the
	// day's point is replaced, and no second snapshot is appended.
	stats, err = st.WriteFetched(ctx, []item.Fetched{repo("o/a", 1260, "# A v2")}, day2.Add(time.Hour))
	if err != nil || stats != (FetchStats{Items: 1, Blobs: 1}) {
		t.Fatalf("same-day write = %+v, %v", stats, err)
	}
	it = getItem(t, st, "o/a")
	if len(it.History) != 2 || it.History[1].V != 1260 || *it.Growth.D1 != 260 {
		t.Errorf("same-day refresh: history=%v d1=%v", it.History, *it.Growth.D1)
	}
	if n := count(t, st, CollSnapshots, bson.M{"meta.source": "github", "meta.externalId": "o/a"}); n != 2 {
		t.Errorf("%d snapshots, want one per day", n)
	}
	var blob item.Blob
	if err := st.Blobs().FindOne(ctx, bson.M{"externalId": "o/a"}).Decode(&blob); err != nil ||
		blob.Readme != "# A v2" || blob.Hash != item.BlobHash("# A v2", nil) || !blob.UpdatedAt.Equal(day2.Add(time.Hour)) {
		t.Errorf("blob = %+v, %v", blob, err)
	}
}

func TestWriteFetchedBatchRules(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 3, 30, 0, 0, time.UTC)

	// The same repo twice in one shard: written once, the later listing wins.
	stats, err := st.WriteFetched(ctx, []item.Fetched{repo("o/a", 10, ""), repo("o/b", 5, ""), repo("o/a", 11, "")}, now)
	if err != nil || stats != (FetchStats{Items: 2, New: 2, Snapshots: 2}) {
		t.Fatalf("stats = %+v, %v", stats, err)
	}
	if getItem(t, st, "o/a").Metrics["stars"] != 11 {
		t.Error("the last occurrence in a batch must win")
	}

	model := item.Fetched{Item: item.Item{Source: item.SourceHuggingFace, ExternalID: "a/m", PrimaryMetric: "likes",
		Metrics: map[string]float64{"likes": 1}}}
	if _, err := st.WriteFetched(ctx, []item.Fetched{repo("o/c", 1, ""), model}, now); err == nil {
		t.Error("a batch mixing sources must be rejected")
	}
	if s, err := st.WriteFetched(ctx, nil, now); err != nil || s != (FetchStats{}) {
		t.Errorf("empty batch = %+v, %v", s, err)
	}

	// The ring keeps HistoryDays points.
	full := make([]item.Point, 0, item.HistoryDays)
	for d := item.HistoryDays; d >= 1; d-- {
		full = append(full, item.Point{D: item.DayOf(now.AddDate(0, 0, -d)), V: float64(100 - d)})
	}
	if _, err := st.Items().UpdateOne(ctx, bson.M{"externalId": "o/b"}, bson.M{"$set": bson.M{"history": full}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.WriteFetched(ctx, []item.Fetched{repo("o/b", 200, "")}, now); err != nil {
		t.Fatal(err)
	}
	b := getItem(t, st, "o/b")
	if len(b.History) != item.HistoryDays || b.History[len(b.History)-1].V != 200 || b.History[0].D != full[1].D {
		t.Errorf("ring: %d points, first %v, last %v", len(b.History), b.History[0], b.History[len(b.History)-1])
	}
	if b.Growth.D1 == nil || *b.Growth.D1 != 101 || b.Growth.D7 == nil || *b.Growth.D7 != 107 || b.Growth.D30 == nil || *b.Growth.D30 != 130 {
		t.Errorf("growth from the ring = %v %v %v", b.Growth.D1, b.Growth.D7, b.Growth.D30)
	}
}

func TestWriteFetchedSourceClassifiedItem(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 3, 30, 0, 0, time.UTC)
	model := func(task string) item.Fetched {
		return item.Fetched{Item: item.Item{
			Source: item.SourceHuggingFace, ExternalID: "a/m", Name: "m", Topics: []string{},
			PrimaryMetric: "likes", MetricDirection: "desc-better", Metrics: map[string]float64{"likes": 9},
			Labels: item.Labels{Domain: []string{"hf/" + task}, By: item.BySource},
			Hf:     &item.HuggingFace{Author: "a", Task: task},
		}}
	}
	if _, err := st.WriteFetched(ctx, []item.Fetched{model("text-gen")}, now); err != nil {
		t.Fatal(err)
	}
	it := getItem(t, st, "a/m")
	if len(it.Labels.Domain) != 1 || it.Labels.Domain[0] != "hf/text-gen" || it.Labels.By != item.BySource ||
		it.Jobs.Judge.Status != item.StatusDone || it.Hf == nil || it.Gh != nil {
		t.Fatalf("model = labels %+v jobs %+v hf %+v", it.Labels, it.Jobs, it.Hf)
	}
	// The Hub re-tags the model: its classification follows.
	if _, err := st.WriteFetched(ctx, []item.Fetched{model("embeddings")}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if it = getItem(t, st, "a/m"); it.Labels.Domain[0] != "hf/embeddings" || it.Hf.Task != "embeddings" {
		t.Errorf("after re-tag: %+v %+v", it.Labels, it.Hf)
	}
	if n := count(t, st, item.CollBlobs, bson.M{}); n != 0 {
		t.Errorf("%d blobs for a model", n)
	}
}

func TestShardStatus(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	at := time.Date(2026, 10, 12, 3, 30, 0, 0, time.UTC)

	if s, err := st.ShardStatus(ctx, "github", "2026-10-12", "1000..1010"); err != nil || s != "" {
		t.Fatalf("unknown shard = %q, %v", s, err)
	}
	for _, status := range []string{ShardRunning, ShardFailed, ShardRunning, ShardDone} {
		if err := st.SetShardStatus(ctx, "github", "2026-10-12", "1000..1010", status, "", at); err != nil {
			t.Fatal(err)
		}
	}
	if s, _ := st.ShardStatus(ctx, "github", "2026-10-12", "1000..1010"); s != ShardDone {
		t.Errorf("status = %q", s)
	}
	if s, _ := st.ShardStatus(ctx, "github", "2026-10-13", "1000..1010"); s != "" {
		t.Errorf("another day's status = %q", s)
	}
	if n := count(t, st, CollFetchRuns, bson.M{"startedAt": at, "endedAt": at}); n != 1 {
		t.Errorf("%d fetch_runs documents with both timestamps, want 1", n)
	}

	cur, err := st.FetchRuns().Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var idx []bson.M
	if err := cur.All(ctx, &idx); err != nil {
		t.Fatal(err)
	}
	ttl := false
	for _, i := range idx {
		if v, ok := i["expireAfterSeconds"].(int32); ok && v == 30*24*3600 {
			ttl = true
		}
	}
	if !ttl {
		t.Errorf("no 30-day TTL index on fetch_runs: %v", idx)
	}
}
