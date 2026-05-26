package mcpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
		X *int `json:"x" jsonschema:"Crop origin X coordinate"`
		Y *int `json:"y" jsonschema:"Crop origin Y coordinate"`
		W *int `json:"w" jsonschema:"Crop width"`
		H *int `json:"h" jsonschema:"Crop height"`
	}
	type screenshotArgs struct {
		Crop *cropArgs `json:"crop" jsonschema:"Optional screenshot crop rectangle"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "screenshot",
		Description: "Capture the latest remote desktop framebuffer as a PNG image, optionally cropped",
	}, requireToolScope(authorizer, authz.ScopeReadScreenshot, func(ctx context.Context, req *mcp.CallToolRequest, args screenshotArgs) (*mcp.CallToolResult, any, error) {
		var crop *control.Crop
		if args.Crop != nil {
			crop = &control.Crop{X: args.Crop.X, Y: args.Crop.Y, W: args.Crop.W, H: args.Crop.H}
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
		X      int    `json:"x" jsonschema:"Absolute pointer X coordinate"`
		Y      int    `json:"y" jsonschema:"Absolute pointer Y coordinate"`
		Button string `json:"button" jsonschema:"Mouse button name such as left, middle, or right"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "click",
		Description: "Move to a coordinate and click the specified mouse button",
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args clickArgs) (*mcp.CallToolResult, any, error) {
		if err := service.Click(control.ClickCommand{X: args.X, Y: args.Y, Button: args.Button}); err != nil {
			return nil, nil, err
		}
		return okResult("click sent"), nil, nil
	}))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "double_click",
		Description: "Move to a coordinate and double click the specified mouse button",
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args clickArgs) (*mcp.CallToolResult, any, error) {
		if err := service.DoubleClick(control.DoubleClickCommand{X: args.X, Y: args.Y, Button: args.Button}); err != nil {
			return nil, nil, err
		}
		return okResult("double click sent"), nil, nil
	}))

	type pointArgs struct {
		X int `json:"x" jsonschema:"Absolute pointer X coordinate"`
		Y int `json:"y" jsonschema:"Absolute pointer Y coordinate"`
	}
	type dragArgs struct {
		Path []pointArgs `json:"path" jsonschema:"Drag path with at least two points"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "drag",
		Description: "Drag with the left mouse button along a path of points",
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args dragArgs) (*mcp.CallToolResult, any, error) {
		path := make([]control.Point, 0, len(args.Path))
		for _, point := range args.Path {
			path = append(path, control.Point{X: point.X, Y: point.Y})
		}
		if err := service.Drag(control.DragCommand{Path: path}); err != nil {
			return nil, nil, err
		}
		return okResult("drag sent"), nil, nil
	}))

	type moveArgs struct {
		X int `json:"x" jsonschema:"Absolute pointer X coordinate"`
		Y int `json:"y" jsonschema:"Absolute pointer Y coordinate"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "move",
		Description: "Move the remote mouse pointer",
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args moveArgs) (*mcp.CallToolResult, any, error) {
		if err := service.Move(control.MoveCommand{X: args.X, Y: args.Y}); err != nil {
			return nil, nil, err
		}
		return okResult("move sent"), nil, nil
	}))

	type scrollArgs struct {
		X       int `json:"x" jsonschema:"Absolute pointer X coordinate"`
		Y       int `json:"y" jsonschema:"Absolute pointer Y coordinate"`
		ScrollX int `json:"scrollX" jsonschema:"Horizontal wheel delta in multiples of 120"`
		ScrollY int `json:"scrollY" jsonschema:"Vertical wheel delta in multiples of 120"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "scroll",
		Description: "Send horizontal and/or vertical wheel events at a coordinate",
	}, requireToolScope(authorizer, authz.ScopeWriteMouse, func(ctx context.Context, req *mcp.CallToolRequest, args scrollArgs) (*mcp.CallToolResult, any, error) {
		if err := service.Scroll(control.ScrollCommand{
			X:       args.X,
			Y:       args.Y,
			ScrollX: args.ScrollX,
			ScrollY: args.ScrollY,
		}); err != nil {
			return nil, nil, err
		}
		return okResult("scroll sent"), nil, nil
	}))

	type keypressArgs struct {
		Key string `json:"key" jsonschema:"Named key to press and release"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "keypress",
		Description: "Press and release a named keyboard key",
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
