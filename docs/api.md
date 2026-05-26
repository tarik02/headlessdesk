# HTTP API

The `serve` subcommand always mounts:

- `GET /healthz` for backend state and framebuffer size.

The REST API can be enabled or disabled with `--enable-http-api`.
When enabled, it mounts:

- `GET /screenshot` or `POST /screenshot` returns the latest framebuffer as `image/png`, with optional cropping.
- `POST /click` moves to a coordinate and clicks a mouse button.
- `POST /double_click` moves to a coordinate and double clicks a mouse button.
- `POST /drag` drags with the left mouse button along a path of points.
- `POST /move` moves the remote pointer to an absolute position.
- `POST /scroll` sends horizontal and/or vertical wheel events.
- `POST /keypress` presses and releases a named key.
- `POST /type` types text into the remote session.

If `server.auth.tokens` is non-empty, these REST endpoints require
`Authorization: Bearer <token>` with an `http` audience. `/healthz` remains
unauthenticated. REST scopes are `read:screenshot`, `write:mouse`, and
`write:keyboard`; `read:*`, `write:*`, and `*` wildcards are supported.

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
curl -X POST http://127.0.0.1:4243/type \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello from http"}'

curl -X POST http://127.0.0.1:4243/keypress \
  -H 'Content-Type: application/json' \
  -d '{"key":"enter"}'

curl -X POST http://127.0.0.1:4243/move \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360}'

curl -X POST http://127.0.0.1:4243/click \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360,"button":"left"}'

curl -X POST http://127.0.0.1:4243/scroll \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360,"scrollY":120}'

curl -X POST http://127.0.0.1:4243/screenshot \
  -H 'Content-Type: application/json' \
  -d '{"crop":{"x":100,"y":100,"w":400,"h":300}}' \
  --output cropped.png
```
