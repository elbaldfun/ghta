package taxonomy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/config"
	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// The shipped tree must be v3-shaped: versioned, apps three deep, nothing
// else deeper than two, and every migration target a real leaf.
func TestRealTreeAndMigrations(t *testing.T) {
	tree, err := LoadTree(filepath.Join(repoRoot(), "taxonomy", "taxonomy.yaml"))
	if err != nil {
		t.Fatalf("load tree: %v", err)
	}
	if tree.Version < 3 {
		t.Fatalf("tree version %d, want >= 3", tree.Version)
	}
	shelves := 0
	for _, l := range tree.Leaves() {
		if IsShelfPath(l.Path) {
			shelves++
		}
	}
	if shelves < 30 {
		t.Errorf("only %d shelves under apps/, want the v3 set", shelves)
	}
	ms, err := LoadMigrations(filepath.Join(repoRoot(), "taxonomy", "migrations.yaml"), tree)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var step *Migration
	for i := range ms {
		if ms[i].To == tree.Version {
			step = &ms[i]
		}
	}
	if step == nil {
		t.Fatalf("no migration step lands on v%d", tree.Version)
	}
	al := Aliases(ms)
	if al["web/css-ui"] != "web/ui-components" || al["infra/iac"] != "infra/automation" {
		t.Errorf("aliases missing mechanical moves: %v", al)
	}
	if al["ai/llm"] == "" {
		t.Errorf("split leaf ai/llm has no alias")
	}
}

func TestLoadTreeRejectsBadShapes(t *testing.T) {
	cases := map[string]string{
		"third level outside apps": "version: 3\nnodes:\n- path: web\n  name: W\n  children:\n  - path: web/a\n    name: a\n    children:\n    - path: web/a/b\n      name: b\n",
		"depth/path mismatch":      "version: 3\nnodes:\n- path: web/a\n  name: W\n",
	}
	for name, body := range cases {
		p := filepath.Join(t.TempDir(), "t.yaml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadTree(p); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	// Legacy bare-list shape still loads as v2.
	p := filepath.Join(t.TempDir(), "v2.yaml")
	if err := os.WriteFile(p, []byte("- path: web\n  name: W\n  children:\n  - { path: web/a, name: a }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tr, err := LoadTree(p)
	if err != nil || tr.Version != 2 || len(tr.Nodes) != 2 {
		t.Errorf("legacy load: %v %+v", err, tr)
	}
}

func TestShelfHelpers(t *testing.T) {
	if !IsShelfPath("apps/productivity/notes") || IsShelfPath("apps/productivity") || IsShelfPath("devtools/terminal") {
		t.Error("IsShelfPath")
	}
	if ShelfSlug("apps/productivity/notes") != "productivity/notes" || ShelfPath("productivity/notes") != "apps/productivity/notes" || ShelfPath("apps/x/y") != "apps/x/y" {
		t.Error("ShelfSlug/ShelfPath")
	}
}

// ---- mongo-backed: the migration ops end to end ----

var v3Nodes = Tree{Version: 3, Nodes: []Node{
	{Path: "web", Name: "web"}, {Path: "web/ui-components", Name: "ui"}, {Path: "web/frontend", Name: "fe"}, {Path: "web/data-viz", Name: "viz"},
	{Path: "infra", Name: "infra"}, {Path: "infra/automation", Name: "auto"},
	{Path: "apps", Name: "apps"}, {Path: "apps/productivity", Name: "prod"}, {Path: "apps/productivity/notes", Name: "notes"},
	{Path: "devtools", Name: "dt"}, {Path: "devtools/editors", Name: "ed"},
}}

func migrateTestStore(t *testing.T) *repository.Store {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set; skipping mongo integration test")
	}
	ctx := context.Background()
	store, err := repository.Connect(ctx, &config.Config{MongoURI: uri, MongoDB: "ghta_taxonomy_test"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("schema: %v", err)
	}
	_, _ = store.Items().DeleteMany(ctx, bson.M{})
	_, _ = store.Categories().DeleteMany(ctx, bson.M{})
	// Old leaves exist alongside the new tree, as they would after a restart
	// on the new yaml while items still reference them.
	old := append([]Node{}, v3Nodes.Nodes...)
	old = append(old, Node{Path: "web/css-ui", Name: "old css"}, Node{Path: "infra/ci-cd", Name: "old ci"}, Node{Path: "infra/iac", Name: "old iac"}, Node{Path: "apps/productivity-old", Name: "old apps"})
	if err := Sync(ctx, store, old); err != nil {
		t.Fatalf("sync: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func TestMigrateOps(t *testing.T) {
	store := migrateTestStore(t)
	ctx := context.Background()
	ins := func(id string, paths []string, extra bson.M) {
		doc := bson.M{"source": domain.SourceGitHub, "externalId": id, "name": id, "categoryPath": paths, "categoryId": []string{},
			"analysisStatus": domain.AnalysisDone, "metrics": bson.M{"stars": 5000}}
		for k, v := range extra {
			doc[k] = v
		}
		if _, err := store.Items().InsertOne(ctx, doc); err != nil {
			t.Fatal(err)
		}
	}
	ins("o/rename", []string{"web/css-ui", "web/frontend"}, nil)
	ins("o/merge", []string{"infra/ci-cd", "infra/iac"}, nil)
	ins("o/split", []string{"web/frontend"}, nil)
	ins("o/shelf", []string{"apps/productivity-old"}, bson.M{"store": bson.M{"category": "productivity/notes"}})
	ins("o/shelf-none", []string{"apps/productivity-old"}, nil)
	ins("o/shelf-excl", []string{"apps/productivity-old", "web/frontend"}, bson.M{"store": bson.M{"category": "excluded"}})
	ins("o/moved", []string{"web/frontend"}, bson.M{"store": bson.M{"category": "devtools/editor"}})
	ins("o/rejudge", []string{"web/frontend"}, bson.M{"store": bson.M{"category": "devtools/devops", "status": "done", "version": 1}})

	m := Migration{From: 2, To: 3,
		Domain: []Op{
			{Kind: OpRename, Old: "web/css-ui", New: "web/ui-components"},
			{Kind: OpMerge, Old: "infra/ci-cd", New: "infra/automation"},
			{Kind: OpMerge, Old: "infra/iac", New: "infra/automation"},
			{Kind: OpSplit, Old: "web/frontend", Candidates: []string{"web/frontend", "web/data-viz"}},
			{Kind: OpShelf, Old: "apps/productivity-old"},
		},
		Shelves: []Op{
			{Kind: OpMoved, Old: "devtools/editor", New: "devtools/editors"},
			{Kind: OpRejudge, Old: "devtools/devops", Candidates: []string{"infra/automation", "web/data-viz"}},
		},
	}
	mg := NewMigrator(store, v3Nodes, discardLogger())
	st, err := mg.Run(ctx, m, 2)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// rewritten: o/rename once, o/merge twice (one per merged source).
	if st.DomainRewritten != 3 || st.DomainRequeued < 1 || st.AppsRelabelled != 2 || st.ShelfRewritten != 1 || st.ShelfRequeued != 1 {
		t.Errorf("stats %+v", st)
	}

	get := func(id string) domain.TrackedItem {
		var it domain.TrackedItem
		if err := store.Items().FindOne(ctx, bson.M{"externalId": id}).Decode(&it); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return it
	}
	if it := get("o/rename"); !eq(it.CategoryPath, "web/ui-components", "web/frontend") || it.TaxonomyVersion != 3 || len(it.CategoryID) != 2 || !eq(it.CategoryPathPrev, "web/css-ui", "web/frontend") {
		t.Errorf("rename: %v v%d ids=%v prev=%v", it.CategoryPath, it.TaxonomyVersion, it.CategoryID, it.CategoryPathPrev)
	}
	if it := get("o/merge"); !eq(it.CategoryPath, "infra/automation") {
		t.Errorf("merge should dedupe: %v", it.CategoryPath)
	}
	if it := get("o/split"); it.AnalysisStatus != domain.AnalysisPending || !eq(it.CategoryPath, "web/frontend") || it.TaxonomyVersion == 3 {
		t.Errorf("split: status=%s paths=%v v%d", it.AnalysisStatus, it.CategoryPath, it.TaxonomyVersion)
	}
	if it := get("o/shelf"); !eq(it.CategoryPath, "apps/productivity/notes") || it.TaxonomyVersion != 3 {
		t.Errorf("shelf relabel: %v v%d", it.CategoryPath, it.TaxonomyVersion)
	}
	if it := get("o/shelf-none"); len(it.CategoryPath) != 0 || it.AnalysisStatus != domain.AnalysisPending {
		t.Errorf("shelf-none should be requeued: %v %s", it.CategoryPath, it.AnalysisStatus)
	}
	if it := get("o/shelf-excl"); !eq(it.CategoryPath, "web/frontend") {
		t.Errorf("excluded loses only the apps label: %v", it.CategoryPath)
	}
	if it := get("o/moved"); it.Store.Category != "devtools/editors" || it.Store.Version != 2 || !has(it.CategoryPath, "devtools/editors") {
		t.Errorf("moved: %+v paths=%v", it.Store, it.CategoryPath)
	}
	if it := get("o/rejudge"); it.Store.Version != 0 || it.Store.Status != "" {
		t.Errorf("rejudge should drop the verdict version: %+v", it.Store)
	}

	// Idempotent: a second run touches nothing.
	st2, err := mg.Run(ctx, m, 2)
	if err != nil {
		t.Fatal(err)
	}
	if st2.DomainRewritten != 0 || st2.AppsRelabelled != 0 || st2.ShelfRewritten != 0 || st2.ShelfRequeued != 0 || st2.Stamped != 0 {
		t.Errorf("second run not idempotent: %+v", st2)
	}

	// Sync now prunes only the old leaves nothing references: web/css-ui,
	// infra/ci-cd, infra/iac and apps/productivity-old are drained.
	if err := Sync(ctx, store, v3Nodes.Nodes); err != nil {
		t.Fatal(err)
	}
	n, _ := store.Categories().CountDocuments(ctx, bson.M{"path": bson.M{"$in": bson.A{"web/css-ui", "infra/ci-cd", "infra/iac", "apps/productivity-old"}}})
	if n != 0 {
		t.Errorf("drained old leaves should be pruned, %d remain", n)
	}

	// Rollback restores every snapshot.
	ids, _ := LeafIDs(ctx, store)
	if _, err := mg.Rollback(ctx, 2, ids); err != nil {
		t.Fatal(err)
	}
	if it := get("o/rename"); !eq(it.CategoryPath, "web/css-ui", "web/frontend") || it.TaxonomyVersion != 2 || len(it.CategoryPathPrev) != 0 {
		t.Errorf("rollback: %v v%d prev=%v", it.CategoryPath, it.TaxonomyVersion, it.CategoryPathPrev)
	}
}

func eq(got domain.PathList, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		if !has(got, w) {
			return false
		}
	}
	return true
}

func has(xs domain.PathList, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
