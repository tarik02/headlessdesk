---
name: headlessdesk
description: >
  Headlessdesk remote desktop control. Use when interacting with an existing
  headlessdesk HTTP API, MCP tools, or FUSE mount for screenshots, health,
  pointer input, keyboard input, or desktop automation.
---

Headlessdesk exposes one desktop control service through several surfaces.
Route to the surface the user or environment gives you.

## Route

- Readiness, dimensions, scopes: [setup.md](setup.md).
- REST/curl/HTTP API: [api.md](api.md).
- MCP tools/client: [mcp.md](mcp.md).
- FUSE mount/filesystem access: [fuse.md](fuse.md).
- Coordinates, expressions, screenshots, input: [common.md](common.md).

Only this router names surfaces. Domain files own their own command shapes and
nuances. Shared files stay surface-neutral.
