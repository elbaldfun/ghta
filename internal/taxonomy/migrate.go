package taxonomy

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/elbaldfun/ghta/internal/repository"
)

// Op kinds. Domain ops rewrite categoryPath; shelf ops rewrite store.category.
const (
	OpRename  = "rename"  // 1:1 — mechanical
	OpMerge   = "merge"   // n:1 — mechanical
	OpSplit   = "split"   // 1:n — items re-queued for classification
	OpShelf   = "shelf"   // old apps/* domain leaf -> apps/<effective shelf>
	OpMoved   = "moved"   // shelf left the apps branch; store.category becomes a domain leaf
	OpRejudge = "rejudge" // shelf needs the LLM to pick among candidates
)

// Op is one line of migrations.yaml.
type Op struct {
	Kind       string   `yaml:"kind"`
	Old        string   `yaml:"old"`
	New        string   `yaml:"new,omitempty"`
	Candidates []string `yaml:"candidates,omitempty"`
	Fallback   string   `yaml:"fallback,omitempty"` // shelf op: slug to use when the item has no shelf
}

// Migration moves items from tree version From to To.
type Migration struct {
	From    int  `yaml:"from"`
	To      int  `yaml:"to"`
	Domain  []Op `yaml:"domain"`
	Shelves []Op `yaml:"shelves"`
}

// LoadMigrations parses migrations.yaml and validates it against the tree:
// every rename/merge/moved target and every split/rejudge candidate must be a
// leaf of the current tree, so a typo fails at boot instead of silently
// scattering items.
func LoadMigrations(path string, tree Tree) ([]Migration, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var ms []Migration
	if err := yaml.Unmarshal(raw, &ms); err != nil {
		return nil, fmt.Errorf("parse migrations: %w", err)
	}
	leaves := map[string]bool{}
	for _, n := range tree.Leaves() {
		leaves[n.Path] = true
	}
	shelfOK := func(slug string) bool { return leaves[ShelfPath(slug)] }
	for _, m := range ms {
		if m.To != m.From+1 {
			return nil, fmt.Errorf("migration %d->%d: must step one version", m.From, m.To)
		}
		if m.To != tree.Version {
			continue // older step: targets may since have moved again; only the current step is validated
		}
		for _, op := range m.Domain {
			switch op.Kind {
			case OpRename, OpMerge:
				if !leaves[op.New] {
					return nil, fmt.Errorf("domain %s %s: target %q is not a leaf", op.Kind, op.Old, op.New)
				}
				if op.Old == op.New {
					return nil, fmt.Errorf("domain %s %s: old and new are identical", op.Kind, op.Old)
				}
			case OpSplit:
				if len(op.Candidates) < 2 {
					return nil, fmt.Errorf("domain split %s: needs >= 2 candidates", op.Old)
				}
				for _, c := range op.Candidates {
					if !leaves[c] {
						return nil, fmt.Errorf("domain split %s: candidate %q is not a leaf", op.Old, c)
					}
				}
			case OpShelf:
				if op.Fallback != "" && !shelfOK(op.Fallback) {
					return nil, fmt.Errorf("domain shelf %s: fallback %q is not a shelf", op.Old, op.Fallback)
				}
			default:
				return nil, fmt.Errorf("domain op %s: unknown kind %q", op.Old, op.Kind)
			}
		}
		for _, op := range m.Shelves {
			switch op.Kind {
			case OpRename, OpMerge:
				if !shelfOK(op.New) {
					return nil, fmt.Errorf("shelf %s %s: target %q is not a shelf", op.Kind, op.Old, op.New)
				}
			case OpMoved:
				if !leaves[op.New] || IsShelfPath(op.New) {
					return nil, fmt.Errorf("shelf moved %s: target %q must be a non-apps leaf", op.Old, op.New)
				}
			case OpRejudge:
				if len(op.Candidates) < 2 {
					return nil, fmt.Errorf("shelf rejudge %s: needs >= 2 candidates", op.Old)
				}
				for _, c := range op.Candidates {
					if !leaves[c] {
						return nil, fmt.Errorf("shelf rejudge %s: candidate %q is not a leaf", op.Old, c)
					}
				}
			default:
				return nil, fmt.Errorf("shelf op %s: unknown kind %q", op.Old, op.Kind)
			}
		}
	}
	return ms, nil
}

// Stats is what one Migrate run did.
type Stats struct {
	DomainRewritten int64 `json:"domainRewritten"`
	DomainRequeued  int64 `json:"domainRequeued"`
	ShelfRewritten  int64 `json:"shelfRewritten"`
	ShelfRequeued   int64 `json:"shelfRequeued"`
	AppsRelabelled  int64 `json:"appsRelabelled"`
	Stamped         int64 `json:"stamped"`
}

// Migrator applies one migration step. It is idempotent: every write is keyed
// on the OLD value, so a second run (or a run resumed after a crash) finds
// nothing left to touch. The categorizer and storefinder do the LLM part later
// by picking up what Migrate re-queued; Migrate itself never calls a model.
type Migrator struct {
	store *repository.Store
	tree  Tree
	log   *slog.Logger
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+){1,2}$`)
