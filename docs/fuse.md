# FUSE Filesystem

The `mount` subcommand exposes the control service through a FUSE filesystem.
This is Linux-only.

```bash
headlessdesk mount --config ./server.yaml
cat "$XDG_RUNTIME_DIR/headlessdesk/health.json"
cp "$XDG_RUNTIME_DIR/headlessdesk/screenshot.png" ./screenshot.png
cp "$XDG_RUNTIME_DIR/headlessdesk/crop/100,100,400,300.png" ./cropped.png
printf 'hello from fuse' > "$XDG_RUNTIME_DIR/headlessdesk/input/type"
printf '{"x":640,"y":360,"button":"left"}' > "$XDG_RUNTIME_DIR/headlessdesk/input/click.json"
fusermount3 -u "$XDG_RUNTIME_DIR/headlessdesk"
```

Without an explicit mountpoint, `mount` uses `$XDG_RUNTIME_DIR/headlessdesk`
when available, then platform runtime/cache/temp fallbacks.

## Files

- `README.md`: filesystem usage summary.
- `health.json`: current backend status as JSON.
- `status.json`: same as `health.json`.
- `screenshot.png`: latest full screenshot as PNG.
- `crop/<x>,<y>,<w>,<h>.png`: cropped screenshot as PNG.
- `input/type`: raw text to type.
- `input/keypress`: key name to press and release.
- `input/click.json`: `{"x":640,"y":360,"button":"left"}`.
- `input/double_click.json`: same shape as `click.json`.
- `input/move.json`: `{"x":640,"y":360}`.
- `input/scroll.json`: `{"x":640,"y":360,"scrollY":120}`.
- `input/drag.json`: `{"path":[{"x":1,"y":1},{"x":2,"y":2}]}`.

Read files capture fresh service output on open. Input files collect write
chunks per open file handle and execute the corresponding service command on
close, so normal shell redirection works with JSON payloads.
