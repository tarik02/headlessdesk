package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"headlessdesk/internal/authz"
	"headlessdesk/internal/control"
)

type screenshotRequest struct {
	Crop *control.Crop `json:"crop"`
}

type clickRequest struct {
	X      int    `json:"x" binding:"required"`
	Y      int    `json:"y" binding:"required"`
	Button string `json:"button" binding:"required"`
}

type pointRequest struct {
	X int `json:"x" binding:"required"`
	Y int `json:"y" binding:"required"`
}

type dragRequest struct {
	Path []pointRequest `json:"path" binding:"required"`
}

type moveRequest struct {
	X int `json:"x" binding:"required"`
	Y int `json:"y" binding:"required"`
}

type scrollRequest struct {
	X       int `json:"x" binding:"required"`
	Y       int `json:"y" binding:"required"`
	ScrollX int `json:"scrollX"`
	ScrollY int `json:"scrollY"`
}

type keypressRequest struct {
	Key string `json:"key" binding:"required"`
}

type typeRequest struct {
	Text string `json:"text" binding:"required"`
}

type Handler struct {
	service    *control.Service
	authorizer *authz.Authorizer
}

func RegisterRoutes(router gin.IRouter, service *control.Service, authorizer *authz.Authorizer) {
	handler := &Handler{service: service, authorizer: authorizer}

	router.GET("/screenshot", handler.requireScope(authz.ScopeReadScreenshot), handler.screenshot)
	router.POST("/screenshot", handler.requireScope(authz.ScopeReadScreenshot), handler.screenshot)
	router.POST("/click", handler.requireScope(authz.ScopeWriteMouse), handler.click)
	router.POST("/double_click", handler.requireScope(authz.ScopeWriteMouse), handler.doubleClick)
	router.POST("/drag", handler.requireScope(authz.ScopeWriteMouse), handler.drag)
	router.POST("/move", handler.requireScope(authz.ScopeWriteMouse), handler.move)
	router.POST("/scroll", handler.requireScope(authz.ScopeWriteMouse), handler.scroll)
	router.POST("/keypress", handler.requireScope(authz.ScopeWriteKeyboard), handler.keypress)
	router.POST("/type", handler.requireScope(authz.ScopeWriteKeyboard), handler.typeText)
}

func (h *Handler) screenshot(c *gin.Context) {
	var req screenshotRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	png, err := h.service.Screenshot(control.ScreenshotCommand{Crop: req.Crop})
	if err != nil {
		h.writeUnavailable(c, err)
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}

func (h *Handler) click(c *gin.Context) {
	var req clickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Click(control.ClickCommand{X: req.X, Y: req.Y, Button: req.Button}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func (h *Handler) doubleClick(c *gin.Context) {
	var req clickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.DoubleClick(control.DoubleClickCommand{X: req.X, Y: req.Y, Button: req.Button}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func (h *Handler) drag(c *gin.Context) {
	var req dragRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	path := make([]control.Point, 0, len(req.Path))
	for _, point := range req.Path {
		path = append(path, control.Point{X: point.X, Y: point.Y})
	}

	if err := h.service.Drag(control.DragCommand{Path: path}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func (h *Handler) move(c *gin.Context) {
	var req moveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Move(control.MoveCommand{X: req.X, Y: req.Y}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func (h *Handler) scroll(c *gin.Context) {
	var req scrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Scroll(control.ScrollCommand{
		X:       req.X,
		Y:       req.Y,
		ScrollX: req.ScrollX,
		ScrollY: req.ScrollY,
	}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func (h *Handler) keypress(c *gin.Context) {
	var req keypressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Keypress(control.KeypressCommand{Key: req.Key}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func (h *Handler) typeText(c *gin.Context) {
	var req typeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Type(control.TypeCommand{Text: req.Text}); err != nil {
		h.writeUnavailable(c, err)
		return
	}
	h.writeOK(c)
}

func bindOptionalJSON(c *gin.Context, dst any) error {
	if c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (h *Handler) requireScope(scope authz.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.authorizer.AuthorizeRequest(c.Request, authz.AudienceHTTP, scope); err != nil {
			if err.StatusCode == http.StatusUnauthorized || err.StatusCode == http.StatusForbidden {
				c.Header("WWW-Authenticate", "Bearer")
			}
			c.JSON(err.StatusCode, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) writeBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (h *Handler) writeUnavailable(c *gin.Context, err error) {
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "status": h.service.Status()})
}

func (h *Handler) writeOK(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
