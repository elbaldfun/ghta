// Package item is the data model of the rebuilt store (docs/REBUILD.md §4).
//
// An Item is the hot document every ranking scans: small, typed, and split by
// who owns each part — the fetcher owns the identity/metrics/Gh/Hf fields, the
// judge owns Labels, the asset jobs own Assets, the rank job owns Ranks. Large
// cold text (README, release notes) lives beside it in a Blob.
//
// Nothing here tolerates a legacy shape. Old tracked_items documents are
// normalised once, by internal/importer.
package item

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Collection names of the rebuilt store.
const (
	CollItems = "items"
	CollBlobs = "item_blobs"
)

// Sources.
const (
	SourceGitHub      = "github"
	SourceHuggingFace = "huggingface"
)

// Job status values.
const (
	StatusPending = "pending"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// ShelfExcluded is the shelf verdict "not a runnable app".
const ShelfExcluded = "excluded"

// Who produced a set of labels.
const (
	ByRule   = "rule"
	ByLLM    = "llm"
	ByManual = "manual"
	BySource = "source" // the source's own taxonomy (HuggingFace task)
)

// Item is one tracked project.
type Item struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Source      string             `bson:"source"`
	ExternalID  string             `bson:"externalId"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	Language    string             `bson:"language,omitempty"`
	Homepage    string             `bson:"homepage,omitempty"`
	License     string             `bson:"license,omitempty"`
	Topics      []string           `bson:"topics"`

	PrimaryMetric   string             `bson:"primaryMetric"`
	MetricDirection string             `bson:"metricDirection"`
	Metrics         map[string]float64 `bson:"metrics"`
	Growth          Growth             `bson:"growth"`
	History         []Point            `bson:"history"`

	Labels Labels `bson:"labels"`
	Assets Assets `bson:"assets,omitempty"`
	Ranks  *Ranks `bson:"ranks,omitempty"`
	Jobs   Jobs   `bson:"jobs"`

	Gh *GitHub      `bson:"gh,omitempty"`
	Hf *HuggingFace `bson:"hf,omitempty"`

	Stale       bool      `bson:"stale,omitempty"`
	StaleReason string    `bson:"staleReason,omitempty"`
	StaleAt     time.Time `bson:"staleAt,omitempty"`

	FetchedAt time.Time `bson:"fetchedAt"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

// Growth is the primary metric's change over 1, 7 and 30 days. A nil window
// means there is no observation that far back yet — not "zero growth".
type Growth struct {
	D1  *float64 `bson:"d1"`
	D7  *float64 `bson:"d7"`
	D30 *float64 `bson:"d30"`
}

// Point is one day's closing value of the primary metric. D is a UTC date,
// "2006-01-02", so the ring sorts and dedupes as strings.
type Point struct {
	D string  `bson:"d"`
	V float64 `bson:"v"`
}

// HistoryDays is how many daily points the History ring keeps: enough for the
// 30-day growth window plus a few days of fetch gaps.
const HistoryDays = 35

// Labels is everything the judge decides about an item.
type Labels struct {
	// Domain holds taxonomy leaf paths, apps/ shelves included, or the single
	// source-namespaced path of a HuggingFace model ("hf/<task>").
	Domain []string `bson:"domain"`
	Type   string   `bson:"type,omitempty"`
	// Shelf is the effective store shelf: the override when one is set, else the
	// judged slug, ShelfExcluded, or "" for not judged yet.
	Shelf         string        `bson:"shelf,omitempty"`
	ShelfOverride string        `bson:"shelfOverride,omitempty"`
	HasGui        *bool         `bson:"hasGui,omitempty"`
	TaglineZh     string        `bson:"taglineZh,omitempty"`
	TaglineEn     string        `bson:"taglineEn,omitempty"`
	Alternatives  []Alternative `bson:"alternatives,omitempty"`
	Tags          []string      `bson:"tags,omitempty"`
	By            string        `bson:"by,omitempty"`
	// Confidence is how the judge's answers fell: "agreed", "majority" or
	// "split" (judge.Agreed and friends). A split item wants a human look.
	Confidence string `bson:"confidence,omitempty"`
	// Version is the taxonomy tree generation Domain was judged on.
	Version int `bson:"version,omitempty"`
	// Prev snapshots Domain before a taxonomy migration rewrote it.
	Prev []string  `bson:"prev,omitempty"`
	At   time.Time `bson:"at,omitempty"`
}

// Alternative is a commercial product an open-source app can replace.
type Alternative struct {
	Name string `bson:"name"`
	Slug string `bson:"slug"`
	Kind string `bson:"kind,omitempty"`
}

// Assets are the images extracted for an app.
type Assets struct {
	IconURL       string `bson:"iconUrl,omitempty"`
	ScreenshotURL string `bson:"screenshotUrl,omitempty"`
	ShotSource    string `bson:"shotSource,omitempty"` // readme | og
}

// Ranks are the item's positions within its source, by primary metric, written
// nightly by the rank job. At is when the positions last changed.
type Ranks struct {
	Overall  int            `bson:"overall"`
	Language int            `bson:"language,omitempty"`
	Domain   map[string]int `bson:"domain,omitempty"`
	At       time.Time      `bson:"at"`
}

// JobState is one job's progress on one item. Every job uses the same shape.
type JobState struct {
	Status string `bson:"status,omitempty"`
	Fail   int    `bson:"fail,omitempty"`
	// Version is the prompt/enum generation that processed the item, for jobs
	// that re-queue the corpus by bumping it.
	Version int       `bson:"version,omitempty"`
	At      time.Time `bson:"at,omitempty"`
}

// Jobs holds per-job state. Store and Alt are separate from Judge until the
// three LLM passes are merged (REBUILD M3); after that only Judge remains.
type Jobs struct {
	Judge JobState `bson:"judge"`
	Store JobState `bson:"store,omitempty"`
	Alt   JobState `bson:"alt,omitempty"`
	Icon  JobState `bson:"icon,omitempty"`
	Shot  JobState `bson:"shot,omitempty"`
}

// GitHub holds the GitHub-only fields of an item.
type GitHub struct {
	Owner          string         `bson:"owner"`
	PushedAt       time.Time      `bson:"pushedAt,omitempty"`
	Platforms      []string       `bson:"platforms,omitempty"`
	PlatformSource string         `bson:"platformSource,omitempty"`
	ReleaseAssets  []ReleaseAsset `bson:"releaseAssets,omitempty"`
	LatestRelease  *ReleaseRef    `bson:"latestRelease,omitempty"`
}

// ReleaseAsset is a downloadable release file resolved to a platform.
type ReleaseAsset struct {
	Name     string `bson:"name"`
	Platform string `bson:"platform"`
	URL      string `bson:"url,omitempty"`
	Size     int64  `bson:"size,omitempty"`
}

// ReleaseRef identifies the latest release.
type ReleaseRef struct {
	Tag         string    `bson:"tag"`
	PublishedAt time.Time `bson:"publishedAt,omitempty"`
}

// HuggingFace holds the HuggingFace-only fields of a model.
type HuggingFace struct {
	Author        string    `bson:"author"`
	Task          string    `bson:"task,omitempty"`
	PipelineTag   string    `bson:"pipelineTag,omitempty"`
	Library       string    `bson:"library,omitempty"`
	Gated         bool      `bson:"gated,omitempty"`
	QuantFormats  []string  `bson:"quantFormats,omitempty"`
	Params        int64     `bson:"params,omitempty"`
	Tags          []string  `bson:"tags,omitempty"`
	BaseModels    []string  `bson:"baseModels,omitempty"`
	Datasets      []string  `bson:"datasets,omitempty"`
	Arxiv         []string  `bson:"arxiv,omitempty"`
	Languages     []string  `bson:"languages,omitempty"`
	TrendingScore float64   `bson:"trendingScore,omitempty"`
	CreatedAt     time.Time `bson:"createdAt,omitempty"`
	LastModified  time.Time `bson:"lastModified,omitempty"`
}

// Blob is an item's cold text, keyed like the item.
type Blob struct {
	Source     string    `bson:"source"`
	ExternalID string    `bson:"externalId"`
	Readme     string    `bson:"readme,omitempty"`
	Releases   []Release `bson:"releases,omitempty"`
	// Hash is BlobHash of the content. The fetcher compares it to skip
	// rewriting a README that has not changed, which is nearly all of them.
	Hash      string    `bson:"hash"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

// NewBlob returns the blob for an item's cold text, or nil when there is none.
func NewBlob(source, externalID, readme string, releases []Release, at time.Time) *Blob {
	if readme == "" && len(releases) == 0 {
		return nil
	}
	return &Blob{
		Source: source, ExternalID: externalID, Readme: readme, Releases: releases,
		Hash: BlobHash(readme, releases), UpdatedAt: at,
	}
}

// BlobHash identifies a blob's content.
func BlobHash(readme string, releases []Release) string {
	h := sha256.New()
	h.Write([]byte(readme))
	for _, r := range releases {
		fmt.Fprintf(h, "\x00%s\x00%s\x00%t%t%t\x00%d", r.Name, r.TagName, r.IsPrerelease, r.IsLatest, r.IsDraft, r.PublishedAt.UnixMilli())
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Fetched is what a source adapter returns for one item: the fields the
// fetcher owns, filled in, and the item's cold text. Everything owned by
// another job (most of Labels, Assets, Ranks, Jobs) is left zero and is never
// written from here.
//
// One exception: a source that carries its own taxonomy (HuggingFace's task)
// sets Labels.Domain and Labels.By, and the fetcher stores them.
type Fetched struct {
	Item Item
	Blob *Blob
}

// Release is one entry of a repository's release list.
type Release struct {
	Name         string    `bson:"name,omitempty"`
	TagName      string    `bson:"tagName"`
	IsPrerelease bool      `bson:"isPrerelease,omitempty"`
	IsLatest     bool      `bson:"isLatest,omitempty"`
	IsDraft      bool      `bson:"isDraft,omitempty"`
	PublishedAt  time.Time `bson:"publishedAt,omitempty"`
}
