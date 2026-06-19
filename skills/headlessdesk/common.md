# Common Behavior

Load this when coordinates, screenshots, expressions, or input semantics matter.

## Coordinates

Coordinates are in output screenshot space. Some backends map those coordinates
to a different input space internally. Use status dimensions and cropped
screenshots to confirm the target before clicking or dragging.

## Numeric Expressions

Geometry fields accept JSON numbers or arithmetic expression strings.
Supported expression syntax: numeric literals, parentheses, `+`, `-`, `*`,
`/`, unary `+`, unary `-`.

Expression fields:

- pointer `x`, `y` in `click`, `double_click`, `drag`, `move`, `scroll`;
- screenshot crop `x`, `y`, `w`, `h`;
- scroll `scrollX`, `scrollY`.

Pointer expressions may resolve to fractions. Integer-only backends round
pointer coordinates. Crop fields and scroll deltas round to integers.

Examples:

```json
{"x":"1280/2","y":"720/2","button":"left"}
{"crop":{"x":"1280/2-200","y":"720/2-150","w":400,"h":300}}
{"x":"640","y":"360","scrollY":"120*3"}
```

## Screenshots

Prefer cropped screenshots once the relevant region is known. Use a full
screenshot only for initial orientation or when the target area is unknown.

## Input

Mouse buttons: `left`, `middle`, `right`. Drag uses the left button and needs at
least two path points. Scroll sends `scrollX` and/or `scrollY`; wheel deltas are
normally multiples of `120`.

Keyboard input:

- `keypress` presses and releases a named key or `+`-delimited chord, for
  example `enter`, `Ctrl+L`, `Ctrl+Shift+P`, `Alt+Tab`.
- `type` sends text into the remote session.
