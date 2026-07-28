package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// ModelHandler serves the HuggingFace model boards.
type ModelHandler struct {
	svc *service.ModelService
}

func NewModelHandler(svc *service.ModelService) *ModelHandler {
	return &ModelHandler{svc: svc}
}

func (h *ModelHandler) Register(r gin.IRoutes) {
	r.GET("/models", h.List)
}

// List handles GET /models?task=&sort=hot|downloads|likes|new&limit=&page=.
// task/sort are normalized in the service (whitelisted cache keys).
func (h *ModelHandler) List(c *gin.Context) {
	task := c.Query("task")
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

	rows, total, err := h.svc.Ranking(c.Request.Context(), task, sort, limit, page)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "task": task, "sort": sort, "page": page})
}
