# MCP API

`headlessdesk` exposes MCP over two transports:

- streamable HTTP through `headlessdesk serve`;
- stdio through `headlessdesk stdio-mcp`.

The streamable HTTP API can be enabled or disabled with `--enable-mcp-api`.
When enabled, it mounts at `--mcp-path`, which defaults to `/mcp`.
If `server.auth.tokens` is non-empty, MCP-over-HTTP requires
`Authorization: Bearer <token>` with an `mcp` audience. Bearer auth does not
apply to stdio MCP.

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

MCP tool scopes are `read:status` for `session_status`, `read:screenshot` for
`screenshot`, `read:wait` for `wait`, `write:mouse` for pointer/button/wheel
tools, and `write:keyboard` for keypress/text tools. `read:*`, `write:*`, and
`*` wildcards are supported.

Input coordinates are accepted in output screenshot space. Backends that expose
a different input coordinate space, such as KWin EIS on scaled Wayland desktops,
can translate those coordinates before sending input.
