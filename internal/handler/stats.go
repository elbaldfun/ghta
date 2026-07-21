package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/domain"
	"github.com/elbaldfun/ghta/internal/repository"
)

const (
	defaultLanguageLimit = 25
	maxLanguageLimit     = 100
)

type StatsHandler struct {
	store *repository.Store
}

func NewStatsHandler(store *repository.Store) *StatsHandler {
	return &StatsHandler{store: store}
}

func (h *StatsHandler) Register(r gin.IRoutes) {
	r.GET("/stats/languages", h.Languages)
}

// Languages handles GET /stats/languages — per-language corpus totals, used by
// the site's own analysis content. Read-only and public, like /trending.
func (h *StatsHandler) Languages(c *gin.Context) {
	limit := defaultLanguageLimit
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxLanguageLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be 1..100"})
			return
		}
		limit = n
	}

	src := domain.Source(c.DefaultQuery("source", string(domain.SourceGitHub)))
	stats, err := h.store.LanguageStats(c.Request.Context(), src, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}
