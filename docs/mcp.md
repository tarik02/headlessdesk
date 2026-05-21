# MCP API

`headlessdesk` exposes MCP over two transports:

- streamable HTTP through `headlessdesk serve`;
- stdio through `headlessdesk stdio-mcp`.

The streamable HTTP API can be enabled or disabled with `--enable-mcp-api`.
When enabled, it mounts at `--mcp-path`, which defaults to `/mcp`.

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

Input coordinates are accepted in output screenshot space. Backends that expose
a different input coordinate space, such as KWin EIS on scaled Wayland desktops,
can translate those coordinates before sending input.
