package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/elbaldfun/ghta/internal/service"
)

// respondErr maps service errors to HTTP status codes.
func respondErr(c *gin.Context, err error) {
	var inputErr service.InputError
	switch {
	case errors.As(err, &inputErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": inputErr.Error()})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// ---- Category ----

type CategoryHandler struct{ svc *service.CategoryService }

func NewCategoryHandler(svc *service.CategoryService) *CategoryHandler { return &CategoryHandler{svc} }

func (h *CategoryHandler) Register(r gin.IRoutes) {
	r.POST("/category", h.Create)
	r.GET("/category", h.FindAll)
	r.GET("/category/:id", h.FindOne)
	r.PATCH("/category/:id", h.Update)
	r.DELETE("/category/:id", h.Remove)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cat, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *CategoryHandler) FindAll(c *gin.Context) {
	tree, err := h.svc.FindAll(c.Request.Context())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tree})
}

func (h *CategoryHandler) FindOne(c *gin.Context) {
	cat, err := h.svc.FindOne(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, cat)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	cat, err := h.svc.Update(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, cat)
}

func (h *CategoryHandler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("id")); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ---- User ----

type UserHandler struct{ svc *service.UserService }

func NewUserHandler(svc *service.UserService) *UserHandler { return &UserHandler{svc} }

func (h *UserHandler) Register(r gin.IRoutes) {
	r.POST("/user", h.Create)
	r.GET("/user", h.FindAll)
	r.GET("/user/:id", h.FindOne)
	r.PATCH("/user/:id", h.Update)
	r.DELETE("/user/:id", h.Remove)
}

func (h *UserHandler) Create(c *gin.Context) {
	var in service.UserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	u, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *UserHandler) FindAll(c *gin.Context) {
	users, err := h.svc.FindAll(c.Request.Context())
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}

func (h *UserHandler) FindOne(c *gin.Context) {
	u, err := h.svc.FindOne(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	var in service.UserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	u, err := h.svc.Update(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("id")); err != nil {
		respondErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
