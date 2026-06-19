# HTTP API

Use this when interacting through `headlessdesk serve` with REST endpoints.
Load [common.md](common.md) for shared coordinate and input behavior.

Assume:

```bash
HD_URL="${HD_URL:-http://127.0.0.1:4243}"
HD_AUTH=()
# If auth is configured:
# HD_AUTH=(-H "Authorization: Bearer $HD_TOKEN")
```

## Methods

Health:

```bash
curl -fsS "${HD_AUTH[@]}" "$HD_URL/healthz"
```

Bearer auth uses the `http` audience.

Cropped screenshot:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"crop":{"x":"1280/2-200","y":"720/2-150","w":400,"h":300}}' \
  "$HD_URL/screenshot" \
  --output headlessdesk-crop.png
```

Crop object fields accept expression strings.

Full screenshot, only when needed:

```bash
curl -fsS "${HD_AUTH[@]}" "$HD_URL/screenshot" \
  --output headlessdesk-screenshot.png
```

Full screenshot with `POST`, only when a single verb is easier for tooling:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" "$HD_URL/screenshot" \
  --output headlessdesk-screenshot.png
```

Move pointer:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"x":"1280/2","y":"720/2"}' \
  "$HD_URL/move"
```

Pointer coordinates accept expression strings.

Click:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"x":"1280/2","y":"720/2","button":"left"}' \
  "$HD_URL/click"
```

Double click:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"x":"1280/2","y":"720/2","button":"left"}' \
  "$HD_URL/double_click"
```

Drag:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"path":[{"x":"1280/2-200","y":"720/2-100"},{"x":"1280/2+200","y":"720/2+100"}]}' \
  "$HD_URL/drag"
```

Drag path coordinates accept expression strings.

Scroll:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"x":640,"y":360,"scrollY":"120*3"}' \
  "$HD_URL/scroll"
```

Scroll deltas accept expression strings and round to integers.

Keypress:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"key":"Ctrl+L"}' \
  "$HD_URL/keypress"
```

Type text:

```bash
curl -fsS -X POST "${HD_AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello from http"}' \
  "$HD_URL/type"
```

## Responses

Input methods return `{"ok":true}`. Screenshot methods return `image/png`.
Bad requests return JSON with `error`. Backend unavailable responses include
`error` and current `status`.
