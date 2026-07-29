package huggingface

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/elbaldfun/ghta/internal/domain"
)

// Corpus bounds (design §2): the union of three boards, refreshed daily. Items
// that fall off the boards stop updating but keep their history.
const (
	maxByDownloads = 20000
	maxByLikes     = 10000
	maxByTrending  = 5000
)

// Adapter implements source.Fetcher for HuggingFace models.
type Adapter struct {
	client *Client
	log    *slog.Logger
}

func NewAdapter(token string, log *slog.Logger) *Adapter {
	return &Adapter{client: NewClient(token), log: log}
}

func (a *Adapter) Source() domain.Source { return domain.SourceHuggingFace }

// Shards: one per board. Cursor pagination is sequential, so a board can't be
// split into resumable page-shards; each shard paginates internally (~a few
// minutes each) and retries whole on failure.
func (a *Adapter) Shards() []string { return []string{"downloads", "likes", "trending"} }

// Fetch pulls one board and normalizes it. Private/disabled models are dropped;
// gated ones kept and flagged (design §2).
func (a *Adapter) Fetch(ctx context.Context, shard string) ([]domain.TrackedItem, error) {
	var (
		sortKey string
		max     int
	)
	switch shard {
	case "downloads":
		sortKey, max = "downloads", maxByDownloads
	case "likes":
		sortKey, max = "likes", maxByLikes
	case "trending":
		sortKey, max = "trendingScore", maxByTrending
	default:
		return nil, fmt.Errorf("unknown hf shard %q", shard)
	}

	models, err := a.client.ListSorted(ctx, sortKey, max)
	if err != nil {
		return nil, err
	}
	items := make([]domain.TrackedItem, 0, len(models))
	for _, m := range models {
		if m.Private || m.ID == "" {
			continue
		}
		items = append(items, mapModel(m))
	}
	return items, nil
}

// mapModel normalizes one Hub model into the source-agnostic TrackedItem.
//
// Metric semantics (design §3): `likes` is cumulative (star-like), so it is the
// PRIMARY metric — the shared metrics job then derives daily/weekly increases
// from snapshots, giving the "hot" velocity axis for free. `downloads30d` is a
// rolling 30-day gauge (not cumulative); it ranks the "current adoption" board
// directly and must never be treated as a counter.
func mapModel(m Model) domain.TrackedItem {
	author := ""
	name := m.ID
	if i := strings.IndexByte(m.ID, '/'); i > 0 {
		author, name = m.ID[:i], m.ID[i+1:]
	}

	task := TaskGroup(m.PipelineTag)
	// Language tags are bare ISO codes mixed into tags (e.g. "en", "zh").
	var license string
	for _, t := range m.Tags {
		if strings.HasPrefix(t, "license:") {
			license = strings.TrimPrefix(t, "license:")
			break
		}
	}
	var params int64
	if m.Safetensors != nil {
		params = m.Safetensors.Total
	}
	refs := ParseTagRefs(m.Tags)
	gated := false
	switch v := m.Gated.(type) {
	case bool:
		gated = v
	case string:
		gated = v != "" && v != "false"
	}

	return domain.TrackedItem{
		Source:          domain.SourceHuggingFace,
		ExternalID:      m.ID,
		Name:            name,
		Description:     "", // the list API carries no description; the model card lives on HF
		PrimaryMetric:   "likes",
		MetricDirection: domain.DirectionDescBetter,
		Metrics: map[string]float64{
			"likes":        float64(m.Likes),
			"downloads30d": float64(m.Downloads),
			"downloadsAll": float64(m.DownloadsAll),
		},
		CategoryPath: domain.PathList{"hf/" + task},
		SourceData: map[string]any{
			"author":        author,
			"url":           "https://huggingface.co/" + m.ID,
			"pipelineTag":   m.PipelineTag,
			"task":          task,
			"library":       m.LibraryName,
			"license":       license,
			"gated":         gated,
			"quantFormats":  QuantFormats(m.Tags),
			"params":        params,
			"tags":          m.Tags,
			"baseModels":    refs.BaseModels,
			"datasets":      refs.Datasets,
			"arxiv":         refs.Arxiv,
			"languages":     refs.Languages,
			"trendingScore": m.TrendingScore,
			"createdAt":     parseTime(m.CreatedAt),
			"lastModified":  parseTime(m.LastModified),
		},
	}
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
