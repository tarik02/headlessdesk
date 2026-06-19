# Setup

Use this after routing when readiness, dimensions, or scopes matter. Do not use
this as backend configuration or install docs.

## Ready Gate

Before sending input, read current status and keep the reported width and
height. `ready` means screenshot output is available. `input_ready` and
`output_ready` show split backend state when present.

Use status dimensions for coordinates. Re-check status after connection errors,
backend restarts, resolution changes, or permission errors.

## Scopes

Read scopes: `read:status`, `read:screenshot`, `read:wait`.
Write scopes: `write:mouse`, `write:keyboard`.
Wildcards: `read:*`, `write:*`, `*`.
