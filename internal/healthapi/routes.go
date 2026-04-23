package healthapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"libfreerdp-golang-poc/internal/control"
)

type Handler struct {
	service *control.Service
}

func RegisterRoutes(router gin.IRouter, service *control.Service) {
	handler := &Handler{service: service}
	router.GET("/healthz", handler.healthz)
}

func (h *Handler) healthz(c *gin.Context) {
	c.JSON(http.StatusOK, h.service.Status())
}
