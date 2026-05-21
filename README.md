# headlessdesk

Minimal headless remote-desktop control server.

`headlessdesk` connects to RDP, VNC, command-backed, or KWin desktop backends, keeps a
framebuffer in memory when the output backend provides one, and exposes
screenshots plus keyboard/mouse input through HTTP and MCP.
Command backends can run locally or over one persistent SSH client connection.

## Quick Start

Run an RDP-backed HTTP server:

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

Capture a screenshot:

```bash
curl -X POST http://127.0.0.1:8080/screenshot --output screenshot.png
```

## Documentation

- [installation and builds](docs/builds.md)
- [configuration](docs/configuration.md)
- [HTTP API](docs/api.md)
- [MCP API](docs/mcp.md)
- [FUSE filesystem](docs/fuse.md)
- [architecture and backends](docs/architecture.md)
