package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// HeatmapHandler serves the ecosystem map: every domain with its scale and
// current velocity.
type HeatmapHandler struct {
	svc *service.HeatmapService
}

func NewHeatmapHandler(svc *service.HeatmapService) *HeatmapHandler {
	return &HeatmapHandler{svc: svc}
}

func (h *HeatmapHandler) Register(r gin.IRoutes) {
	r.GET("/category/heat", h.List)
}

// List handles GET /category/heat.
func (h *HeatmapHandler) List(c *gin.Context) {
	cells, err := h.svc.Map(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cells})
}
