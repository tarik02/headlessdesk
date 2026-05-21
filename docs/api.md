# HTTP and MCP API

The `serve` subcommand always mounts:

- `GET /healthz` for backend state and framebuffer size.

The REST API can be enabled or disabled with `--enable-http-api`.
When enabled, it mounts:

- `POST /screenshot` returns the latest framebuffer as `image/png`, with optional cropping.
- `POST /click` moves to a coordinate and clicks a mouse button.
- `POST /double_click` moves to a coordinate and double clicks a mouse button.
- `POST /drag` drags with the left mouse button along a path of points.
- `POST /move` moves the remote pointer to an absolute position.
- `POST /scroll` sends horizontal and/or vertical wheel events.
- `POST /keypress` presses and releases a named key.
- `POST /type` types text into the remote session.

The MCP streamable HTTP API can be enabled or disabled with `--enable-mcp-api`.
When enabled, it mounts at `--mcp-path`, which defaults to `/mcp`.

The HTTP and stdio MCP transports expose the same tool set:

- `session_status`
- `screenshot`
- `click`
- `double_click`
- `drag`
- `move`
- `scroll`
- `keypress`
- `type`
- `wait`

The `mount` subcommand exposes the same control service through a FUSE
filesystem:

- `health.json` and `status.json` read current backend status as JSON.
- `screenshot.png` reads the latest framebuffer as PNG.
- `crop/<x>,<y>,<w>,<h>.png` reads a cropped screenshot as PNG.
- `input/type` accepts raw text to type.
- `input/keypress` accepts a key name.
- `input/click.json` accepts `{"x":640,"y":360,"button":"left"}`.
- `input/double_click.json` accepts the same shape as `click.json`.
- `input/move.json` accepts `{"x":640,"y":360}`.
- `input/scroll.json` accepts `{"x":640,"y":360,"scrollY":120}`.
- `input/drag.json` accepts `{"path":[{"x":1,"y":1},{"x":2,"y":2}]}`.

Example health response:

```json
{
  "protocol": "rdp",
  "connected": true,
  "active": true,
  "ready": true,
  "state": "CONNECTION_STATE_ACTIVE",
  "version": "3.24.2",
  "width": 1280,
  "height": 720,
  "input_backend": "vnc-control",
  "output_backend": "rdp-visual",
  "input_protocol": "vnc",
  "output_protocol": "rdp",
  "input_ready": true,
  "output_ready": true,
  "input_width": 1280,
  "input_height": 720,
  "output_width": 1280,
  "output_height": 720
}
```

Top-level health fields describe the output backend. `ready` means the selected
output backend has a screenshot available. Input and output backend names,
protocols, ready flags, dimensions, regions, and errors are included as
additive fields when available. `/healthz` returns `200` only when both
`input_ready` and `output_ready` are true; otherwise it returns `503`.

Input coordinates are accepted in output screenshot space. Backends that expose
a different input coordinate space, such as KWin EIS on scaled Wayland desktops,
can translate those coordinates before sending input.

## Examples

```bash
curl -X POST http://127.0.0.1:8080/type \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello from http"}'

curl -X POST http://127.0.0.1:8080/keypress \
  -H 'Content-Type: application/json' \
  -d '{"key":"enter"}'

curl -X POST http://127.0.0.1:8080/move \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360}'

curl -X POST http://127.0.0.1:8080/click \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360,"button":"left"}'

curl -X POST http://127.0.0.1:8080/scroll \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360,"scrollY":120}'

curl -X POST http://127.0.0.1:8080/screenshot \
  -H 'Content-Type: application/json' \
  -d '{"crop":{"x":100,"y":100,"w":400,"h":300}}' \
  --output cropped.png
```
