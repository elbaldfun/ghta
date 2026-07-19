// Package handler wires HTTP routes to services.
package handler

import (
	"net/http"
	"strconv"
	"strings"

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
		Q:        c.Query("q"),
		License:  c.Query("license"),
		Sort:     c.Query("sort"),
	}
	if topics := c.Query("topics"); topics != "" {
		for _, t := range strings.Split(topics, ",") {
			if t = strings.TrimSpace(t); t != "" {
				q.Topics = append(q.Topics, t)
			}
		}
	}
	if l := c.Query("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
			return
		}
		q.Limit = n
	}
	if p := c.Query("page"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a number"})
			return
		}
		q.Page = n
	}

	items, total, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
}

// Item handles GET /trending/item?source=&externalId= — one item plus its
// snapshot history, for the detail page.
func (h *TrendingHandler) Item(c *gin.Context) {
	item, history, err := h.svc.Item(c.Request.Context(), c.Query("source"), c.Query("externalId"))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"item": item, "history": history}})
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
