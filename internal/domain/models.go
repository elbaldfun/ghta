package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Source identifies where a tracked item comes from.
type Source string

const (
	SourceGitHub   Source = "github"
	SourceAppStore Source = "appstore"
	SourceChrome   Source = "chrome"
	SourceMSStore  Source = "msstore"
)

// MetricDirection expresses whether a higher or lower primary metric is "better"
// (higher stars is better; lower store rank is better).
type MetricDirection string

const (
	DirectionDescBetter MetricDirection = "desc-better" // higher value ranks first
	DirectionAscBetter  MetricDirection = "asc-better"  // lower value ranks first (e.g. rank position)
)

// AnalysisStatus tracks AI categorization progress for an item.
const (
	AnalysisPending = "pending"
	AnalysisDone    = "done"
	AnalysisFailed  = "failed"
)

// TrackedItem is the source-agnostic main document. Every adapter normalizes its
// raw data into this shape; source-specific fields live under SourceData.
type TrackedItem struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Source       Source             `bson:"source" json:"source"`
	ExternalID   string             `bson:"externalId" json:"externalId"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description" json:"description"`
	Language     string             `bson:"language,omitempty" json:"language,omitempty"`
	CategoryID   []string           `bson:"categoryId" json:"categoryId"`
	CategoryPath string             `bson:"categoryPath,omitempty" json:"categoryPath"`

	PrimaryMetric   string             `bson:"primaryMetric" json:"primaryMetric"`
	MetricDirection MetricDirection    `bson:"metricDirection" json:"metricDirection"`
	Metrics         map[string]float64 `bson:"metrics" json:"metrics"`

	DailyIncrease   *float64 `bson:"dailyIncrease" json:"dailyIncrease"`
	WeeklyIncrease  *float64 `bson:"weeklyIncrease" json:"weeklyIncrease"`
	MonthlyIncrease *float64 `bson:"monthlyIncrease" json:"monthlyIncrease"`

	AnalysisStatus    string         `bson:"analysisStatus" json:"analysisStatus"`
	AnalysisFailCount int            `bson:"analysisFailCount" json:"analysisFailCount"`
	ClassifiedBy      string         `bson:"classifiedBy,omitempty" json:"classifiedBy,omitempty"` // rule | embedding | llm
	SourceData        map[string]any `bson:"sourceData,omitempty" json:"sourceData,omitempty"`

	FetchedAt time.Time `bson:"fetchedAt" json:"fetchedAt"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// Category is a node in the materialized-path classification tree.
type Category struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name        string              `bson:"name" json:"name"`
	Description string              `bson:"description,omitempty" json:"description,omitempty"`
	ParentID    *primitive.ObjectID `bson:"parentId" json:"parentId"`
	Level       int                 `bson:"level" json:"level"`
	Path        string              `bson:"path" json:"path"`
	CreatedBy   string              `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt   time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// User holds an authenticated account (OAuth binding added in the auth change).
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	GoogleID  string             `bson:"googleId,omitempty" json:"googleId,omitempty"`
	Email     string             `bson:"email,omitempty" json:"email,omitempty"`
	Name      string             `bson:"name,omitempty" json:"name,omitempty"`
	Avatar    string             `bson:"avatar,omitempty" json:"avatar,omitempty"`
	Role      string             `bson:"role" json:"role"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// FetchRunStatus is the lifecycle of one fetch shard.
type FetchRunStatus string

const (
	FetchPending FetchRunStatus = "pending"
	FetchRunning FetchRunStatus = "running"
	FetchDone    FetchRunStatus = "done"
	FetchFailed  FetchRunStatus = "failed"
)

// FetchRun records the progress of a single source+shard on a given day so the
// fetch job can resume after a crash and retry failed shards.
type FetchRun struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Source    Source             `bson:"source" json:"source"`
	Date      string             `bson:"date" json:"date"` // YYYY-MM-DD in UTC
	Shard     string             `bson:"shard" json:"shard"`
	Status    FetchRunStatus     `bson:"status" json:"status"`
	Error     string             `bson:"error,omitempty" json:"error,omitempty"`
	StartedAt *time.Time         `bson:"startedAt,omitempty" json:"startedAt,omitempty"`
	EndedAt   *time.Time         `bson:"endedAt,omitempty" json:"endedAt,omitempty"`
}

// CategorySuggestion records an AI proposal for a category that doesn't exist.
// Humans review these and, if accepted, add the path to taxonomy.yaml — the AI
// never mutates the tree itself.
type CategorySuggestion struct {
	Path      string    `bson:"path" json:"path"`
	Count     int       `bson:"count" json:"count"`
	Example   string    `bson:"example,omitempty" json:"example,omitempty"` // an item that triggered it
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// StarPoint is one point of a backfilled star-history curve.
type StarPoint struct {
	T time.Time `bson:"t" json:"t"`
	V float64   `bson:"v" json:"v"`
}

// StarHistory holds the once-backfilled long-term star curve for one repo
// (monthly granularity, sourced from GH Archive mirrors). Unlike
// metric_snapshots it has no TTL: history is written once and kept forever.
type StarHistory struct {
	Source       Source      `bson:"source" json:"source"`
	ExternalID   string      `bson:"externalId" json:"externalId"`
	Points       []StarPoint `bson:"points" json:"points"`
	BackfilledAt time.Time   `bson:"backfilledAt" json:"backfilledAt"`
}

// SnapshotMeta is the metaField of the metric_snapshots time-series collection.
type SnapshotMeta struct {
	Source     Source `bson:"source" json:"source"`
	ExternalID string `bson:"externalId" json:"externalId"`
}

// MetricSnapshot is an append-only time-series point for an item's metrics.
type MetricSnapshot struct {
	Meta       SnapshotMeta       `bson:"meta" json:"meta"`
	Metrics    map[string]float64 `bson:"metrics" json:"metrics"`
	CapturedAt time.Time          `bson:"capturedAt" json:"capturedAt"`
}
