package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

// StarHistoryService lazily backfills long-term star curves from GH Archive
// mirrors (ClickHouse play, falling back to OSS Insight) and caches them
// forever in the star_history collection. Only repos whose detail page is
// actually requested ever cost an external query.
type StarHistoryService struct {
	store *repository.Store
	http  *http.Client
	log   *slog.Logger
}

func NewStarHistoryService(store *repository.Store, log *slog.Logger) *StarHistoryService {
	return &StarHistoryService{
		store: store,
		http:  &http.Client{Timeout: 20 * time.Second},
		log:   log,
	}
}

// repoNameRe matches "owner/name" with GitHub's allowed characters only —
// anything else is rejected before it reaches an external query.
var repoNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// Ensure returns the backfilled monthly star history for one repo, fetching
// and persisting it on first access. fetched reports whether a remote call
// was made (callers use it to pace bulk warm-ups). A nil result means no
// source could provide history — the caller falls back to snapshots only;
// failures are not cached, so the next request retries.
func (s *StarHistoryService) Ensure(ctx context.Context, source domain.Source, externalID string) (points []domain.StarPoint, fetched bool) {
	var doc domain.StarHistory
	err := s.store.StarHistory().
		FindOne(ctx, bson.M{"source": source, "externalId": externalID}).
		Decode(&doc)
	if err == nil {
		return doc.Points, false
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		s.log.Warn("star history lookup failed", "externalId", externalID, "err", err)
		return nil, false
	}
	if source != domain.SourceGitHub || !repoNameRe.MatchString(externalID) {
		return nil, false
	}

	points = s.fetchClickHouse(ctx, externalID)
	if len(points) == 0 {
		points = s.fetchOSSInsight(ctx, externalID)
	}
	if len(points) == 0 {
		return nil, true
	}

	doc = domain.StarHistory{
		Source:       source,
		ExternalID:   externalID,
		Points:       points,
		BackfilledAt: time.Now().UTC(),
	}
	if _, err := s.store.StarHistory().UpdateOne(ctx,
		bson.M{"source": source, "externalId": externalID},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	); err != nil {
		s.log.Warn("star history save failed", "externalId", externalID, "err", err)
	}
	return points, true
}

// fetchClickHouse queries the public github_events dataset (GH Archive mirror,
// near-realtime) for monthly star-event counts and integrates them into a
// cumulative curve.
func (s *StarHistoryService) fetchClickHouse(ctx context.Context, externalID string) []domain.StarPoint {
	sql := fmt.Sprintf(
		"SELECT toStartOfMonth(created_at) AS m, count() AS c FROM github_events"+
			" WHERE repo_name = '%s' AND event_type = 'WatchEvent'"+
			" GROUP BY m ORDER BY m FORMAT JSONCompact", externalID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://play.clickhouse.com/?user=play", strings.NewReader(sql))
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "ghta-star-history/1.0")

	res, err := s.http.Do(req)
	if err != nil {
		s.log.Warn("clickhouse query failed", "externalId", externalID, "err", err)
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		s.log.Warn("clickhouse query failed", "externalId", externalID, "status", res.StatusCode)
		return nil
	}

	var body struct {
		Data [][]any `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&body); err != nil {
		s.log.Warn("clickhouse decode failed", "externalId", externalID, "err", err)
		return nil
	}

	var points []domain.StarPoint
	total := 0.0
	for _, row := range body.Data {
		if len(row) != 2 {
			continue
		}
		month, _ := row[0].(string)
		t, err := time.Parse("2006-01-02", month)
		if err != nil {
			continue
		}
		total += toFloat(row[1])
		points = append(points, domain.StarPoint{T: t, V: total})
	}
	return points
}

// fetchOSSInsight is the fallback: OSS Insight already serves the cumulative
// monthly curve as JSON.
func (s *StarHistoryService) fetchOSSInsight(ctx context.Context, externalID string) []domain.StarPoint {
	url := fmt.Sprintf("https://api.ossinsight.io/v1/repos/%s/stargazers/history", externalID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "ghta-star-history/1.0")

	res, err := s.http.Do(req)
	if err != nil {
		s.log.Warn("ossinsight query failed", "externalId", externalID, "err", err)
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		s.log.Warn("ossinsight query failed", "externalId", externalID, "status", res.StatusCode)
		return nil
	}

	var body struct {
		Data struct {
			Rows []struct {
				Date       string `json:"date"`
				Stargazers string `json:"stargazers"`
			} `json:"rows"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&body); err != nil {
		s.log.Warn("ossinsight decode failed", "externalId", externalID, "err", err)
		return nil
	}

	var points []domain.StarPoint
	for _, row := range body.Data.Rows {
		t, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(row.Stargazers, 64)
		if err != nil {
			continue
		}
		points = append(points, domain.StarPoint{T: t, V: v})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].T.Before(points[j].T) })
	return points
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}
