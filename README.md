# libfreerdp-golang-poc

Minimal Go-to-FreeRDP proof of concept.

It now does three things:

- initializes a Go module,
- exposes a small binding in `internal/freerdp`,
- keeps a persistent RDP session open and exposes health, REST control, and MCP interfaces through `cmd/server`.

## What this POC does

The binding has two modes:

- auth-only probing through `internal/freerdp.Probe`,
- persistent session capture with `cmd/server serve`,
- MCP tool access through `cmd/server serve` or `cmd/server stdio-mcp`.

The auth-only probe does not open a window or render a desktop. A successful probe means:

- TCP/TLS/NLA negotiation completed,
- credentials were accepted,
- FreeRDP reported the connection attempt as successful.

The persistent server keeps a headless RDP session alive, copies the latest decoded framebuffer into memory, and exposes that image through an HTTP endpoint.

## Requirements

You need FreeRDP development files visible to `pkg-config`.
This POC currently expects the pkg-config package names used by FreeRDP 3:

- `freerdp3`
- `winpr3`

On Debian or Ubuntu this usually means installing FreeRDP 3 development packages or building FreeRDP 3 yourself.

## HTTP server

Run:

```bash
go run ./cmd/server serve \
  --listen-addr :8080 \
  --host 127.0.0.1 \
  --port 3391 \
  --username gmtest \
  --password gmtest \
  --width 1280 \
  --height 720 \
  --insecure
```

The server command uses Cobra and Viper for parsing. You can provide settings through:

- command-line flags such as `--host` and `--listen-addr`
- environment variables such as `RDP_HOST`, `RDP_LISTEN_ADDR`, `RDP_USERNAME`, and `RDP_PASSWORD`
- `--config /path/to/server.yaml` with YAML, TOML, or JSON

The `serve` subcommand always mounts:

- `GET /healthz` for session state and framebuffer size

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

Example health response:

```json
{
  "connected": true,
  "active": true,
  "ready": true,
  "state": "CONNECTION_STATE_ACTIVE",
  "freerdp": "3.24.2",
  "width": 1280,
  "height": 720
}
```

If the connection has started but no full frame has been decoded yet, `/healthz` can briefly report `CONNECTION_STATE_NEGO` or `ready=false`, and `/screenshot` returns `503` until a snapshot becomes available.

Example input requests:

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

Example toggles:

```bash
go run ./cmd/server serve \
  --listen-addr :8080 \
  --enable-http-api=false \
  --enable-mcp-api=true \
  --host 127.0.0.1 \
  --port 3391 \
  --username gmtest \
  --password gmtest \
  --insecure
```

## stdio MCP server

Run:

```bash
go run ./cmd/server stdio-mcp \
  --host 127.0.0.1 \
  --port 3391 \
  --username gmtest \
  --password gmtest \
  --width 1280 \
  --height 720 \
  --insecure
```

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

## Notes

- `--insecure` accepts unknown or changed certificates through the FreeRDP certificate callbacks.
- without `--insecure`, certificate validation stays strict.
- the persistent session is intentionally minimal: it connects once, keeps the latest framebuffer, and exposes basic keyboard and mouse input over HTTP.
- this is intentionally small and not yet a general-purpose wrapper.
