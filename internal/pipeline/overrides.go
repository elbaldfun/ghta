package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/elbaldfun/ghta/internal/judge"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

// Overrides brings stored labels in line with the rules the judge layer
// keeps on top of the model. Every night after the judge, so a correction
// holds whatever the model said, and before rank and boards, which file items
// by their labels, it
//
//   - takes the client-development leaves off apps on a consumer shelf
//     (judge.ClientBranch), and
//   - writes the corrections a person made (taxonomy/overrides.yaml) over the
//     labels of the items they name.
//
// Both ship in the image: a correction takes effect with a release, never by
// editing the database.
type Overrides struct {
	st      *store.Store
	list    []judge.Override
	version int
	log     *slog.Logger
	now     func() time.Time
}

func NewOverrides(st *store.Store, list []judge.Override, treeVersion int, log *slog.Logger) *Overrides {
	if log == nil {
		log = slog.Default()
	}
	return &Overrides{st: st, list: list, version: treeVersion, log: log, now: time.Now}
}

func (*Overrides) Name() string { return "overrides" }

func (j *Overrides) Run(ctx context.Context) (rep worker.Report, err error) {
	var missing, dropped int64
	defer func() {
		rep.Note("missing", missing)
		rep.Note("clientDropped", dropped)
	}()
	if dropped, err = j.st.DropClientLeaves(ctx, judge.ClientBranch); err != nil {
		return rep, err
	}
	at := j.now()
	for _, o := range j.list {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		found, changed, err := j.st.ApplyOverride(ctx, store.LabelOverride{
			ExternalID: o.ID, Type: o.Type, Domain: o.Domain(), Shelf: o.Shelf,
		}, j.version, at)
		if err != nil {
			return rep, err
		}
		switch {
		case !found:
			// Renamed or not taken in yet: the table wants a look, the run goes on.
			missing++
			j.log.Warn("overrides: no such item", "id", o.ID)
		case changed:
			rep.Processed++
		}
	}
	return rep, nil
}
