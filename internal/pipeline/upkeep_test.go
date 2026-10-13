package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/source/github"
	"github.com/elbaldfun/ghta/internal/store"
)

func find(t *testing.T, st *store.Store, id string) (item.Item, bool) {
	t.Helper()
	var it item.Item
	err := st.Items().FindOne(context.Background(), bson.M{"externalId": id}).Decode(&it)
	return it, err == nil
}

type fakeGitHub struct {
	repos  map[string]*github.RepoInfo // by the name asked for; missing = 404
	users  map[string]*github.UserProfile
	broken map[string]bool
	rate   github.RateInfo
	asked  []string
}

func (f *fakeGitHub) FetchRepo(_ context.Context, name string) (*github.RepoInfo, github.RateInfo, error) {
	f.asked = append(f.asked, name)
	if f.broken[name] {
		return nil, f.rate, errors.New("status 500")
	}
	if r, ok := f.repos[name]; ok {
		return r, f.rate, nil
	}
	return nil, f.rate, github.ErrRepoNotFound
}

func (f *fakeGitHub) FetchUser(_ context.Context, login string) (*github.UserProfile, github.RateInfo, error) {
	f.asked = append(f.asked, login)
	if f.broken[login] {
		return nil, f.rate, errors.New("status 500")
	}
	if u, ok := f.users[login]; ok {
		return u, f.rate, nil
	}
	return nil, f.rate, github.ErrUserNotFound
}

func TestReconcile(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 7, 0, 0, 0, time.UTC)
	old := now.Add(-10 * 24 * time.Hour)

	frozen := func(id string, stars float64, age time.Duration) item.Item {
		it := gh(id, "Go", stars)
		it.Name, it.FetchedAt = id[2:], now.Add(-age)
		it.Gh = &item.GitHub{Owner: "o"}
		it.History = []item.Point{{D: item.DayOf(old), V: stars}}
		return it
	}
	tracked := frozen("new/taken", 50, time.Hour) // fetched an hour ago: not a candidate
	docs := []any{
		frozen("o/gone", 10, 12*24*time.Hour),
		frozen("o/alive", 900, 11*24*time.Hour),
		frozen("o/moved", 300, 10*24*time.Hour),
		frozen("o/ghost", 200, 9*24*time.Hour),
		frozen("o/flaky", 100, 8*24*time.Hour),
		tracked,
	}
	if _, err := st.Items().InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Blobs().InsertOne(ctx, item.NewBlob(item.SourceGitHub, "o/moved", "# moved", nil, old)); err != nil {
		t.Fatal(err)
	}

	api := &fakeGitHub{
		repos: map[string]*github.RepoInfo{
			"o/alive": {FullName: "O/Alive", Stars: 940, Forks: 3}, // case differences are not a rename
			"o/moved": {FullName: "acme/renamed", Stars: 320},
			"o/ghost": {FullName: "new/taken", Stars: 50},
		},
		broken: map[string]bool{"o/flaky": true},
	}
	job := NewReconcile(st, api, 100, nil)
	job.now = func() time.Time { return now }
	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 4 || rep.Failed != 1 {
		t.Fatalf("report = %+v, %v", rep, err)
	}
	for verdict, want := range map[string]int{"gone": 1, "refreshed": 1, "renamed": 1, "merged": 1} {
		if rep.Notes[verdict] != want {
			t.Errorf("notes[%s] = %v, want %d (%v)", verdict, rep.Notes[verdict], want, rep.Notes)
		}
	}
	if len(api.asked) != 5 || api.asked[0] != "o/gone" {
		t.Errorf("asked %v; want the five old records, longest-untouched first", api.asked)
	}

	if it, _ := find(t, st, "o/gone"); !it.Stale || it.StaleReason != "gone" || !it.StaleAt.Equal(now) {
		t.Errorf("gone: %+v", it)
	}
	if it, _ := find(t, st, "o/ghost"); !it.Stale || it.StaleReason != "renamed:new/taken" {
		t.Errorf("ghost: stale=%v reason=%q", it.Stale, it.StaleReason)
	}
	if it, _ := find(t, st, "new/taken"); it.Stale || it.Metrics["stars"] != 50 {
		t.Errorf("the record already under the new name was touched: %+v", it)
	}
	// A refresh is an observation: metrics, a history point, growth, a snapshot.
	alive, _ := find(t, st, "o/alive")
	if alive.Metrics["stars"] != 940 || !alive.FetchedAt.Equal(now) || len(alive.History) != 2 ||
		alive.Growth.D7 == nil || *alive.Growth.D7 != 40 {
		t.Errorf("alive: metrics=%v history=%v growth=%+v", alive.Metrics, alive.History, alive.Growth)
	}
	if n, _ := st.Snapshots().CountDocuments(ctx, bson.M{"meta.externalId": "o/alive"}); n != 1 {
		t.Errorf("%d snapshots for the refreshed repo", n)
	}
	// A rename carries the record, its history and its blob.
	if _, still := find(t, st, "o/moved"); still {
		t.Error("the old name is still there after a rename")
	}
	moved, ok := find(t, st, "acme/renamed")
	if !ok || moved.Name != "renamed" || moved.Gh.Owner != "acme" || moved.Metrics["stars"] != 320 ||
		len(moved.History) != 2 || moved.History[0].V != 300 {
		t.Errorf("moved: %+v gh=%+v", moved, moved.Gh)
	}
	if readme, _ := st.Readme(ctx, item.SourceGitHub, "acme/renamed"); readme != "# moved" {
		t.Errorf("blob did not follow the rename: %q", readme)
	}
	if it, _ := find(t, st, "o/flaky"); it.Stale || !it.FetchedAt.Before(now) {
		t.Errorf("a failed check must leave the record alone: %+v", it)
	}

	// Budget low: the sweep waits for the window to reset after each call.
	var waited []time.Duration
	api.rate = github.RateInfo{Remaining: 50, Reset: now.Add(10 * time.Minute)}
	job.wait = func(_ context.Context, d time.Duration) { waited = append(waited, d) }
	if _, err := job.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(waited) != 1 || waited[0] != 10*time.Minute+rateMargin {
		t.Errorf("waits = %v; want one wait until the reset for the one remaining candidate", waited)
	}
}

func TestIconAndShot(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 7, 0, 0, 0, time.UTC)

	app := func(id string, stars float64, homepage string) item.Item {
		it := gh(id, "Go", stars)
		it.Labels.Type, it.Homepage = "app", homepage
		return it
	}
	lib := app("o/lib", 9000, "https://lib.dev")
	lib.Labels.Type = "library"
	excluded := app("o/excluded", 8000, "https://x.dev")
	excluded.Labels.Shelf = item.ShelfExcluded
	builds := gh("o/builds", "Go", 700) // no type yet, but ships platform builds
	builds.Homepage, builds.Gh = "https://builds.dev", &item.GitHub{Owner: "o", Platforms: []string{"macos"}}
	done := app("o/done", 600, "https://done.dev")
	done.Jobs.Icon.Status, done.Jobs.Shot.Status = item.StatusDone, item.StatusDone
	docs := []any{
		app("o/small", 100, "https://small.dev"), app("o/big", 500, "https://big.dev"),
		app("o/nohome", 400, ""), lib, excluded, builds, done,
	}
	if _, err := st.Items().InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Blobs().InsertOne(ctx, item.NewBlob(item.SourceGitHub, "o/big", "![shot](a.png)", nil, now)); err != nil {
		t.Fatal(err)
	}

	var order []string
	ic := NewIcon(st, nil)
	ic.pace, ic.now = 0, func() time.Time { return now }
	ic.extract = func(_ context.Context, homepage string) string {
		order = append(order, homepage)
		if homepage == "https://big.dev" {
			return "https://big.dev/icon.png"
		}
		return ""
	}
	rep, err := ic.Run(ctx)
	if err != nil || rep.Processed != 3 || rep.Notes["found"] != 1 {
		t.Fatalf("icon report = %+v, %v", rep, err)
	}
	if len(order) != 3 || order[0] != "https://builds.dev" || order[1] != "https://big.dev" || order[2] != "https://small.dev" {
		t.Errorf("icon order = %v; want apps with a homepage, most popular first", order)
	}
	big, _ := find(t, st, "o/big")
	small, _ := find(t, st, "o/small")
	if big.Assets.IconURL != "https://big.dev/icon.png" || big.Jobs.Icon.Status != item.StatusDone || !big.Jobs.Icon.At.Equal(now) ||
		small.Assets.IconURL != "" || small.Jobs.Icon.Status != item.StatusDone {
		t.Errorf("icon verdicts: big=%+v/%+v small=%+v/%+v", big.Assets, big.Jobs.Icon, small.Assets, small.Jobs.Icon)
	}
	if rep, _ = ic.Run(ctx); rep.Processed != 0 {
		t.Errorf("icon rerun processed %d; every app is processed once", rep.Processed)
	}

	sh := NewShot(st, nil)
	sh.pace, sh.now = 0, func() time.Time { return now }
	sh.find = func(_ context.Context, id, readme, homepage string) (string, string) {
		switch {
		case readme != "":
			return "https://raw/" + id + "/a.png", "readme"
		case homepage == "https://small.dev":
			return "https://small.dev/og.png", "og"
		}
		return "", ""
	}
	rep, err = sh.Run(ctx)
	if err != nil || rep.Processed != 4 || rep.Notes["found"] != 2 || rep.Notes["viaOg"] != 1 {
		t.Fatalf("shot report = %+v, %v; want big, nohome, builds and small", rep, err)
	}
	big, _ = find(t, st, "o/big")
	small, _ = find(t, st, "o/small")
	nohome, _ := find(t, st, "o/nohome")
	if big.Assets.ShotSource != "readme" || big.Assets.IconURL == "" || small.Assets.ShotSource != "og" ||
		small.Assets.ScreenshotURL != "https://small.dev/og.png" || nohome.Jobs.Shot.Status != item.StatusDone || nohome.Assets.ScreenshotURL != "" {
		t.Errorf("shot verdicts: big=%+v small=%+v nohome=%+v", big.Assets, small.Assets, nohome.Jobs.Shot)
	}
	for _, id := range []string{"o/lib", "o/excluded"} {
		if it, _ := find(t, st, id); it.Jobs.Icon.Status != "" || it.Jobs.Shot.Status != "" {
			t.Errorf("%s is outside the app corpus but was processed: %+v", id, it.Jobs)
		}
	}
}

func TestDevSync(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)

	owned := func(id, owner string) item.Item {
		it := gh(id, "Go", 1)
		it.Gh = &item.GitHub{Owner: owner}
		return it
	}
	docs := []any{owned("a/x", "a"), owned("a/y", "a"), owned("b/x", "b"), owned("c/x", "c"), owned("d/x", "d"), owned("e/x", "e")}
	if _, err := st.Items().InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	// b was fetched yesterday; c three weeks ago and carries a field another job owns.
	if _, err := st.Developers().InsertMany(ctx, []any{
		bson.M{"login": "b", "followers": 1, "fetchedAt": now.Add(-24 * time.Hour)},
		bson.M{"login": "c", "followers": 1, "fetchedAt": now.Add(-21 * 24 * time.Hour), "merit": 42},
	}); err != nil {
		t.Fatal(err)
	}

	api := &fakeGitHub{
		users: map[string]*github.UserProfile{
			"a": {Login: "a", Type: domain.DeveloperOrg, Followers: 10},
			"c": {Login: "c", Type: domain.DeveloperUser, Followers: 7},
		},
		broken: map[string]bool{"e": true},
	}
	job := NewDevSync(st, api, 100, nil)
	job.now = func() time.Time { return now }
	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 2 || rep.Failed != 1 ||
		rep.Notes["owners"] != 5 || rep.Notes["fresh"] != 1 || rep.Notes["notFound"] != 1 {
		t.Fatalf("report = %+v, %v", rep, err)
	}
	var c bson.M
	if err := st.Developers().FindOne(ctx, bson.M{"login": "c"}).Decode(&c); err != nil {
		t.Fatal(err)
	}
	if c["followers"] != int32(7) || c["merit"] != int32(42) || c["type"] != domain.DeveloperUser {
		t.Errorf("c after refresh = %v; the profile is replaced, other fields kept", c)
	}
	if n, _ := st.Developers().CountDocuments(ctx, bson.M{}); n != 3 {
		t.Errorf("%d developers, want a, b, c", n)
	}

	// Nothing is due right after a sweep except what failed or was not found.
	api.asked = nil
	if _, err := job.Run(ctx); err != nil || len(api.asked) != 2 {
		t.Errorf("second sweep asked for %v, %v; want only d and e", api.asked, err)
	}
}
