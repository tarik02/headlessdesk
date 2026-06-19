# FUSE

Use this when headlessdesk is mounted as a filesystem. FUSE is Linux-only.
Load [common.md](common.md) for shared coordinate and input behavior.

Assume:

```bash
HD_MOUNT="${HD_MOUNT:-${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/headlessdesk}"
```

Read screenshots and JSON directly from `$HD_MOUNT`. Do not copy FUSE files to
`/tmp` or another staging directory before inspecting them.

## Files

Filesystem summary:

```bash
sed -n '1,120p' "$HD_MOUNT/README.md"
```

Health/status:

```bash
sed -n '1,120p' "$HD_MOUNT/health.json"
sed -n '1,120p' "$HD_MOUNT/status.json"
```

Cropped screenshot:

```bash
file "$HD_MOUNT/crop/100,100,400,300.png"
```

Cropped screenshot with expressions:

```bash
file "$HD_MOUNT/crop/1280%2F2-200,720%2F2-150,400,300.png"
```

Crop filename parts accept expression strings. Use percent-encoding for `/`
inside one path part.

Pass the crop path itself to image inspection tools:

```text
$HD_MOUNT/crop/1280%2F2-200,720%2F2-150,400,300.png
```

Full screenshot, only when needed:

```bash
file "$HD_MOUNT/screenshot.png"
```

Type text:

```bash
printf 'hello from fuse' > "$HD_MOUNT/input/type"
```

Keypress:

```bash
printf 'Ctrl+L' > "$HD_MOUNT/input/keypress"
```

Move pointer:

```bash
printf '{"x":"1280/2","y":"720/2"}' > "$HD_MOUNT/input/move.json"
```

Click:

```bash
printf '{"x":"1280/2","y":"720/2","button":"left"}' > "$HD_MOUNT/input/click.json"
```

Double click:

```bash
printf '{"x":"1280/2","y":"720/2","button":"left"}' > "$HD_MOUNT/input/double_click.json"
```

Drag:

```bash
printf '{"path":[{"x":"1280/2-200","y":"720/2-100"},{"x":"1280/2+200","y":"720/2+100"}]}' > "$HD_MOUNT/input/drag.json"
```

Scroll:

```bash
printf '{"x":640,"y":360,"scrollY":"120*3"}' > "$HD_MOUNT/input/scroll.json"
```

Read files capture fresh service output on open. Input files buffer one open
handle and execute when it closes, so shell redirection is enough.
