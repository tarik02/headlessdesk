package mcpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"headlessdesk/internal/apiinput"
	"headlessdesk/internal/authz"
	"headlessdesk/internal/control"
	"headlessdesk/internal/version"
)

func NewServer(service *control.Service, authorizer *authz.Authorizer) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "headlessdesk",
		Version: version.Get().Version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "session_status",
		Description: "Return the current remote desktop session status and framebuffer metadata",
	}, requireToolScope(authorizer, authz.ScopeReadStatus, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, control.Status, error) {
		status := service.Status()
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf(
						"connected=%t active=%t ready=%t state=%s size=%dx%d",
						status.Connected,
						status.Active,
						status.Ready,
						status.State,
						status.Width,
						status.Height,
					),
				},
			},
		}, status, nil
	}))

	type cropArgs struct {
		X *apiinput.Number `json:"x"`
		Y *apiinput.Number `json:"y"`
		W *apiinput.Number `json:"w"`
		H *apiinput.Number `json:"h"`
	}
	type screenshotArgs struct {
		Crop *cropArgs `json:"crop"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "screenshot",
		Description: "Capture the latest remote desktop framebuffer as a PNG image, optionally cropped",
		InputSchema: screenshotInputSchema(),
	}, requireToolScope(authorizer, authz.ScopeReadScreenshot, func(ctx context.Context, req *mcp.CallToolRequest, args screenshotArgs) (*mcp.CallToolResult, any, error) {
		var crop *control.Crop
		if args.Crop != nil {
			x, err := apiinput.OptionalInt("crop.x", args.Crop.X)
			if err != nil {
				return nil, nil, err
			}
			y, err := apiinput.OptionalInt("crop.y", args.Crop.Y)
			if err != nil {
				return nil, nil, err
			}
			w, err := apiinput.OptionalInt("crop.w", args.Crop.W)
			if err != nil {
				return nil, nil, err
			}
			h, err := apiinput.OptionalInt("crop.h", args.Crop.H)
			if err != nil {
				return nil, nil, err
			}
			crop = &control.Crop{X: x, Y: y, W: w, H: h}
		}
		png, err := service.Screenshot(control.ScreenshotCommand{Crop: crop})
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.ImageContent{
					MIMEType: "image/png",
					Data:     png,
				},
			},
		}, nil, nil
	}))

	type clickArgs struct {
		X      *apiinput.Number `json:"x"`
		Y      *apiinput.Number `json:"y"`
		Button string           `json:"button"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "click",
		Description: "Move to a coordinate and click the specified mouse button",
		InputSchema: clickInputSchema(),
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args clickArgs) (*mcp.CallToolResult, any, error) {
		x, y, err := parsePoint("", args.X, args.Y)
		if err != nil {
			return nil, nil, err
		}
		if err := service.Click(control.ClickCommand{X: x, Y: y, Button: args.Button}); err != nil {
			return nil, nil, err
		}
		return okResult("click sent"), nil, nil
	}))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "double_click",
		Description: "Move to a coordinate and double click the specified mouse button",
		InputSchema: clickInputSchema(),
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args clickArgs) (*mcp.CallToolResult, any, error) {
		x, y, err := parsePoint("", args.X, args.Y)
		if err != nil {
			return nil, nil, err
		}
		if err := service.DoubleClick(control.DoubleClickCommand{X: x, Y: y, Button: args.Button}); err != nil {
			return nil, nil, err
		}
		return okResult("double click sent"), nil, nil
	}))

	type pointArgs struct {
		X *apiinput.Number `json:"x"`
		Y *apiinput.Number `json:"y"`
	}
	type dragArgs struct {
		Path []pointArgs `json:"path"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "drag",
		Description: "Drag with the left mouse button along a path of points",
		InputSchema: dragInputSchema(),
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args dragArgs) (*mcp.CallToolResult, any, error) {
		path := make([]control.Point, 0, len(args.Path))
		for i, point := range args.Path {
			x, y, err := parsePoint(fmt.Sprintf("path[%d].", i), point.X, point.Y)
			if err != nil {
				return nil, nil, err
			}
			path = append(path, control.Point{X: x, Y: y})
		}
		if err := service.Drag(control.DragCommand{Path: path}); err != nil {
			return nil, nil, err
		}
		return okResult("drag sent"), nil, nil
	}))

	type moveArgs struct {
		X *apiinput.Number `json:"x"`
		Y *apiinput.Number `json:"y"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "move",
		Description: "Move the remote mouse pointer",
		InputSchema: moveInputSchema(),
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args moveArgs) (*mcp.CallToolResult, any, error) {
		x, y, err := parsePoint("", args.X, args.Y)
		if err != nil {
			return nil, nil, err
		}
		if err := service.Move(control.MoveCommand{X: x, Y: y}); err != nil {
			return nil, nil, err
		}
		return okResult("move sent"), nil, nil
	}))

	type scrollArgs struct {
		X       *apiinput.Number `json:"x"`
		Y       *apiinput.Number `json:"y"`
		ScrollX apiinput.Number  `json:"scrollX"`
		ScrollY apiinput.Number  `json:"scrollY"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scroll",
		Description: "Send horizontal and/or vertical wheel events at a coordinate",
		InputSchema: scrollInputSchema(),
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args scrollArgs) (*mcp.CallToolResult, any, error) {
		x, y, err := parsePoint("", args.X, args.Y)
		if err != nil {
			return nil, nil, err
		}
		scrollX, err := args.ScrollX.Int("scrollX")
		if err != nil {
			return nil, nil, err
		}
		scrollY, err := args.ScrollY.Int("scrollY")
		if err != nil {
			return nil, nil, err
		}
		if err := service.Scroll(control.ScrollCommand{
			X:       x,
			Y:       y,
			ScrollX: scrollX,
			ScrollY: scrollY,
		}); err != nil {
			return nil, nil, err
		}
		return okResult("scroll sent"), nil, nil
	}))

	type keypressArgs struct {
		Key string `json:"key" jsonschema:"Named key or key chord to press and release, such as enter or Ctrl+L"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "keypress",
		Description: "Press and release a named keyboard key or key chord",
	}, requireToolScope(authorizer, authz.ScopeWriteKeyboard, func(ctx context.Context, req *mcp.CallToolRequest, args keypressArgs) (*mcp.CallToolResult, any, error) {
		if err := service.Keypress(control.KeypressCommand{Key: args.Key}); err != nil {
			return nil, nil, err
		}
		return okResult("keypress sent"), nil, nil
	}))

	type typeArgs struct {
		Text string `json:"text" jsonschema:"Text to type into the remote session"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "type",
		Description: "Type text into the remote session",
	}, requireToolScope(authorizer, authz.ScopeWriteKeyboard, func(ctx context.Context, req *mcp.CallToolRequest, args typeArgs) (*mcp.CallToolResult, any, error) {
		if err := service.Type(control.TypeCommand{Text: args.Text}); err != nil {
			return nil, nil, err
		}
		return okResult("text sent"), nil, nil
	}))

	type waitArgs struct {
		MS int `json:"ms" jsonschema:"Milliseconds to wait before returning"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "wait",
		Description: "Sleep for the requested number of milliseconds",
	}, requireToolScope(authorizer, authz.ScopeReadWait, func(ctx context.Context, req *mcp.CallToolRequest, args waitArgs) (*mcp.CallToolResult, any, error) {
		if err := service.Wait(control.WaitCommand{Duration: time.Duration(args.MS) * time.Millisecond}); err != nil {
			return nil, nil, err
		}
		return okResult("wait complete"), nil, nil
	}))

	return server
}

func NewHTTPHandler(service *control.Service, authorizer *authz.Authorizer) http.Handler {
	server := NewServer(service, authorizer)
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)
	return authorizer.MCPMiddleware(handler)
}

func RunStdio(ctx context.Context, service *control.Service) error {
	return NewServer(service, nil).Run(ctx, &mcp.StdioTransport{})
}

func requireToolScope[In, Out any](authorizer *authz.Authorizer, scope authz.Scope, next mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		if err := authorizeTool(authorizer, req, scope); err != nil {
			return nil, zero, err
		}
		return next(ctx, req, args)
	}
}

func authorizeTool(authorizer *authz.Authorizer, req *mcp.CallToolRequest, scope authz.Scope) error {
	if !authorizer.Configured() {
		return nil
	}
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return errors.New("missing bearer token")
	}
	if err := authorizer.AuthorizeScopes(scope, req.Extra.TokenInfo.Scopes); err != nil {
		return err
	}
	return nil
}

func okResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
	}
}

func parsePoint(prefix string, xValue *apiinput.Number, yValue *apiinput.Number) (float64, float64, error) {
	x, err := apiinput.RequiredFloat64(prefix+"x", xValue)
	if err != nil {
		return 0, 0, err
	}
	y, err := apiinput.RequiredFloat64(prefix+"y", yValue)
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func screenshotInputSchema() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"crop": objectSchema(map[string]*jsonschema.Schema{
			"x": numberExpressionSchema("Crop origin X coordinate"),
			"y": numberExpressionSchema("Crop origin Y coordinate"),
			"w": numberExpressionSchema("Crop width"),
			"h": numberExpressionSchema("Crop height"),
		}, nil),
	}, nil)
}

func clickInputSchema() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"x":      numberExpressionSchema("Absolute pointer X coordinate"),
		"y":      numberExpressionSchema("Absolute pointer Y coordinate"),
		"button": &jsonschema.Schema{Type: "string", Description: "Mouse button name such as left, middle, or right"},
	}, []string{"x", "y", "button"})
}

func dragInputSchema() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"path": &jsonschema.Schema{
			Type:        "array",
			Description: "Drag path with at least two points",
			Items: objectSchema(map[string]*jsonschema.Schema{
				"x": numberExpressionSchema("Absolute pointer X coordinate"),
				"y": numberExpressionSchema("Absolute pointer Y coordinate"),
			}, []string{"x", "y"}),
		},
	}, []string{"path"})
}

func moveInputSchema() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"x": numberExpressionSchema("Absolute pointer X coordinate"),
		"y": numberExpressionSchema("Absolute pointer Y coordinate"),
	}, []string{"x", "y"})
}

func scrollInputSchema() *jsonschema.Schema {
	return objectSchema(map[string]*jsonschema.Schema{
		"x":       numberExpressionSchema("Absolute pointer X coordinate"),
		"y":       numberExpressionSchema("Absolute pointer Y coordinate"),
		"scrollX": numberExpressionSchema("Horizontal wheel delta in multiples of 120"),
		"scrollY": numberExpressionSchema("Vertical wheel delta in multiples of 120"),
	}, []string{"x", "y"})
}

func numberExpressionSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: description + "; accepts a JSON number or arithmetic expression string",
		AnyOf: []*jsonschema.Schema{
			{Type: "number"},
			{Type: "string"},
		},
	}
}

func objectSchema(properties map[string]*jsonschema.Schema, required []string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}
