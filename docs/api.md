# HTTP and MCP API

The `serve` subcommand always mounts:

- `GET /healthz` for session state and framebuffer size.

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
- `POST /wait` sleeps for a given number of milliseconds.

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
  "height": 720
}
```

If the connection has started but no full frame has been decoded yet, `/healthz`
can briefly report a negotiation state or `ready=false`, and `/screenshot`
returns `503` until a snapshot becomes available.

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
