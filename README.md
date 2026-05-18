# headlessdesk

Minimal headless remote-desktop control server.

`headlessdesk` connects to an RDP or VNC desktop, keeps a framebuffer in memory,
and exposes screenshots plus keyboard/mouse input through HTTP and MCP.

## Installation

Build with Nix:

```bash
nix build .#
```

Run from the flake:

```bash
nix run .# -- serve --config ./server.yaml
```

Or build with Go when FreeRDP and LibVNCClient development files are available
through `pkg-config`:

```bash
go build ./cmd/headlessdesk
```

## Usage

Run an RDP-backed server:

```bash
headlessdesk serve \
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

Run a VNC-backed server:

```bash
headlessdesk serve \
  --listen-addr :8080 \
  --protocol vnc \
  --remote-host 127.0.0.1 \
  --remote-port 5900 \
  --password secret \
  --width 1280 \
  --height 720 \
  --vnc-shared
```

Run stdio MCP:

```bash
headlessdesk stdio-mcp \
  --protocol rdp \
  --remote-host 127.0.0.1 \
  --remote-port 3391 \
  --username gmtest \
  --password gmtest \
  --width 1280 \
  --height 720 \
  --insecure
```

## Configuration

Example `server.yaml`:

```yaml
server:
  listen_addr: ":8080"
  mcp_path: "/mcp"
  enable_http_api: true
  enable_mcp_api: true
session:
  protocol: "rdp"
  host: "127.0.0.1"
  port: 3391
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

Environment variables use the `HEADLESSDESK_` prefix:

```bash
HEADLESSDESK_SESSION_PROTOCOL=rdp
HEADLESSDESK_SESSION_HOST=127.0.0.1
HEADLESSDESK_SESSION_USERNAME=gmtest
HEADLESSDESK_SESSION_PASSWORD=gmtest
HEADLESSDESK_SERVER_LISTEN_ADDR=:8080
```

## HTTP Examples

```bash
curl -X POST http://127.0.0.1:8080/type \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello from http"}'

curl -X POST http://127.0.0.1:8080/click \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360,"button":"left"}'

curl -X POST http://127.0.0.1:8080/screenshot \
  -H 'Content-Type: application/json' \
  -d '{"crop":{"x":100,"y":100,"w":400,"h":300}}' \
  --output cropped.png
```

More details:

- [configuration](docs/configuration.md)
- [http and mcp api](docs/api.md)
- [architecture and backends](docs/architecture.md)
