package importer

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

// Collections of the old store the importer reads. It never writes to them.
const (
	oldItems     = "tracked_items"
	oldSnapshots = "metric_snapshots"
)

// Options tune one import run.
type Options struct {
	Batch int       // items per write batch; default 500
	Now   time.Time // history window anchor; default time.Now()
	Log   *slog.Logger
}

// WindowDiff compares one growth window between the old stored value and the
// one recomputed from the history ring.
type WindowDiff struct {
	Same        int `json:"same"`
	Differ      int `json:"differ"`
	OnlyOld     int `json:"onlyOld"` // old had a value, the ring has no baseline
	OnlyNew     int `json:"onlyNew"`
	BothMissing int `json:"bothMissing"`
}

// Report is what one import did. Everything dropped is counted here.
type Report struct {
	Started      time.Time             `json:"started"`
	Finished     time.Time             `json:"finished"`
	Items        map[string]int        `json:"items"` // by source
	Blobs        int                   `json:"blobs"`
	WithHistory  int                   `json:"withHistory"`
	DecodeErrors int                   `json:"decodeErrors"`
	DroppedPaths map[string]int        `json:"droppedPaths"`
	UnknownKeys  map[string]int        `json:"unknownKeys"`
	Growth       map[string]WindowDiff `json:"growth"` // d1 | d7 | d30
}

// Run rebuilds items and item_blobs from tracked_items. It drops the two new
// collections first, so it can be repeated at will; the old collections are
// only read.
func Run(ctx context.Context, db *mongo.Database, r Rules, o Options) (Report, error) {
	if o.Batch <= 0 {
		o.Batch = 500
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	rep := Report{
		Started: time.Now().UTC(), Items: map[string]int{},
		DroppedPaths: map[string]int{}, UnknownKeys: map[string]int{},
		Growth: map[string]WindowDiff{},
	}
	st := store.New(db)
	if err := st.Reset(ctx); err != nil {
		return rep, err
	}
	if err := st.EnsureIndexes(ctx); err != nil {
		return rep, err
	}

	// README bodies make documents large; a small cursor batch keeps each
	// round trip well under the 16 MB reply limit.
	cur, err := db.Collection(oldItems).Find(ctx, bson.M{}, options.Find().SetBatchSize(100))
	if err != nil {
		return rep, fmt.Errorf("read %s: %w", oldItems, err)
	}
	defer cur.Close(ctx)

	since := o.Now.UTC().AddDate(0, 0, -(item.HistoryDays + 1))
	var items []item.Item
	var blobs []any
	flush := func() error {
		if len(items) == 0 {
			return nil
		}
		if err := fillHistory(ctx, db, items, since, &rep); err != nil {
			return err
		}
		docs := make([]any, len(items))
		for i := range items {
			docs[i] = items[i]
			rep.Items[items[i].Source]++
		}
		unordered := options.InsertMany().SetOrdered(false)
		if _, err := st.Items().InsertMany(ctx, docs, unordered); err != nil {
			return fmt.Errorf("insert items: %w", err)
		}
		if len(blobs) > 0 {
			if _, err := st.Blobs().InsertMany(ctx, blobs, unordered); err != nil {
				return fmt.Errorf("insert blobs: %w", err)
			}
			rep.Blobs += len(blobs)
		}
		items, blobs = items[:0], blobs[:0]
		return nil
	}

	n := 0
	for cur.Next(ctx) {
		var old domain.TrackedItem
		if err := cur.Decode(&old); err != nil {
			rep.DecodeErrors++
			o.Log.Warn("import: decode failed", "err", err)
			continue
		}
		it, blob, notes := Convert(old, r)
		for _, p := range notes.DroppedPaths {
			rep.DroppedPaths[p]++
		}
		for _, k := range notes.UnknownKeys {
			rep.UnknownKeys[k]++
		}
		items = append(items, it)
		if blob != nil {
			blobs = append(blobs, *blob)
		}
		if len(items) >= o.Batch {
			if err := flush(); err != nil {
				return rep, err
			}
		}
		if n++; n%10000 == 0 {
			o.Log.Info("import: progress", "items", n)
		}
	}
	if err := cur.Err(); err != nil {
		return rep, fmt.Errorf("read %s: %w", oldItems, err)
	}
	if err := flush(); err != nil {
		return rep, err
	}
	rep.Finished = time.Now().UTC()
	return rep, nil
}

// fillHistory builds each item's history ring from metric_snapshots — one
// point per UTC day, the day's last capture — and recomputes growth from it,
// recording how the result compares with the growth the old store held.
func fillHistory(ctx context.Context, db *mongo.Database, items []item.Item, since time.Time, rep *Report) error {
	type key struct{ source, id string }
	index := make(map[key]int, len(items))
	bySource := map[string][]string{}
	for i, it := range items {
		index[key{it.Source, it.ExternalID}] = i
		bySource[it.Source] = append(bySource[it.Source], it.ExternalID)
	}
	type capture struct {
		at time.Time
		v  float64
	}
	captures := make([][]capture, len(items))

	for source, ids := range bySource {
		cur, err := db.Collection(oldSnapshots).Find(ctx, bson.M{
			"meta.source":     source,
			"meta.externalId": bson.M{"$in": ids},
			"capturedAt":      bson.M{"$gte": since},
		})
		if err != nil {
			return fmt.Errorf("read %s: %w", oldSnapshots, err)
		}
		for cur.Next(ctx) {
			var s domain.MetricSnapshot
			if err := cur.Decode(&s); err != nil {
				continue
			}
			i, ok := index[key{string(s.Meta.Source), s.Meta.ExternalID}]
			if !ok {
				continue
			}
			if v, ok := s.Metrics[items[i].PrimaryMetric]; ok {
				captures[i] = append(captures[i], capture{s.CapturedAt, v})
			}
		}
		err = cur.Err()
		cur.Close(ctx)
		if err != nil {
			return fmt.Errorf("read %s: %w", oldSnapshots, err)
		}
	}

	for i := range items {
		it := &items[i]
		cs := captures[i]
		sort.Slice(cs, func(a, b int) bool { return cs[a].at.Before(cs[b].at) })
		for _, c := range cs {
			it.History = item.SetPoint(it.History, item.Point{D: item.DayOf(c.at), V: c.v})
		}
		old := it.Growth
		current, hasMetric := it.Metrics[it.PrimaryMetric]
		if len(it.History) > 0 && hasMetric {
			rep.WithHistory++
			it.Growth = item.ComputeGrowth(it.History, current, item.DayOf(it.FetchedAt))
		} else {
			it.Growth = item.Growth{}
		}
		compare(rep, "d1", old.D1, it.Growth.D1)
		compare(rep, "d7", old.D7, it.Growth.D7)
		compare(rep, "d30", old.D30, it.Growth.D30)
	}
	return nil
}

func compare(rep *Report, window string, old, new *float64) {
	d := rep.Growth[window]
	switch {
	case old == nil && new == nil:
		d.BothMissing++
	case old == nil:
		d.OnlyNew++
	case new == nil:
		d.OnlyOld++
	case math.Abs(*old-*new) < 0.5:
		d.Same++
	default:
		d.Differ++
	}
	rep.Growth[window] = d
}
