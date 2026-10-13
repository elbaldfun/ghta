package job

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
	"github.com/elbaldfun/ghta/internal/taxonomy"
)

// Thresholds are RELATIVE so they scale with the corpus (adding sub-1k-star
// repos must not make every leaf "too big"). A leaf is flagged to split when
// it holds more than SplitShare of the GitHub corpus or more than SplitHotShare
// of the hot items (weeklyIncrease >= HotWeekly), and to merge when it holds
// less than MergeShare. The audit only flags; a human changes the tree.
const (
	auditSplitShare    = 0.04
	auditSplitHotShare = 0.06
	auditMergeShare    = 0.005
	auditHotWeekly     = 100.0
)

// LeafAudit is one leaf's size and heat with the verdict against the thresholds.
type LeafAudit struct {
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Items    int64   `json:"items"`
	Share    float64 `json:"share"`    // of GitHub items (multi-label: shares sum > 1)
	Hot      int64   `json:"hot"`      // items with weeklyIncrease >= 100 under this leaf
	HotShare float64 `json:"hotShare"` // of all hot items
	Flag     string  `json:"flag"`     // "" | split | merge
}

// AuditReport is the whole table plus the totals it was computed against.
type AuditReport struct {
	TreeVersion int         `json:"treeVersion"`
	At          time.Time   `json:"at"`
	GitHubItems int64       `json:"githubItems"`
	HotItems    int64       `json:"hotItems"`
	Leaves      []LeafAudit `json:"leaves"`
	Flagged     []LeafAudit `json:"flagged"`
}

// TaxonomyAudit computes the per-leaf report nightly and keeps the last one
// for GET /internal/taxonomy-audit. Cost: one index-only count per leaf plus
// ONE pass over the hot items (~2k docs via the weeklyIncrease index) —
// never a scan of the whole collection, which on this box means an OOM.
type TaxonomyAudit struct {
	store *repository.Store
	tree  taxonomy.Tree
	log   *slog.Logger

	mu   sync.RWMutex
	last *AuditReport
}

func NewTaxonomyAudit(store *repository.Store, tree taxonomy.Tree, log *slog.Logger) *TaxonomyAudit {
	return &TaxonomyAudit{store: store, tree: tree, log: log}
}

// Last returns the most recent report, or nil before the first run.
func (a *TaxonomyAudit) Last() *AuditReport {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.last
}

// Run computes the report, logs every flagged leaf, and stores the result.
func (a *TaxonomyAudit) Run(ctx context.Context) (*AuditReport, error) {
	rep, err := a.compute(ctx)
	if err != nil {
		a.log.Error("taxonomy audit failed", "err", err)
		return nil, err
	}
	for _, l := range rep.Flagged {
		a.log.Warn("taxonomy audit: leaf over threshold", "leaf", l.Path, "flag", l.Flag,
			"items", l.Items, "share", l.Share, "hot", l.Hot, "hotShare", l.HotShare)
	}
	a.log.Info("taxonomy audit done", "leaves", len(rep.Leaves), "flagged", len(rep.Flagged), "githubItems", rep.GitHubItems, "hotItems", rep.HotItems)
	a.mu.Lock()
	a.last = rep
	a.mu.Unlock()
	return rep, nil
}

func (a *TaxonomyAudit) compute(ctx context.Context) (*AuditReport, error) {
	live := bson.M{"source": domain.SourceGitHub, "stale": bson.M{"$ne": true}}
	total, err := a.store.Items().CountDocuments(ctx, live)
	if err != nil {
		return nil, err
	}

	// Hot items in one pass: match on the weeklyIncrease index, project only
	// the paths, unwind, group. ~2k docs at today's corpus.
	hotByLeaf := map[string]int64{}
	var hotTotal int64
	cur, err := a.store.Items().Aggregate(ctx, bson.A{
		bson.D{{Key: "$match", Value: bson.M{"source": domain.SourceGitHub, "stale": bson.M{"$ne": true}, "weeklyIncrease": bson.M{"$gte": auditHotWeekly}}}},
		bson.D{{Key: "$project", Value: bson.M{"categoryPath": 1}}},
		bson.D{{Key: "$facet", Value: bson.M{
			"total": bson.A{bson.D{{Key: "$count", Value: "n"}}},
			"byLeaf": bson.A{
				bson.D{{Key: "$unwind", Value: "$categoryPath"}},
				bson.D{{Key: "$group", Value: bson.M{"_id": "$categoryPath", "n": bson.M{"$sum": 1}}}},
			},
		}}},
	}, options.Aggregate().SetAllowDiskUse(false))
	if err != nil {
		return nil, err
	}
	var facet []struct {
		Total []struct {
			N int64 `bson:"n"`
		} `bson:"total"`
		ByLeaf []struct {
			ID string `bson:"_id"`
			N  int64  `bson:"n"`
		} `bson:"byLeaf"`
	}
	if err := cur.All(ctx, &facet); err != nil {
		return nil, err
	}
	if len(facet) == 1 {
		if len(facet[0].Total) == 1 {
			hotTotal = facet[0].Total[0].N
		}
		for _, b := range facet[0].ByLeaf {
			hotByLeaf[b.ID] = b.N
		}
	}

	rep := &AuditReport{TreeVersion: a.tree.Version, At: time.Now().UTC(), GitHubItems: total, HotItems: hotTotal}
	for _, leaf := range a.tree.Leaves() {
		n, err := a.store.Items().CountDocuments(ctx, bson.M{"source": domain.SourceGitHub, "stale": bson.M{"$ne": true}, "categoryPath": leaf.Path})
		if err != nil {
			return nil, err
		}
		la := LeafAudit{Path: leaf.Path, Name: leaf.Name, Items: n, Hot: hotByLeaf[leaf.Path]}
		if total > 0 {
			la.Share = float64(n) / float64(total)
		}
		if hotTotal > 0 {
			la.HotShare = float64(la.Hot) / float64(hotTotal)
		}
		switch {
		case la.Share > auditSplitShare || la.HotShare > auditSplitHotShare:
			la.Flag = "split"
		case la.Share < auditMergeShare && !taxonomy.IsShelfPath(leaf.Path):
			// Shelves are allowed to be small: a consumer shelf with 90 good
			// apps is a fine shelf. Only domain leaves get the merge flag.
			la.Flag = "merge"
		}
		rep.Leaves = append(rep.Leaves, la)
		if la.Flag != "" {
			rep.Flagged = append(rep.Flagged, la)
		}
	}
	sort.Slice(rep.Leaves, func(i, j int) bool { return rep.Leaves[i].Items > rep.Leaves[j].Items })
	sort.Slice(rep.Flagged, func(i, j int) bool { return rep.Flagged[i].HotShare > rep.Flagged[j].HotShare })
	return rep, nil
}
