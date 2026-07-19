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

	s.save(ctx, source, externalID, points)
	return points, true
}

func (s *StarHistoryService) save(ctx context.Context, source domain.Source, externalID string, points []domain.StarPoint) {
	doc := domain.StarHistory{
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
}

// Warmup backfills the top-N repos by stars. Missing repos are fetched from
// ClickHouse in batches (one query per ~50 repos — far friendlier to the
// public playground than per-repo bursts); whatever a batch can't provide
// falls back to the per-repo path, slowly.
func (s *StarHistoryService) Warmup(ctx context.Context, top int) {
	opts := options.Find().
		SetSort(bson.D{{Key: "metrics.stars", Value: -1}}).
		SetLimit(int64(top)).
		SetProjection(bson.M{"externalId": 1})
	cur, err := s.store.Items().Find(ctx, bson.M{"source": domain.SourceGitHub}, opts)
	if err != nil {
		s.log.Error("history warmup: query failed", "err", err)
		return
	}
	var ids []string
	for cur.Next(ctx) {
		var it struct {
			ExternalID string `bson:"externalId"`
		}
		if cur.Decode(&it) == nil && repoNameRe.MatchString(it.ExternalID) {
			ids = append(ids, it.ExternalID)
		}
	}
	cur.Close(ctx)

	// Drop the ones already backfilled.
	haveCur, err := s.store.StarHistory().Find(ctx,
		bson.M{"source": domain.SourceGitHub, "externalId": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"externalId": 1}))
	if err != nil {
		s.log.Error("history warmup: existing lookup failed", "err", err)
		return
	}
	have := map[string]bool{}
	for haveCur.Next(ctx) {
		var doc struct {
			ExternalID string `bson:"externalId"`
		}
		if haveCur.Decode(&doc) == nil {
			have[doc.ExternalID] = true
		}
	}
	haveCur.Close(ctx)

	var missing []string
	for _, id := range ids {
		if !have[id] {
			missing = append(missing, id)
		}
	}
	s.log.Info("history warmup starting", "top", top, "missing", len(missing))

	const chunkSize = 50
	saved := 0
	for start := 0; start < len(missing); start += chunkSize {
		chunk := missing[start:min(start+chunkSize, len(missing))]
		byRepo := s.fetchClickHouseBatch(ctx, chunk)
		for _, id := range chunk {
			if points := byRepo[id]; len(points) > 0 {
				s.save(ctx, domain.SourceGitHub, id, points)
				saved++
			}
		}
		s.log.Info("history warmup progress", "done", start+len(chunk), "of", len(missing), "saved", saved)
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}

	// Per-repo fallback (OSS Insight) for whatever the batches couldn't cover.
	recovered := 0
	for _, id := range missing {
		if _, fetched := s.Ensure(ctx, domain.SourceGitHub, id); fetched {
			recovered++
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
	s.log.Info("history warmup complete", "missing", len(missing), "batchSaved", saved, "fallbackTried", recovered)
}

// fetchClickHouseBatch pulls the monthly cumulative curves for many repos in
// one query. Repos with no events simply don't appear in the result.
func (s *StarHistoryService) fetchClickHouseBatch(ctx context.Context, ids []string) map[string][]domain.StarPoint {
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		if repoNameRe.MatchString(id) {
			quoted = append(quoted, "'"+id+"'")
		}
	}
	if len(quoted) == 0 {
		return nil
	}
	sql := fmt.Sprintf(
		"SELECT repo_name, toStartOfMonth(created_at) AS m, count() AS c FROM github_events"+
			" WHERE repo_name IN (%s) AND event_type = 'WatchEvent'"+
			" GROUP BY repo_name, m ORDER BY repo_name, m FORMAT JSONCompact",
		strings.Join(quoted, ","))

	body := s.clickhouseQuery(ctx, sql, "batch:"+strconv.Itoa(len(quoted)))
	if body == nil {
		return nil
	}

	out := map[string][]domain.StarPoint{}
	totals := map[string]float64{}
	for _, row := range body {
		if len(row) != 3 {
			continue
		}
		repo, _ := row[0].(string)
		month, _ := row[1].(string)
		t, err := time.Parse("2006-01-02", month)
		if err != nil {
			continue
		}
		totals[repo] += toFloat(row[2])
		out[repo] = append(out[repo], domain.StarPoint{T: t, V: totals[repo]})
	}
	return out
}

// fetchClickHouse queries the public github_events dataset (GH Archive mirror,
// near-realtime) for monthly star-event counts and integrates them into a
// cumulative curve.
func (s *StarHistoryService) fetchClickHouse(ctx context.Context, externalID string) []domain.StarPoint {
	sql := fmt.Sprintf(
		"SELECT toStartOfMonth(created_at) AS m, count() AS c FROM github_events"+
			" WHERE repo_name = '%s' AND event_type = 'WatchEvent'"+
			" GROUP BY m ORDER BY m FORMAT JSONCompact", externalID)

	body := s.clickhouseQuery(ctx, sql, externalID)
	var points []domain.StarPoint
	total := 0.0
	for _, row := range body {
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

// clickhouseQuery runs one SQL statement against the public playground with a
// single backoff retry (it throttles sustained load with transient 5xx).
func (s *StarHistoryService) clickhouseQuery(ctx context.Context, sql, label string) [][]any {
	var res *http.Response
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://play.clickhouse.com/?user=play", strings.NewReader(sql))
		if err != nil {
			return nil
		}
		req.Header.Set("User-Agent", "ghta-star-history/1.0")

		res, err = s.http.Do(req)
		if err == nil && res.StatusCode == http.StatusOK {
			break
		}
		if err != nil {
			s.log.Warn("clickhouse query failed", "label", label, "err", err)
		} else {
			res.Body.Close()
			s.log.Warn("clickhouse query failed", "label", label, "status", res.StatusCode)
		}
		if attempt >= 1 {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	defer res.Body.Close()

	var body struct {
		Data [][]any `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(&body); err != nil {
		s.log.Warn("clickhouse decode failed", "label", label, "err", err)
		return nil
	}
	return body.Data
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
