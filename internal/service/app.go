package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

const (
	appRankTTL = time.Hour
	appTopN    = 300
)

// validOS is the platform filter whitelist — also bounds the cache key space.
var validOS = map[string]bool{
	"macos": true, "windows": true, "linux": true, "android": true, "ios": true, "web": true,
}

// AppDownload is the one recommended installer for a platform — the file a
// "Download for macOS" button should point straight at.
type AppDownload struct {
	OS   string `json:"os"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// AppItem is one row of the open-source app directory: a downloadable app/CLI
// with the platforms it ships for, a primary download per platform, and the
// trust signals (stars, license) shown as secondary context.
type AppItem struct {
	ExternalID      string               `json:"externalId"`
	Name            string               `json:"name"`
	Description     string               `json:"description,omitempty"`
	Language        string               `json:"language,omitempty"`
	Homepage        string               `json:"homepage,omitempty"`
	License         string               `json:"license,omitempty"`
	Stars           int                  `json:"stars"`
	Forks           int                  `json:"forks"`
	Growth          int                  `json:"growth"`
	Type            string               `json:"type,omitempty"`
	Kind            string               `json:"kind"` // app | cli
	Platforms       []string             `json:"platforms"`
	PlatformSource  string               `json:"platformSource,omitempty"`
	CategoryPath    []string             `json:"categoryPath,omitempty"`
	LatestReleaseAt *time.Time           `json:"latestReleaseAt,omitempty"`
	Downloads       []AppDownload        `json:"downloads,omitempty"`
	AlternativeTo   []domain.Alternative `json:"alternativeTo,omitempty"`
	HasDownloads    bool                 `json:"hasDownloads"`
}

// downloadPriority ranks asset extensions per platform so the button points at
// the installer a user actually wants (a .dmg over a raw .zip, an installer
// .exe over a portable archive), not just the first asset GitHub lists.
var downloadPriority = map[string][]string{
	"macos":   {".dmg", ".pkg", ".app.zip", ".app.tar.gz"},
	"windows": {".exe", ".msi", ".appx", ".msix"},
	"linux":   {".appimage", ".deb", ".rpm", ".flatpak", ".snap"},
	"android": {".apk", ".aab"},
	"ios":     {".ipa"},
}

// primaryDownloads picks one best asset per platform from the matched release
// assets, ordered by platformOrder for stable output.
func primaryDownloads(assets []assetRow) []AppDownload {
	best := map[string]assetRow{}
	rank := func(os, name string) int {
		lower := strings.ToLower(name)
		for i, ext := range downloadPriority[os] {
			if strings.HasSuffix(lower, ext) {
				return i
			}
		}
		return len(downloadPriority[os]) + 1 // unranked archive: worst, but still usable
	}
	for _, a := range assets {
		if a.URL == "" {
			continue
		}
		cur, ok := best[a.Platform]
		if !ok || rank(a.Platform, a.Name) < rank(a.Platform, cur.Name) {
			best[a.Platform] = a
		}
	}
	var out []AppDownload
	for _, os := range []string{"macos", "windows", "linux", "android", "ios"} {
		if a, ok := best[os]; ok {
			out = append(out, AppDownload{OS: os, URL: a.URL, Name: a.Name})
		}
	}
	return out
}

// AppService ranks the open-source app directory. Cached per os+kind+category+sort.
type AppService struct {
	store *repository.Store
	cache *ttlCache[[]AppItem]
}

func NewAppService(store *repository.Store) *AppService {
	return &AppService{store: store, cache: newTTLCache[[]AppItem](appRankTTL)}
}

// Ranking returns one page of the directory. os filters to a platform;
// kind is app|cli|"" (all); category is a domain subtree; sort is
// hot|popular|new. Values are normalized so the cache key space stays bounded.
func (s *AppService) Ranking(ctx context.Context, os, kind, category, sort string, limit, page int) ([]AppItem, int, error) {
	if !validOS[os] {
		os = ""
	}
	if kind != "app" && kind != "cli" {
		kind = ""
	}
	switch sort {
	case "popular", "new":
	default:
		sort = "hot"
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if page < 1 {
		page = 1
	}

	rows, err := s.board(ctx, os, kind, category, sort)
	if err != nil {
		return nil, 0, err
	}
	total := len(rows)
	start := (page - 1) * limit
	if start >= total {
		return []AppItem{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
}

func (s *AppService) board(ctx context.Context, os, kind, category, sort string) ([]AppItem, error) {
	key := os + "|" + kind + "|" + category + "|" + sort
	return s.cache.get(ctx, key, func(ctx context.Context) ([]AppItem, error) {
		return s.compute(ctx, os, kind, category, sort)
	})
}

// appMatch builds the corpus filter: an item is in the directory when it's an
// app/cli OR ships platform builds, never a library. os/kind/category narrow it.
func appMatch(os, kind, category string) bson.M {
	m := bson.M{
		"type": bson.M{"$ne": "library"},
		"$or": bson.A{
			bson.M{"type": bson.M{"$in": bson.A{"app", "cli"}}},
			bson.M{"sourceData.platforms.0": bson.M{"$exists": true}},
		},
	}
	if os != "" {
		m["sourceData.platforms"] = os
	}
	switch kind {
	case "cli":
		m["type"] = "cli"
	case "app":
		m["type"] = bson.M{"$nin": bson.A{"cli", "library"}}
	}
	if category != "" {
		m["categoryPath"] = bson.M{"$regex": "^" + regexp.QuoteMeta(category) + "/"}
	}
	return m
}

func (s *AppService) compute(ctx context.Context, os, kind, category, sort string) ([]AppItem, error) {
	sortKey := "dailyIncrease"
	switch sort {
	case "popular":
		sortKey = "metrics.stars"
	case "new":
		sortKey = "sourceData.latestRelease.publishedAt"
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: appMatch(os, kind, category)}},
		bson.D{{Key: "$addFields", Value: bson.M{
			"growth":    bson.M{"$ifNull": bson.A{"$dailyIncrease", 0}},
			"platforms": bson.M{"$ifNull": bson.A{"$sourceData.platforms", bson.A{}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: sortKey, Value: -1}}}},
		bson.D{{Key: "$limit", Value: appTopN}},
		bson.D{{Key: "$project", Value: bson.M{
			"externalId":      1,
			"name":            1,
			"description":     1,
			"language":        1,
			"stars":           "$metrics.stars",
			"forks":           "$metrics.forks",
			"growth":          1,
			"type":            1,
			"platforms":       1,
			"platformSource":  "$sourceData.platformSource",
			"categoryPath":    1,
			"latestReleaseAt": "$sourceData.latestRelease.publishedAt",
			"homepage":        "$sourceData.homepageUrl",
			"license":         "$sourceData.license",
			"alternativeTo":   1,
			"releaseAssets":   bson.M{"$ifNull": bson.A{"$sourceData.releaseAssets", bson.A{}}},
			"assetCount":      bson.M{"$size": bson.M{"$ifNull": bson.A{"$sourceData.releaseAssets", bson.A{}}}},
		}}},
	}

	cur, err := s.store.Items().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("app ranking aggregate: %w", err)
	}
	var raw []appRow
	if err := cur.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("app ranking decode: %w", err)
	}

	rows := make([]AppItem, 0, len(raw))
	for _, r := range raw {
		kind := "app"
		if r.Type == "cli" {
			kind = "cli"
		}
		rows = append(rows, AppItem{
			ExternalID:      r.ExternalID,
			Name:            r.Name,
			Description:     r.Description,
			Language:        r.Language,
			Homepage:        r.Homepage,
			License:         r.License,
			Stars:           int(r.Stars),
			Forks:           int(r.Forks),
			Growth:          int(r.Growth),
			Type:            r.Type,
			Kind:            kind,
			Platforms:       r.Platforms,
			PlatformSource:  r.PlatformSource,
			CategoryPath:    r.CategoryPath,
			LatestReleaseAt: r.LatestReleaseAt,
			Downloads:       primaryDownloads(r.ReleaseAssets),
			AlternativeTo:   r.AlternativeTo,
			HasDownloads:    r.AssetCount > 0,
		})
	}
	return rows, nil
}

type appRow struct {
	ExternalID      string               `bson:"externalId"`
	Name            string               `bson:"name"`
	Description     string               `bson:"description"`
	Language        string               `bson:"language"`
	Homepage        string               `bson:"homepage"`
	License         string               `bson:"license"`
	Stars           float64              `bson:"stars"`
	Forks           float64              `bson:"forks"`
	Growth          float64              `bson:"growth"`
	Type            string               `bson:"type"`
	Platforms       []string             `bson:"platforms"`
	PlatformSource  string               `bson:"platformSource"`
	CategoryPath    []string             `bson:"categoryPath"`
	LatestReleaseAt *time.Time           `bson:"latestReleaseAt"`
	AlternativeTo   []domain.Alternative `bson:"alternativeTo"`
	ReleaseAssets   []assetRow           `bson:"releaseAssets"`
	AssetCount      int                  `bson:"assetCount"`
}

type assetRow struct {
	Name     string `bson:"name"`
	Platform string `bson:"platform"`
	URL      string `bson:"url"`
	Size     int64  `bson:"size"`
}
