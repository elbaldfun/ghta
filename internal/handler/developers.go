package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// DeveloperHandler serves the "developers to follow" boards.
type DeveloperHandler struct {
	svc *service.DeveloperService
}

func NewDeveloperHandler(svc *service.DeveloperService) *DeveloperHandler {
	return &DeveloperHandler{svc: svc}
}

func (h *DeveloperHandler) Register(r gin.IRoutes) {
	r.GET("/developers", h.List)
}

// List handles GET /developers?board=rising|merit&domain=ai&limit=&page=.
// domain defaults to "ai" (the launch focus); pass domain=all to span every
// domain.
func (h *DeveloperHandler) List(c *gin.Context) {
	board := c.DefaultQuery("board", "rising")
	if board != "rising" && board != "merit" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "board must be rising|merit"})
		return
	}
	domain := c.DefaultQuery("domain", "ai")

	limit := 24
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

	rows, total, err := h.svc.Ranking(c.Request.Context(), board, domain, limit, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":   rows,
		"total":  total,
		"page":   page,
		"board":  board,
		"domain": domain,
	})
}
