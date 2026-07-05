// Package handler wires HTTP routes to services.
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

type TrendingHandler struct {
	svc *service.TrendService
}

func NewTrendingHandler(svc *service.TrendService) *TrendingHandler {
	return &TrendingHandler{svc: svc}
}

// List handles GET /trending.
func (h *TrendingHandler) List(c *gin.Context) {
	q := service.TrendQuery{
		Source:   c.Query("source"),
		Stars:    c.Query("stars"),
		Issues:   c.Query("issues"),
		Language: c.Query("language"),
		Category: c.Query("category"),
		Sort:     c.Query("sort"),
	}
	if l := c.Query("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
			return
		}
		q.Limit = n
	}

	items, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Rising handles GET /trending/rising.
func (h *TrendingHandler) Rising(c *gin.Context) {
	q := service.RisingQuery{
		Window:   c.Query("window"),
		Source:   c.Query("source"),
		Category: c.Query("category"),
		Language: c.Query("language"),
	}
	if l := c.Query("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
			return
		}
		q.Limit = n
	}
	items, err := h.svc.Rising(c.Request.Context(), q)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
