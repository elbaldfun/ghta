package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// TopicHandler serves the "hot topics in a domain" board.
type TopicHandler struct {
	svc *service.TopicService
}

func NewTopicHandler(svc *service.TopicService) *TopicHandler {
	return &TopicHandler{svc: svc}
}

func (h *TopicHandler) Register(r gin.IRoutes) {
	r.GET("/topics", h.List)
}

// List handles GET /topics?domain=ai&sort=hot|popular&limit=.
func (h *TopicHandler) List(c *gin.Context) {
	domain := c.DefaultQuery("domain", "ai")
	sort := c.DefaultQuery("sort", "hot")
	if sort != "hot" && sort != "popular" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be hot|popular"})
		return
	}
	limit := 40
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 60 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be 1..60"})
			return
		}
		limit = n
	}

	rows, err := h.svc.Ranking(c.Request.Context(), domain, sort, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "domain": domain, "sort": sort})
}
