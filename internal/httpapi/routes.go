package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"headlessdesk/internal/apiinput"
	"headlessdesk/internal/authz"
	"headlessdesk/internal/control"
)

type screenshotRequest struct {
	Crop *cropRequest `json:"crop"`
}

type clickRequest struct {
	X      *apiinput.Number `json:"x"`
	Y      *apiinput.Number `json:"y"`
	Button string           `json:"button" binding:"required"`
}

type pointRequest struct {
	X *apiinput.Number `json:"x"`
	Y *apiinput.Number `json:"y"`
}

type cropRequest struct {
	X *apiinput.Number `json:"x"`
	Y *apiinput.Number `json:"y"`
	W *apiinput.Number `json:"w"`
	H *apiinput.Number `json:"h"`
}

type dragRequest struct {
	Path []pointRequest `json:"path" binding:"required"`
}

type moveRequest struct {
	X *apiinput.Number `json:"x"`
	Y *apiinput.Number `json:"y"`
}

type scrollRequest struct {
	X       *apiinput.Number `json:"x"`
	Y       *apiinput.Number `json:"y"`
	ScrollX apiinput.Number  `json:"scrollX"`
	ScrollY apiinput.Number  `json:"scrollY"`
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

	crop, err := req.Crop.controlCrop()
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}

	png, err := h.service.Screenshot(control.ScreenshotCommand{Crop: crop})
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

	x, y, err := req.point("")
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Click(control.ClickCommand{X: x, Y: y, Button: req.Button}); err != nil {
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

	x, y, err := req.point("")
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.DoubleClick(control.DoubleClickCommand{X: x, Y: y, Button: req.Button}); err != nil {
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
	for i, point := range req.Path {
		x, y, err := point.point(fmt.Sprintf("path[%d].", i))
		if err != nil {
			h.writeBadRequest(c, err)
			return
		}
		path = append(path, control.Point{X: x, Y: y})
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

	x, y, err := req.point("")
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Move(control.MoveCommand{X: x, Y: y}); err != nil {
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

	x, y, err := req.point("")
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}
	scrollX, err := req.ScrollX.Int("scrollX")
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}
	scrollY, err := req.ScrollY.Int("scrollY")
	if err != nil {
		h.writeBadRequest(c, err)
		return
	}

	if err := h.service.Scroll(control.ScrollCommand{
		X:       x,
		Y:       y,
		ScrollX: scrollX,
		ScrollY: scrollY,
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

func (r *clickRequest) point(prefix string) (float64, float64, error) {
	return parsePoint(prefix, r.X, r.Y)
}

func (r *pointRequest) point(prefix string) (float64, float64, error) {
	return parsePoint(prefix, r.X, r.Y)
}

func (r *moveRequest) point(prefix string) (float64, float64, error) {
	return parsePoint(prefix, r.X, r.Y)
}

func (r *scrollRequest) point(prefix string) (float64, float64, error) {
	return parsePoint(prefix, r.X, r.Y)
}

func parsePoint(prefix string, xValue *apiinput.Number, yValue *apiinput.Number) (float64, float64, error) {
	x, err := requiredNumber(prefix+"x", xValue)
	if err != nil {
		return 0, 0, err
	}
	y, err := requiredNumber(prefix+"y", yValue)
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func requiredNumber(field string, value *apiinput.Number) (float64, error) {
	if value == nil || !value.Set() {
		return 0, fmt.Errorf("%s is required", field)
	}
	return value.Float64(field)
}

func (r *cropRequest) controlCrop() (*control.Crop, error) {
	if r == nil {
		return nil, nil
	}
	x, err := optionalInt("crop.x", r.X)
	if err != nil {
		return nil, err
	}
	y, err := optionalInt("crop.y", r.Y)
	if err != nil {
		return nil, err
	}
	w, err := optionalInt("crop.w", r.W)
	if err != nil {
		return nil, err
	}
	h, err := optionalInt("crop.h", r.H)
	if err != nil {
		return nil, err
	}
	return &control.Crop{X: x, Y: y, W: w, H: h}, nil
}

func optionalInt(field string, value *apiinput.Number) (*int, error) {
	if value == nil || !value.Set() {
		return nil, nil
	}
	result, err := value.Int(field)
	if err != nil {
		return nil, err
	}
	return &result, nil
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
