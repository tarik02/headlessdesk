# Configuration

The CLI and environment variables use protocol-neutral session settings plus
protocol-specific sections.

## Config File

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

## Environment Variables

Environment variables use the `HEADLESSDESK_` prefix and flatten nested keys with
underscores:

```bash
HEADLESSDESK_SESSION_PROTOCOL=rdp
HEADLESSDESK_SESSION_HOST=127.0.0.1
HEADLESSDESK_SESSION_USERNAME=gmtest
HEADLESSDESK_SESSION_PASSWORD=gmtest
HEADLESSDESK_SERVER_LISTEN_ADDR=:8080
HEADLESSDESK_RDP_GRAPHICS_MODE=bitmap
HEADLESSDESK_VNC_SHARED=true
```

## Requirements

For RDP, FreeRDP development files must be visible to `pkg-config`:

- `freerdp3`
- `freerdp-client3`
- `winpr3`

For VNC, LibVNCClient development files must be visible to `pkg-config`:

- `libvncclient`
