# headlessrdp

Minimal Go remote-desktop control server.

The current implementation supports RDP through FreeRDP and VNC/RFB through
LibVNCClient. The HTTP, REST, and MCP layers use a shared protocol-neutral
desktop abstraction.

## What this POC does

The server keeps a headless remote desktop session alive, copies the latest
decoded framebuffer into memory, and exposes that image and input controls over
HTTP and MCP.

Supported protocol status:

- `rdp`: implemented through `internal/freerdp`.
- `vnc`: implemented through LibVNCClient with framebuffer updates, screenshots,
  keyboard, pointer, and wheel input.

## Requirements

For RDP, you need FreeRDP development files visible to `pkg-config`.
This POC currently expects the pkg-config package names used by FreeRDP 3:

- `freerdp3`
- `freerdp-client3`
- `winpr3`

On Debian or Ubuntu this usually means installing FreeRDP 3 development packages
or building FreeRDP 3 yourself.

For VNC, you also need LibVNCClient development files visible to `pkg-config`:

- `libvncclient`

## Configuration model

The CLI and environment variables now use protocol-neutral session settings plus
protocol-specific sections. There is no backward compatibility with the old
`RDP_*` environment variable layout.

Configuration keys:

```yaml
server:
  listen_addr: ":8080"
  mcp_path: "/mcp"
  enable_http_api: true
  enable_mcp_api: true
session:
  protocol: "rdp" # rdp or vnc
  host: "127.0.0.1"
  port: 0         # 0 means protocol default: 3389 for RDP, 5900 for VNC
  username: "gmtest"
  password: "gmtest"
  width: 1280
  height: 720
  insecure: true
rdp:
  domain: ""
  keyboard_layout: 1033
  graphics_mode: "auto"
vnc:
  shared: true
  view_only: false
```

Environment variables use the `HEADLESSRDP_` prefix and flatten nested keys with
underscores, for example:

- `HEADLESSRDP_SESSION_PROTOCOL=rdp`
- `HEADLESSRDP_SESSION_HOST=127.0.0.1`
- `HEADLESSRDP_SESSION_USERNAME=gmtest`
- `HEADLESSRDP_SESSION_PASSWORD=gmtest`
- `HEADLESSRDP_SERVER_LISTEN_ADDR=:8080`
- `HEADLESSRDP_RDP_GRAPHICS_MODE=bitmap`
- `HEADLESSRDP_VNC_SHARED=true`

## HTTP server

Run an RDP-backed server:

```bash
go run ./cmd/server serve \
  --listen-addr :8080 \
  --protocol rdp \
  --remote-host 127.0.0.1 \
  --remote-port 3391 \
  --username gmtest \
  --password gmtest \
  --width 1280 \
  --height 720 \
  --insecure
```

Run a VNC-backed server using the same public APIs:

```bash
go run ./cmd/server serve \
  --listen-addr :8080 \
  --protocol vnc \
  --remote-host 127.0.0.1 \
  --remote-port 5900 \
  --password secret \
  --width 1280 \
  --height 720 \
  --vnc-shared
```

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

## stdio MCP server

Run:

```bash
go run ./cmd/server stdio-mcp \
  --protocol rdp \
  --remote-host 127.0.0.1 \
  --remote-port 3391 \
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

## VNC backend

The VNC backend uses LibVNCClient for RFB negotiation, server-message parsing,
framebuffer update requests, and keyboard/pointer input.

Current VNC behavior:

- connects to `session.host:session.port`, defaulting to port `5900`;
- uses `session.password` for VNC password authentication while still allowing servers that offer no-auth;
- maps `vnc.shared` to the RFB shared/exclusive connection flag;
- maintains an in-memory framebuffer from server framebuffer updates;
- sends key events with X11/RFB keysyms, including common named keys and Latin-1 text input;
- sends pointer movement, button, and wheel events;
- rejects `vnc.view_only=true` because the public control APIs require input support.

Possible VNC improvements:

1. Add fake RFB server tests that cover handshake, framebuffer, and input-event paths without requiring an external VNC service.
2. Add richer status metadata for the negotiated RFB protocol/security type if LibVNCClient exposes it.

## Notes

- `--insecure` accepts unknown or changed certificates where the selected
  protocol supports certificate validation.
- Without `--insecure`, certificate validation stays strict for RDP.
- The persistent session connects once, keeps the latest framebuffer, and exposes
  basic keyboard and mouse input over HTTP/MCP.
