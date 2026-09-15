package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"erp/pkg/httpserver"
	stockclient "erp/services/assets-service/internal/adapters/stock"
	"erp/services/assets-service/internal/application"
	"erp/services/assets-service/internal/domain"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *application.Service
}

func New(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r *gin.Engine, jwt gin.HandlerFunc) {
	api := r.Group("/", jwt)
	api.GET("/assets", h.list)
	api.POST("/assets", h.create)
	api.POST("/assets/depreciate", h.depreciateAll)
	api.GET("/assets/:id", h.get)
	api.PUT("/assets/:id", h.update)
	api.POST("/assets/:id/transfer", h.transfer)
	api.POST("/assets/:id/depreciate", h.depreciate)
	api.POST("/assets/:id/dispose", h.dispose)
	api.GET("/assets/:id/movements", h.assetMovements)
	api.GET("/movements", h.movements)
}

func (h *Handler) withAuth(c *gin.Context) context.Context {
	return context.WithValue(c.Request.Context(), stockclient.AuthHeaderKey, c.GetHeader("Authorization"))
}

func (h *Handler) list(c *gin.Context) {
	out, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) create(c *gin.Context) {
	var in domain.Asset
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.Create(h.withAuth(c), in)
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) update(c *gin.Context) {
	var in domain.Asset
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	in.ID = c.Param("id")
	out, err := h.svc.Update(c.Request.Context(), in)
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) transfer(c *gin.Context) {
	var in domain.TransferInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.Transfer(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) depreciate(c *gin.Context) {
	asOf, err := parseAsOf(c)
	if err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.Depreciate(c.Request.Context(), c.Param("id"), asOf)
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) depreciateAll(c *gin.Context) {
	asOf, err := parseAsOf(c)
	if err != nil {
		httpserver.Error(c, http.StatusBadRequest, err)
		return
	}
	out, err := h.svc.DepreciateAll(c.Request.Context(), asOf)
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) dispose(c *gin.Context) {
	var in domain.DisposeInput
	_ = c.ShouldBindJSON(&in)
	out, err := h.svc.Dispose(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		httpserver.Error(c, statusOf(err), err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) assetMovements(c *gin.Context) {
	out, err := h.svc.ListMovements(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) movements(c *gin.Context) {
	out, err := h.svc.ListMovements(c.Request.Context(), "")
	if err != nil {
		httpserver.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func parseAsOf(c *gin.Context) (time.Time, error) {
	var in domain.DepreciateInput
	_ = c.ShouldBindJSON(&in)
	if in.AsOf != nil && !in.AsOf.IsZero() {
		return in.AsOf.UTC(), nil
	}
	return time.Now().UTC(), nil
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrInvalid), errors.Is(err, domain.ErrNotFixedAsset):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrDisposed), errors.Is(err, domain.ErrConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
