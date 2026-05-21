package healthapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"headlessdesk/internal/control"
)

type Handler struct {
	service *control.Service
}

func RegisterRoutes(router gin.IRouter, service *control.Service) {
	handler := &Handler{service: service}
	router.GET("/healthz", handler.healthz)
}

func (h *Handler) healthz(c *gin.Context) {
	status := h.service.Status()
	code := http.StatusOK
	if !status.InputReady || !status.OutputReady {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}
