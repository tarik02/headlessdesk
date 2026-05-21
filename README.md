# headlessdesk

Minimal headless remote-desktop control server.

`headlessdesk` connects to RDP, VNC, command-backed, or KWin desktop backends, keeps a
framebuffer in memory when the output backend provides one, and exposes
screenshots plus keyboard/mouse input through HTTP and MCP.
Command backends can run locally or over one persistent SSH client connection.

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
  --backend-type rdp \
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
  --backend-type vnc \
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
  --backend-type rdp \
  --remote-host 127.0.0.1 \
  --remote-port 3391 \
  --username gmtest \
  --password gmtest \
  --width 1280 \
  --height 720 \
  --insecure
```

## Configuration

See [config.example.yaml](config.example.yaml) for a commented config with all
backend options. Minimal `server.yaml`:

```yaml
server:
  listen_addr: ":8080"
  mcp_path: "/mcp"
  enable_http_api: true
  enable_mcp_api: true
input: "desktop"
output: "desktop"
backends:
  desktop:
    type: "rdp"
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
```

Preset-composed local command backend:

```yaml
input: "local"
output: "local"
backends:
  local:
    extends:
      - preset:command-base
      - preset:screenshot-spectacle
      - preset:input-ydotool
```

KDE/Wayland local setup can use KWin for screenshots and EIS/libei for input:

```yaml
input: "local-input"
output: "local-screen"
backends:
  local-screen:
    type: "kwin"
  local-input:
    type: "eis"
```

Environment variables use the `HEADLESSDESK_` prefix:

```bash
HEADLESSDESK_INPUT=default
HEADLESSDESK_OUTPUT=default
HEADLESSDESK_BACKENDS_DEFAULT_TYPE=rdp
HEADLESSDESK_BACKENDS_DEFAULT_HOST=127.0.0.1
HEADLESSDESK_BACKENDS_DEFAULT_USERNAME=gmtest
HEADLESSDESK_BACKENDS_DEFAULT_PASSWORD=gmtest
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
