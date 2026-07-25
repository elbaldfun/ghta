package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// EcosystemHandler serves the "AI stack" board (skills ∪ MCP ∪ agents).
type EcosystemHandler struct {
	svc *service.EcosystemService
}

func NewEcosystemHandler(svc *service.EcosystemService) *EcosystemHandler {
	return &EcosystemHandler{svc: svc}
}

func (h *EcosystemHandler) Register(r gin.IRoutes) {
	r.GET("/ecosystem", h.List)
}

// List handles GET /ecosystem?pillar=all|skill|mcp|agent&sort=hot|popular&limit=&page=.
func (h *EcosystemHandler) List(c *gin.Context) {
	pillar := c.DefaultQuery("pillar", "all")
	switch pillar {
	case "all", "skill", "mcp", "agent":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "pillar must be all|skill|mcp|agent"})
		return
	}
	sort := c.DefaultQuery("sort", "hot")
	if sort != "hot" && sort != "popular" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be hot|popular"})
		return
	}
	limit := 30
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be 1..100"})
			return
		}
		limit = n
	}
	page := 1
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be >= 1"})
			return
		}
		page = n
	}

	rows, total, err := h.svc.Ranking(c.Request.Context(), pillar, sort, limit, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "pillar": pillar, "sort": sort, "page": page})
}
