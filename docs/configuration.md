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

## RDP Graphics and Acceleration

`rdp.graphics_mode` controls the FreeRDP graphics path:

- `auto`: try AVC/H.264 first, then graphics pipeline without H.264, then bitmap;
- `avc` or `h264`: require the graphics pipeline with H.264;
- `gfx` or `graphics`: require the graphics pipeline without H.264;
- `bitmap` or `legacy`: require classic bitmap updates.

Some RDP servers only support the graphics pipeline. For example, KRDP requires
graphics pipeline support, so `bitmap` mode will not connect there.

When FreeRDP uses VAAPI for H.264 decode on NVIDIA, the VAAPI driver may need to
be selected explicitly:

```bash
LIBVA_DRIVER_NAME=nvidia
LIBVA_DRIVERS_PATH=/opt/libva-nvidia-driver-git/lib/dri
NVD_BACKEND=direct
```

The `LIBVA_DRIVERS_PATH` value is distribution-specific. Use the directory that
contains `nvidia_drv_video.so` on the target system.

Client-side hardware decode does not imply server-side hardware encode. KRDP can
still consume significant CPU on NVIDIA systems because its H.264 encoder falls
back to software encoding when VAAPI encoding is unavailable. Lowering KRDP's
video quality or adding a systemd CPU quota can limit the impact, but it does
not remove the underlying software encoding cost.

## Requirements

For RDP, FreeRDP development files must be visible to `pkg-config`:

- `freerdp3`
- `freerdp-client3`
- `winpr3`

For VNC, LibVNCClient development files must be visible to `pkg-config`:

- `libvncclient`
