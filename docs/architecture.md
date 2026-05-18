# Architecture and Backends

`headlessdesk` connects to a remote desktop session, keeps the latest decoded
framebuffer in memory, and exposes screenshots plus keyboard/mouse input through
HTTP and MCP.

The HTTP, REST, and MCP layers use a shared protocol-neutral desktop abstraction.

Supported protocols:

- `rdp`: implemented through `internal/freerdp` and FreeRDP.
- `vnc`: implemented through `internal/vnc` and LibVNCClient.

## RDP Backend

The RDP backend uses FreeRDP 3 through cgo.

`--insecure` accepts unknown or changed certificates for RDP. Without
`--insecure`, certificate validation stays strict.

## VNC Backend

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
