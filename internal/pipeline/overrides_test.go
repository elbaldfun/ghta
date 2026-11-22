package pipeline

import (
	"context"
	"slices"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/store"
)

func TestOverridesJob(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC)

	course := gh("fcc/fcc", "TypeScript", 400000, "web/frontend")
	course.Labels.Type, course.Labels.Shelf, course.Labels.By = "app", "excluded", item.ByLLM
	course.Labels.TaglineZh = "免费学编程"
	course.Jobs.Judge.Status = item.StatusDone
	// Not judged yet: labelled by the old rules, no tagline.
	photos := gh("im/im", "TypeScript", 90000, "apps/selfhosted/cloud", "data/database")
	photos.Labels.Type, photos.Labels.Shelf, photos.Labels.By = "app", "selfhosted/cloud", item.ByRule
	other := gh("o/other", "Go", 10, "data/database")
	phoneApp := gh("p/app", "Kotlin", 500, "apps/system/phone", "client/android")
	phoneApp.Labels.Shelf = "system/phone"
	// A developer app, with a second shelf's leaf beside its own.
	termux := gh("t/termux", "Java", 500, "devtools/terminal", "client/android", "apps/system/phone")
	termux.Labels.Shelf = "devtools/terminal"
	for _, it := range []item.Item{course, photos, other, phoneApp, termux} {
		if _, err := st.Items().InsertOne(ctx, it); err != nil {
			t.Fatal(err)
		}
	}
	list := []judge.Override{
		{ID: "fcc/fcc", Type: "tutorial", Shelf: "excluded", Why: "a course"},
		{ID: "im/im", Type: "app", Shelf: "selfhosted/cloud", Why: "a photo library"},
		{ID: "gone/repo", Type: "cli", Shelf: "excluded", Why: "not stored"},
	}
	job := NewOverrides(st, list, 3, nil)
	job.now = func() time.Time { return now }

	rep, err := job.Run(ctx)
	if err != nil || rep.Processed != 2 || rep.Notes["missing"] != int64(1) || rep.Notes["clientDropped"] != int64(1) {
		t.Fatalf("run = %+v, %v; want two rewritten, one missing, one app out of client/android", rep, err)
	}
	get := func(id string) item.Labels {
		var it item.Item
		if err := st.Items().FindOne(ctx, bson.M{"externalId": id}).Decode(&it); err != nil {
			t.Fatal(err)
		}
		return it.Labels
	}
	l := get("fcc/fcc")
	if l.Type != "tutorial" || len(l.Domain) != 0 || l.Shelf != "excluded" || l.ShelfOverride != "excluded" ||
		l.By != item.ByManual || l.Version != 3 || l.TaglineZh != "免费学编程" {
		t.Errorf("fcc = %+v; want a tutorial off every board, the tagline kept", l)
	}
	// Not judged yet: corrected, but left in the judge's queue so its tagline
	// and the rest still get written.
	if l := get("im/im"); !slices.Equal(l.Domain, []string{"apps/selfhosted/cloud"}) || l.Shelf != "selfhosted/cloud" || l.By != item.ByRule {
		t.Errorf("immich = %+v", l)
	}
	if l := get("o/other"); !slices.Equal(l.Domain, []string{"data/database"}) || l.By == item.ByManual {
		t.Errorf("an item not in the table changed: %+v", l)
	}

	if l := get("p/app"); !slices.Equal(l.Domain, []string{"apps/system/phone"}) {
		t.Errorf("phone app = %v; an app is not an Android library", l.Domain)
	}
	if l := get("t/termux"); !slices.Equal(l.Domain, []string{"devtools/terminal", "client/android", "apps/system/phone"}) {
		t.Errorf("termux = %v; a developer app keeps client/android", l.Domain)
	}

	// The next night finds nothing to do.
	if rep, err := job.Run(ctx); err != nil || rep.Processed != 0 || rep.Notes["clientDropped"] != int64(0) {
		t.Fatalf("second run = %+v, %v; want nothing rewritten", rep, err)
	}
	// The judge queues the item it has not answered for, not the frozen one.
	queued := func() []string {
		q, err := st.JudgeQueue(ctx, store.JudgeScope{TreeVersion: 3, PromptVersion: judge.PromptVersion}, 10)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, c := range q {
			ids = append(ids, c.ExternalID)
		}
		return ids
	}
	if q := queued(); slices.Contains(q, "fcc/fcc") || !slices.Contains(q, "im/im") {
		t.Errorf("judge queue = %v; want im/im waiting and fcc/fcc left alone", q)
	}

	// The judge answers, wrongly again; that night the correction comes back,
	// and now the item is frozen.
	var photosID struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := st.Items().FindOne(ctx, bson.M{"externalId": "im/im"}).Decode(&photosID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetVerdict(ctx, photosID.ID, item.Labels{
		Domain: []string{"apps/selfhosted/cloud", "data/database"}, Type: "app", Shelf: "selfhosted/cloud",
		TaglineZh: "自托管相册", Version: 3,
	}, judge.PromptVersion, now); err != nil {
		t.Fatal(err)
	}
	if rep, err := job.Run(ctx); err != nil || rep.Processed != 1 {
		t.Fatalf("after the judge = %+v, %v; want immich corrected again", rep, err)
	}
	if l := get("im/im"); !slices.Equal(l.Domain, []string{"apps/selfhosted/cloud"}) || l.By != item.ByManual || l.TaglineZh != "自托管相册" {
		t.Errorf("immich after the judge = %+v; want corrected, frozen, tagline kept", l)
	}
	if q := queued(); slices.Contains(q, "im/im") {
		t.Errorf("judge queue = %v; immich is frozen now", q)
	}
}
