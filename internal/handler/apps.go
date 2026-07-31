package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// AppHandler serves the open-source app directory.
type AppHandler struct {
	svc *service.AppService
}

func NewAppHandler(svc *service.AppService) *AppHandler {
	return &AppHandler{svc: svc}
}

func (h *AppHandler) Register(r gin.IRoutes) {
	r.GET("/apps", h.List)
	r.GET("/alternatives", h.AltIndex)
	r.GET("/apps/best/:major/:sub", h.BestOf)
	r.GET("/alternatives/:slug", h.ByAlternative)
}

// BestOf handles GET /apps/best/:major/:sub — the quality-scored collection for
// one shelf ("best open source note-taking apps").
func (h *AppHandler) BestOf(c *gin.Context) {
	shelf := c.Param("major") + "/" + c.Param("sub")
	rows, err := h.svc.BestOf(c.Request.Context(), shelf)
	if err != nil {
		if errors.As(err, &service.InputError{}) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown shelf"})
			return
		}
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "shelf": shelf})
}

// AltIndex handles GET /alternatives — paid products with the most open-source
// alternatives, most-covered first.
func (h *AppHandler) AltIndex(c *gin.Context) {
	targets, err := h.svc.AltTargets(c.Request.Context())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": targets})
}

// ByAlternative handles GET /alternatives/:slug — the open-source apps that
// replace one product.
func (h *AppHandler) ByAlternative(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	rows, name, err := h.svc.ByAlternative(c.Request.Context(), slug)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "slug": slug, "name": name, "total": len(rows)})
}

// List handles GET /apps?os=&kind=&category=&sort=hot|popular|new&limit=&page=.
// os/kind/category are normalized in the service; only limit/page are validated
// here (page is capped so a huge value can't overflow the offset math).
func (h *AppHandler) List(c *gin.Context) {
	os := c.Query("os")
	kind := c.Query("kind")
	category := c.Query("category")
	shelf := c.Query("shelf")
	sort := c.DefaultQuery("sort", "hot")

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
		if err != nil || n < 1 || n > 1000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be 1..1000"})
			return
		}
		page = n
	}

	rows, total, err := h.svc.Ranking(c.Request.Context(), os, kind, category, shelf, sort, limit, page)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "os": os, "kind": kind, "shelf": shelf, "sort": sort, "page": page})
}
