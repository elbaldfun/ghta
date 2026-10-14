package boards

import (
	"slices"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/elbaldfun/ghta/internal/item"
)

// Model boards rank HuggingFace models, overall and per task group: "hot" by
// the day's like gain, "likes", "downloads" (30 days) and "new" by creation.
const (
	ModelHot       = "hot"
	ModelLikes     = "likes"
	ModelDownloads = "downloads"
	ModelNew       = "new"

	ModelTopN = 300
)

var ModelSorts = []string{ModelHot, ModelLikes, ModelDownloads, ModelNew}

// ModelsKey names a model board; task "" spans every task.
func ModelsKey(task, sort string) string {
	if task == "" {
		task = "all"
	}
	return "models|" + task + "|" + sort
}

// ModelRow is the slice of a model the boards read.
type ModelRow struct {
	ExternalID string             `bson:"externalId"`
	Name       string             `bson:"name"`
	License    string             `bson:"license"`
	Metrics    map[string]float64 `bson:"metrics"`
	Growth     item.Growth        `bson:"growth"`
	Hf         *item.HuggingFace  `bson:"hf"`
	Stale      bool               `bson:"stale"`
}

// ModelProjection is what the boards job reads of each model.
func ModelProjection() bson.M {
	return bson.M{
		"externalId": 1, "name": 1, "license": 1, "metrics": 1, "growth.d1": 1, "stale": 1,
		"hf.author": 1, "hf.task": 1, "hf.pipelineTag": 1, "hf.library": 1, "hf.gated": 1,
		"hf.quantFormats": 1, "hf.params": 1, "hf.trendingScore": 1, "hf.createdAt": 1, "hf.lastModified": 1,
	}
}

type Model struct {
	ExternalID   string    `bson:"externalId" json:"externalId"`
	Author       string    `bson:"author,omitempty" json:"author,omitempty"`
	Name         string    `bson:"name" json:"name"`
	Task         string    `bson:"task,omitempty" json:"task,omitempty"`
	PipelineTag  string    `bson:"pipelineTag,omitempty" json:"pipelineTag,omitempty"`
	Library      string    `bson:"library,omitempty" json:"library,omitempty"`
	License      string    `bson:"license,omitempty" json:"license,omitempty"`
	Likes        int       `bson:"likes" json:"likes"`
	Downloads30d int64     `bson:"downloads30d" json:"downloads30d"`
	DownloadsAll int64     `bson:"downloadsAll,omitempty" json:"downloadsAll,omitempty"`
	Params       int64     `bson:"params,omitempty" json:"params,omitempty"`
	Growth       int       `bson:"growth" json:"growth"`
	Gated        bool      `bson:"gated" json:"gated"`
	QuantFormats []string  `bson:"quantFormats,omitempty" json:"quantFormats,omitempty"`
	CreatedAt    time.Time `bson:"createdAt,omitempty" json:"createdAt,omitzero"`
	LastModified time.Time `bson:"lastModified,omitempty" json:"lastModified,omitzero"`
	URL          string    `bson:"url" json:"url"`

	trending float64
}

// Models collects the live models.
type Models struct {
	rows []Model
}

func (m *Models) Add(r ModelRow) {
	if r.Stale || r.Hf == nil {
		return
	}
	row := Model{
		ExternalID: r.ExternalID, Author: r.Hf.Author, Name: r.Name, Task: r.Hf.Task, PipelineTag: r.Hf.PipelineTag,
		Library: r.Hf.Library, License: r.License, Likes: int(r.Metrics["likes"]),
		Downloads30d: int64(r.Metrics["downloads30d"]), DownloadsAll: int64(r.Metrics["downloadsAll"]),
		Params: r.Hf.Params, Gated: r.Hf.Gated, QuantFormats: r.Hf.QuantFormats,
		CreatedAt: r.Hf.CreatedAt.UTC(), LastModified: r.Hf.LastModified.UTC(),
		URL: "https://huggingface.co/" + r.ExternalID, trending: r.Hf.TrendingScore,
	}
	if r.Growth.D1 != nil {
		row.Growth = int(*r.Growth.D1)
	}
	m.rows = append(m.rows, row)
}

func (m *Models) Len() int { return len(m.rows) }

// Board returns one model board. Hot keeps only models that gained likes
// today; on a day none did (the first fetch of a model has no growth yet)
// it falls back to HuggingFace's own trending score.
func (m *Models) Board(task, sortBy string) []Model {
	var rows []Model
	for _, r := range m.rows {
		if task == "" || r.Task == task {
			rows = append(rows, r)
		}
	}
	var key func(Model) float64
	switch sortBy {
	case ModelLikes:
		key = func(r Model) float64 { return float64(r.Likes) }
	case ModelDownloads:
		key = func(r Model) float64 { return float64(r.Downloads30d) }
	case ModelNew:
		key = func(r Model) float64 { return float64(r.CreatedAt.Unix()) }
	default:
		key = func(r Model) float64 { return float64(r.Growth) }
		gaining := slices.DeleteFunc(slices.Clone(rows), func(r Model) bool { return r.Growth <= 0 })
		if len(gaining) > 0 {
			rows = gaining
		} else {
			key = func(r Model) float64 { return r.trending }
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if a, b := key(rows[i]), key(rows[j]); a != b {
			return a > b
		}
		return rows[i].ExternalID < rows[j].ExternalID
	})
	if len(rows) > ModelTopN {
		rows = rows[:ModelTopN]
	}
	if rows == nil {
		rows = []Model{}
	}
	return rows
}
