# MCP

Use this when the agent has headlessdesk MCP tools or when wiring an MCP client.
Load [common.md](common.md) for shared coordinate and input behavior.

## Tools

Session status:

```json
{"tool":"session_status","arguments":{}}
```

Cropped screenshot:

```json
{"tool":"screenshot","arguments":{"crop":{"x":"1280/2-200","y":"720/2-150","w":400,"h":300}}}
```

Crop object fields accept expression strings.

Move pointer:

```json
{"tool":"move","arguments":{"x":"1280/2","y":"720/2"}}
```

Pointer coordinates accept expression strings.

Click:

```json
{"tool":"click","arguments":{"x":"1280/2","y":"720/2","button":"left"}}
```

Double click:

```json
{"tool":"double_click","arguments":{"x":"1280/2","y":"720/2","button":"left"}}
```

Drag:

```json
{"tool":"drag","arguments":{"path":[{"x":"1280/2-200","y":"720/2-100"},{"x":"1280/2+200","y":"720/2+100"}]}}
```

Drag path coordinates accept expression strings.

Scroll:

```json
{"tool":"scroll","arguments":{"x":640,"y":360,"scrollY":"120*3"}}
```

Scroll deltas accept expression strings and round to integers.

Keypress:

```json
{"tool":"keypress","arguments":{"key":"Ctrl+L"}}
```

Type text:

```json
{"tool":"type","arguments":{"text":"hello from mcp"}}
```

Wait:

```json
{"tool":"wait","arguments":{"ms":500}}
```
