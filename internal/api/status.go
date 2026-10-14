package api

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// fetchStale is how old the last good fetch may be before the data counts as
// stale: the fetch runs nightly, so a day and a half means a night was missed.
const fetchStale = 36 * time.Hour

// RegisterHealth mounts GET /health at the root.
func (a *API) RegisterHealth(r gin.IRouter) {
	r.GET("/health", a.health)
}

// RegisterInternal mounts the operator routes; r must already require the
// admin token.
func (a *API) RegisterInternal(r gin.IRouter) {
	r.GET("/internal/jobs", a.jobs)
}

// GET /health?strict=1 — liveness, and how fresh the data is. It answers 200
// while the process serves, so deploys and the container check do not hinge
// on the nightly fetch; with strict=1 it answers 503 when the last good fetch
// is older than fetchStale, for an uptime monitor to alert on.
func (a *API) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	fetch := gin.H{"ok": false}
	at, ok, err := a.st.LastDone(ctx, "fetch")
	switch {
	case err != nil:
		a.log.Warn("health: read job runs", "err", err)
		fetch["error"] = "database unavailable"
	case ok:
		age := time.Since(at)
		fetch["lastAt"] = at
		fetch["ageHours"] = math.Round(age.Hours()*10) / 10
		fetch["ok"] = age <= fetchStale
	}
	status, code := "ok", http.StatusOK
	if fetch["ok"] != true {
		status = "degraded"
		if c.Query("strict") == "1" {
			code = http.StatusServiceUnavailable
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(code, gin.H{"status": status, "fetch": fetch})
}

// GET /internal/jobs?job=&limit=50 — each job's latest run, and the latest
// runs of one job or of all, newest first.
func (a *API) jobs(c *gin.Context) {
	limit, err := intParam(c, "limit", 50, 1, 500)
	if err != nil {
		a.fail(c, err)
		return
	}
	ctx := c.Request.Context()
	latest, err := a.st.LatestRuns(ctx)
	if err != nil {
		a.fail(c, err)
		return
	}
	runs, err := a.st.Runs(ctx, c.Query("job"), limit)
	if err != nil {
		a.fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"latest": latest, "runs": runs})
}
