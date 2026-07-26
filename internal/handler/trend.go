package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

const (
	maxTrendIDs  = 60
	maxTrendDays = 400
)

// TrendHandler serves per-repo star time series from the snapshot history, for
// sparklines on the boards and repo pages. Real snapshots only — never
// interpolated.
type TrendHandler struct {
	store *repository.Store
}

func NewTrendHandler(store *repository.Store) *TrendHandler {
	return &TrendHandler{store: store}
}

func (h *TrendHandler) Register(r gin.IRoutes) {
	r.GET("/trend", h.List)
}

// List handles GET /trend?ids=owner/repo,owner2/repo2&days=30.
func (h *TrendHandler) List(c *gin.Context) {
	raw := c.Query("ids")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required"})
		return
	}
	ids := make([]string, 0, maxTrendIDs)
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
		if len(ids) >= maxTrendIDs {
			break
		}
	}

	days := 30
	if v := c.Query("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxTrendDays {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be 1..400"})
			return
		}
		days = n
	}

	from := time.Now().UTC().AddDate(0, 0, -days)
	series, err := h.store.SnapshotSeries(c.Request.Context(), domain.SourceGitHub, ids, from)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": series})
}
