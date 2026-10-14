package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/taxonomy"
	"github.com/elbaldfun/ghta/internal/worker"
)

// Migrate brings item labels up to the taxonomy tree in the image
// (taxonomy/migrations.yaml). It runs every night before the judge and has
// nothing to do unless the tree moved: what can be rewritten mechanically
// (a renamed or merged leaf, a renamed shelf) is rewritten, what needs
// judgement (a split leaf, a shelf to re-judge) goes back to the judge with
// its old labels still in place.
//
// Every step is keyed on the old value and on the item being behind the
// step's version, so a run cut short is finished by the next one.
type Migrate struct {
	st    *store.Store
	tree  taxonomy.Tree
	steps []taxonomy.Migration
	log   *slog.Logger
}

func NewMigrate(st *store.Store, tree taxonomy.Tree, steps []taxonomy.Migration, log *slog.Logger) *Migrate {
	if log == nil {
		log = slog.Default()
	}
	steps = append([]taxonomy.Migration(nil), steps...)
	sort.Slice(steps, func(a, b int) bool { return steps[a].To < steps[b].To })
	return &Migrate{st: st, tree: tree, steps: steps, log: log}
}

func (*Migrate) Name() string { return "migrate" }

func (j *Migrate) Run(ctx context.Context) (rep worker.Report, err error) {
	var rewritten, requeued, stamped int64
	defer func() {
		rep.Processed = int(rewritten + requeued)
		rep.Note("rewritten", rewritten)
		rep.Note("requeued", requeued)
		rep.Note("stamped", stamped)
	}()
	for _, m := range j.steps {
		if m.To > j.tree.Version {
			continue // a step for a tree this image does not carry yet
		}
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		rw, rq, err := j.step(ctx, m)
		rewritten, requeued = rewritten+rw, requeued+rq
		if err != nil {
			return rep, fmt.Errorf("migrate to v%d: %w", m.To, err)
		}
		// Only now: a step cut short above leaves every item behind, to be
		// picked up whole by the next run.
		n, err := j.st.StampVersion(ctx, m.To)
		if err != nil {
			return rep, err
		}
		stamped += n
		if rw+rq+n > 0 {
			j.log.Info("migrate: step applied", "to", m.To, "rewritten", rw, "requeued", rq, "stamped", n)
		}
	}
	return rep, nil
}

// step applies the operations of one migration.
func (j *Migrate) step(ctx context.Context, m taxonomy.Migration) (rewritten, requeued int64, err error) {
	count := func(into *int64) func(int64, error) error {
		return func(n int64, err error) error {
			*into += n
			return err
		}
	}
	rw, rq := count(&rewritten), count(&requeued)
	for _, op := range m.Domain {
		switch op.Kind {
		case taxonomy.OpRename, taxonomy.OpMerge:
			err = rw(j.st.RenameLeaf(ctx, op.Old, op.New, m.To))
		case taxonomy.OpSplit:
			err = rq(j.st.RequeueLeaf(ctx, op.Old, m.To))
		case taxonomy.OpShelf:
			var a, b int64
			a, b, err = j.st.RetireAppsLeaf(ctx, op.Old, op.Fallback, judge.ShelfLeaf, m.To)
			rewritten, requeued = rewritten+a, requeued+b
		}
		if err != nil {
			return rewritten, requeued, err
		}
	}
	for _, op := range m.Shelves {
		switch op.Kind {
		case taxonomy.OpRename, taxonomy.OpMerge:
			err = rw(j.st.RenameShelf(ctx, op.Old, op.New, taxonomy.ShelfPath(op.Old), taxonomy.ShelfPath(op.New), m.To))
		case taxonomy.OpMoved:
			// The shelf left the apps branch: its slug is now a domain leaf.
			err = rw(j.st.RenameShelf(ctx, op.Old, op.New, taxonomy.ShelfPath(op.Old), op.New, m.To))
		case taxonomy.OpRejudge:
			err = rq(j.st.RequeueShelf(ctx, op.Old, m.To))
		}
		if err != nil {
			return rewritten, requeued, err
		}
	}
	return rewritten, requeued, nil
}
