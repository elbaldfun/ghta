// Package pipeline holds the jobs the worker runs. Each job touches only the
// rebuilt collections (docs/REBUILD.md): never tracked_items.
package pipeline

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
	"github.com/elbaldfun/ghta/internal/worker"
)

const (
	rankWriteBatch = 1000
	// asc-better is the one direction where a smaller value ranks first.
	directionAscBetter = "asc-better"
)

// Rank writes every item's positions: within its source, within its language
// and within each of its domain leaves, all by the primary metric.
//
// The whole corpus is ranked in memory from a narrow projection (a few MB for
// 130k items) and only items whose positions moved are written.
type Rank struct {
	st  *store.Store
	now func() time.Time
}

func NewRank(st *store.Store) *Rank { return &Rank{st: st, now: time.Now} }

func (*Rank) Name() string { return "rank" }

// rankRow is the projection the job reads.
type rankRow struct {
	ID              primitive.ObjectID `bson:"_id"`
	Source          string             `bson:"source"`
	ExternalID      string             `bson:"externalId"`
	Language        string             `bson:"language"`
	PrimaryMetric   string             `bson:"primaryMetric"`
	MetricDirection string             `bson:"metricDirection"`
	Metrics         map[string]float64 `bson:"metrics"`
	Labels          struct {
		Domain []string `bson:"domain"`
	} `bson:"labels"`
	Ranks *item.Ranks `bson:"ranks"`
	Stale bool        `bson:"stale"`

	value float64
}

func (j *Rank) Run(ctx context.Context) (worker.Report, error) {
	var rep worker.Report
	cur, err := j.st.Items().Find(ctx, bson.M{}, options.Find().SetBatchSize(2000).SetProjection(bson.M{
		"source": 1, "externalId": 1, "language": 1, "primaryMetric": 1, "metricDirection": 1,
		"metrics": 1, "labels.domain": 1, "ranks": 1, "stale": 1,
	}))
	if err != nil {
		return rep, fmt.Errorf("read items: %w", err)
	}
	defer cur.Close(ctx)

	// Writes go out in batches as they are produced: holding 130k update
	// models at once costs several hundred MB, more than the worker is given.
	var batch []mongo.WriteModel
	unordered := options.BulkWrite().SetOrdered(false)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := j.st.Items().BulkWrite(ctx, batch, unordered); err != nil {
			return fmt.Errorf("write ranks: %w", err)
		}
		batch = batch[:0]
		return nil
	}
	queue := func(id primitive.ObjectID, update bson.M) error {
		batch = append(batch, mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": id}).SetUpdate(update))
		if len(batch) >= rankWriteBatch {
			return flush()
		}
		return nil
	}

	var ranked []rankRow
	written, cleared := 0, 0
	for cur.Next(ctx) {
		var row rankRow
		if err := cur.Decode(&row); err != nil {
			rep.Failed++
			continue
		}
		v, ok := row.Metrics[row.PrimaryMetric]
		if row.Stale || !ok {
			// Out of every ranking: drop positions it may still carry.
			if row.Ranks != nil {
				if err := queue(row.ID, bson.M{"$unset": bson.M{"ranks": ""}}); err != nil {
					return rep, err
				}
				cleared++
			}
			continue
		}
		row.value = v
		if row.MetricDirection == directionAscBetter {
			row.value = -v
		}
		row.Metrics = nil
		ranked = append(ranked, row)
	}
	if err := cur.Err(); err != nil {
		return rep, fmt.Errorf("read items: %w", err)
	}

	// Best first; the external id breaks ties so reruns give the same order.
	sort.Slice(ranked, func(a, b int) bool {
		x, y := &ranked[a], &ranked[b]
		if x.value != y.value {
			return x.value > y.value
		}
		return x.ExternalID < y.ExternalID
	})
	// One pass over the sorted corpus: a counter per group hands out positions.
	type group struct{ source, kind, key string }
	seen := map[group]int{}
	place := func(g group) int { seen[g]++; return seen[g] }
	at := j.now().UTC().Truncate(time.Millisecond)
	for i := range ranked {
		row := &ranked[i]
		next := item.Ranks{Overall: place(group{row.Source, "all", ""}), At: at}
		if row.Language != "" {
			next.Language = place(group{row.Source, "lang", row.Language})
		}
		for _, p := range row.Labels.Domain {
			if p == "" {
				continue
			}
			if next.Domain == nil {
				next.Domain = map[string]int{}
			}
			if _, dup := next.Domain[p]; !dup {
				next.Domain[p] = place(group{row.Source, "domain", p})
			}
		}
		if !sameRanks(row.Ranks, next) {
			if err := queue(row.ID, bson.M{"$set": bson.M{"ranks": next}}); err != nil {
				return rep, err
			}
			written++
		}
	}
	if err := flush(); err != nil {
		return rep, err
	}
	rep.Processed = len(ranked)
	rep.Note("written", written)
	rep.Note("cleared", cleared)
	return rep, nil
}

// sameRanks reports whether the stored positions already equal the new ones;
// the timestamp is when positions last changed, so it does not count.
func sameRanks(old *item.Ranks, next item.Ranks) bool {
	return old != nil && old.Overall == next.Overall && old.Language == next.Language &&
		len(old.Domain) == len(next.Domain) && maps.Equal(old.Domain, next.Domain)
}
