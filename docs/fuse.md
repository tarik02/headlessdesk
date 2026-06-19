# FUSE Filesystem

The `mount` subcommand exposes the control service through a FUSE filesystem.
This is Linux-only.

```bash
headlessdesk mount --config ./server.yaml
cat "$XDG_RUNTIME_DIR/headlessdesk/health.json"
cp "$XDG_RUNTIME_DIR/headlessdesk/screenshot.png" ./screenshot.png
cp "$XDG_RUNTIME_DIR/headlessdesk/crop/100,100,400,300.png" ./cropped.png
cp "$XDG_RUNTIME_DIR/headlessdesk/crop/1280%2F2-200,720%2F2-150,400,300.png" ./center-crop.png
printf 'hello from fuse' > "$XDG_RUNTIME_DIR/headlessdesk/input/type"
printf '{"x":"1280/2","y":"720/2","button":"left"}' > "$XDG_RUNTIME_DIR/headlessdesk/input/click.json"
fusermount3 -u "$XDG_RUNTIME_DIR/headlessdesk"
```

Without an explicit mountpoint, `mount` uses `$XDG_RUNTIME_DIR/headlessdesk`
when available, then platform runtime/cache/temp fallbacks.

## Files

- `README.md`: filesystem usage summary.
- `health.json`: current backend status as JSON.
- `status.json`: same as `health.json`.
- `screenshot.png`: latest full screenshot as PNG.
- `crop/<x>,<y>,<w>,<h>.png`: cropped screenshot as PNG. Each part may be an integer or an arithmetic expression.
- `input/type`: raw text to type.
- `input/keypress`: key name to press and release.
- `input/click.json`: `{"x":"1280/2","y":"720/2","button":"left"}`.
- `input/double_click.json`: same shape as `click.json`.
- `input/move.json`: `{"x":"1280/2","y":"720/2"}`.
- `input/scroll.json`: `{"x":"1280/2","y":"720/2","scrollY":"120*2"}`.
- `input/drag.json`: `{"path":[{"x":"100+20","y":100},{"x":300,"y":"200-50"}]}`.

Read files capture fresh service output on open. Input files collect write
chunks per open file handle and execute the corresponding service command on
close, so normal shell redirection works with JSON payloads.

Pointer coordinate fields accept JSON numbers or arithmetic expression strings
using numeric literals, parentheses, `+`, `-`, `*`, and `/`. Crop coordinates
and scroll deltas use the same expression syntax, then round to integers.

Crop filenames use one path segment, so escape operators that cannot appear
literally in a path segment. For example, use `%2F` for `/`:

```bash
cp "$XDG_RUNTIME_DIR/headlessdesk/crop/1280%2F2-200,720%2F2-150,400,300.png" ./cropped.png
cp "$XDG_RUNTIME_DIR/headlessdesk/crop/100*2,100,400,300.png" ./wide-crop.png
```
