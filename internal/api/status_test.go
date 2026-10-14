package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/boards"
	"github.com/elbaldfun/ghta/internal/item"
	"github.com/elbaldfun/ghta/internal/store"
)

func TestBadge(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	it := ghRepo("o/app", "Go", 5000, 10, "devtools/terminal", "agents/coding")
	it.Ranks = &item.Ranks{Overall: 40, Language: 7, Domain: map[string]int{"devtools/terminal": 9, "agents/coding": 4}}
	lang := ghRepo("o/lang", "Go", 900, 0)
	lang.Ranks = &item.Ranks{Overall: 900, Language: 120}
	gone := ghRepo("o/gone", "Go", 5000, 0)
	gone.Ranks, gone.Stale = &item.Ranks{Overall: 41}, true
	seed(t, st, it, lang, gone)
	must(t, st.PutBoard(ctx, boards.CategoriesKey, []boards.CategoryNode{
		{Path: "agents", Name: "智能体", Children: []boards.CategoryNode{{Path: "agents/coding", Name: "编程智能体", NameEn: "Coding Agents"}}},
	}, time.Now()))
	r := router(st)

	body := func(url string, code int) string {
		t.Helper()
		w := get(t, r, url, nil, nil)
		if w.Code != code || !strings.HasPrefix(w.Header().Get("Content-Type"), "image/svg+xml") ||
			w.Header().Get("Cache-Control") != "public, max-age=3600" {
			t.Fatalf("%s = %d %v", url, w.Code, w.Header())
		}
		return w.Body.String()
	}
	if b := body("/v1/badge/o/app.svg", http.StatusOK); !strings.Contains(b, ">#4 in Coding Agents<") {
		t.Fatalf("badge = %s; want the best domain, named", b)
	}
	if b := body("/v1/badge/o/app.svg?lang=zh&theme=dark", http.StatusOK); !strings.Contains(b, ">#4 编程智能体<") || !strings.Contains(b, "#2ea043") {
		t.Fatalf("zh dark badge = %s", b)
	}
	if b := body("/v1/badge/o/lang.svg", http.StatusOK); !strings.Contains(b, ">#120 in Go<") {
		t.Fatalf("language badge = %s", b)
	}
	for _, url := range []string{"/v1/badge/o/gone.svg", "/v1/badge/o/none.svg"} {
		if b := body(url, http.StatusNotFound); !strings.Contains(b, "not ranked") {
			t.Fatalf("%s = %s", url, b)
		}
	}
	if w := get(t, r, "/v1/badge/o/app.png", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("png = %d", w.Code)
	}
}

func TestHealthAndJobs(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	a := New(st, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	a.RegisterHealth(r)
	a.RegisterInternal(r)

	var h struct {
		Status string         `json:"status"`
		Fetch  map[string]any `json:"fetch"`
	}
	if w := get(t, r, "/health", nil, &h); w.Code != http.StatusOK || h.Status != "degraded" {
		t.Fatalf("no fetch yet = %d %+v; still alive", w.Code, h)
	}
	if w := get(t, r, "/health?strict=1", nil, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("strict, no fetch = %d", w.Code)
	}

	run := func(job string, start time.Time, status string) {
		id, err := st.StartRun(ctx, job, start)
		must(t, err)
		must(t, st.FinishRun(ctx, id, store.JobRun{Status: status, FinishedAt: start.Add(time.Hour)}))
	}
	now := time.Now().UTC()
	run("fetch", now.Add(-50*time.Hour), store.RunDone)
	run("fetch", now.Add(-3*time.Hour), store.RunFailed)
	if w := get(t, r, "/health", nil, &h); w.Code != http.StatusOK || h.Status != "degraded" || h.Fetch["ageHours"] != 49.0 {
		t.Fatalf("stale = %d %+v; a failed run is not a fetch", w.Code, h)
	}
	if w := get(t, r, "/health?strict=1", nil, nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("strict, stale = %d", w.Code)
	}
	run("fetch", now.Add(-5*time.Hour), store.RunDone)
	run("rank", now.Add(-2*time.Hour), store.RunDone)
	if w := get(t, r, "/health?strict=1", nil, &h); w.Code != http.StatusOK || h.Status != "ok" {
		t.Fatalf("fresh = %d %+v", w.Code, h)
	}

	var j struct {
		Latest []store.JobRun `json:"latest"`
		Runs   []store.JobRun `json:"runs"`
	}
	if w := get(t, r, "/internal/jobs?job=fetch&limit=2", nil, &j); w.Code != http.StatusOK || len(j.Latest) != 2 ||
		j.Latest[0].Job != "fetch" || j.Latest[0].Status != store.RunFailed || len(j.Runs) != 2 {
		t.Fatalf("jobs = %d %+v", w.Code, j)
	}
}
