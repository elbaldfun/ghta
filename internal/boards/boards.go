// Package boards computes the materialised rankings (docs/REDESIGN.md 5.2):
// every list the old api aggregated on request. The functions are pure over
// a Corpus read once per night; the boards job writes their results and the
// api reads them back by key.
//
// The rules follow the old services (internal/service) so the two systems
// can be compared head to head, with two deliberate changes: the domain is
// the taxonomy v3 label set, and tombstoned items count nowhere.
package boards

import (
	"math"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
)

// Row is the slice of an item the boards read.
type Row struct {
	Source      string             `bson:"source"`
	ExternalID  string             `bson:"externalId"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	Language    string             `bson:"language"`
	Topics      []string           `bson:"topics"`
	Metrics     map[string]float64 `bson:"metrics"`
	Growth      item.Growth        `bson:"growth"`
	Labels      struct {
		Domain []string `bson:"domain"`
		Type   string   `bson:"type"`
	} `bson:"labels"`
	Gh *struct {
		Owner    string    `bson:"owner"`
		PushedAt time.Time `bson:"pushedAt"`
	} `bson:"gh"`
	Stale bool `bson:"stale"`
}

// Projection is what the boards job reads of each item.
func Projection() bson.M {
	return bson.M{
		"source": 1, "externalId": 1, "name": 1, "description": 1, "language": 1, "topics": 1,
		"metrics.stars": 1, "metrics.forks": 1, "metrics.openIssues": 1, "growth.d1": 1,
		"labels.domain": 1, "labels.type": 1, "gh.owner": 1, "gh.pushedAt": 1, "stale": 1,
	}
}

func (r *Row) stars() float64 { return r.Metrics["stars"] }

// d1 is the day's gain; an item too young for the window counts as zero, as
// the old boards' $ifNull did.
func (r *Row) d1() float64 {
	if r.Growth.D1 == nil {
		return 0
	}
	return *r.Growth.D1
}

// Corpus is the live GitHub items. HuggingFace models have their own list and
// no owner, topics or domain tree, so no board here counts them.
type Corpus struct {
	rows []Row
}

// Add keeps a row if it belongs on the boards.
func (c *Corpus) Add(r Row) {
	if r.Stale || r.Source != item.SourceGitHub {
		return
	}
	c.rows = append(c.rows, r)
}

func (c *Corpus) Len() int { return len(c.rows) }

// Group is a domain filter: "" spans every domain, otherwise a set of
// top-level domains.
type Group struct {
	Key  string
	Tops []string
}

// Groups are the domain filters the developer and topic boards are computed
// for: everything, and each top-level domain. "ai" also takes agents, which
// taxonomy v3 split out of ai: the AI pages are about both.
func Groups(tops []string) []Group {
	out := []Group{{Key: "all"}}
	for _, t := range tops {
		g := Group{Key: t, Tops: []string{t}}
		if t == "ai" {
			g.Tops = append(g.Tops, "agents")
		}
		out = append(out, g)
	}
	return out
}

func (g Group) has(r *Row) bool {
	if len(g.Tops) == 0 {
		return true
	}
	for _, p := range r.Labels.Domain {
		for _, t := range g.Tops {
			if strings.HasPrefix(p, t+"/") {
				return true
			}
		}
	}
	return false
}

// top tracks the most-starred item of a group; ties go to the smaller id so
// reruns agree.
type top struct {
	ID    string
	Stars float64
}

func (t *top) offer(r *Row) {
	if s := r.stars(); t.ID == "" || s > t.Stars || s == t.Stars && r.ExternalID < t.ID {
		t.ID, t.Stars = r.ExternalID, s
	}
}

// median is the lower median, the value $percentile's approximate method
// returns for p=0.5 on small inputs.
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	return v[int(math.Ceil(0.5*float64(len(v))))-1]
}
