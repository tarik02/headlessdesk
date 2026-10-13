# Installation and Builds

Build with Nix:

```bash
nix build .#
```

After changing `go.mod` or `go.sum`, update Nix module metadata:

```bash
gomod2nix generate
```

Run from the flake:

```bash
nix run .# -- serve --config ./server.yaml
```

Build with Go when FreeRDP and LibVNCClient development files are available
through `pkg-config`:

```bash
go build ./cmd/headlessdesk
```

Build release binaries locally:

```bash
just build-linux-amd64
just build-darwin-arm64
just build-windows-amd64
just snapshot
```

Linux builds include KWin screenshot and EIS input support. Windows builds
include the native local `windows` backend in addition to RDP, VNC, and command
backends. macOS builds include the native local `macos` backend in addition to
RDP, VNC, and command backends. macOS builds must run on macOS when cgo is
enabled because the native backend links Apple frameworks. Windows builds are
cross-compiled from Linux with posix MinGW and need FreeRDP and LibVNCClient
target libraries in `pkg-config`. Binary packaging is configured in
[`.goreleaser.yaml`](../.goreleaser.yaml).

Windows release zips include both `headlessdesk.exe` for console use and
`headlessdeskw.exe` for no-console background `serve` usage. The executable is
linked with static MinGW compiler/thread runtimes where possible, while FreeRDP
and LibVNCClient are still packaged as runtime DLLs.

## Backend Selection

All backends available on the target platform are included by default. Exclude
optional backends with Go build tags:

| Tag                    | Excludes | Drops dependency |
| ---------------------- | -------- | ---------------- |
| `headlessdesk_no_rdp`  | `rdp`    | FreeRDP          |
| `headlessdesk_no_vnc`  | `vnc`    | LibVNCClient     |
| `headlessdesk_no_kwin` | `kwin`   |                  |
| `headlessdesk_no_eis`  | `eis`    | libei            |

```bash
go build -tags headlessdesk_no_rdp,headlessdesk_no_vnc ./cmd/headlessdesk
```

Configs that select an excluded backend fail validation with
`backends.<name>.type <type> is not included in this build`.

The flake package takes a `backends` list (any subset of `rdp`, `vnc`, `kwin`,
`eis`; `command` is always included) and drops the matching build inputs:

```nix
headlessdesk.packages.${system}.default.override {
  backends = [ "kwin" "eis" ];
}
```

`packages.<system>.headlessdesk-local` is that KWin/EIS-only build:

```bash
nix build .#headlessdesk-local
```

## Dependencies

For RDP, FreeRDP development files must be visible to `pkg-config`:

- `freerdp3`
- `freerdp-client3`
- `winpr3`

For VNC, LibVNCClient development files must be visible to `pkg-config`:

- `libvncclient`

Linux KWin EIS input also requires libei development files:

- `libei-1.0`

macOS native local desktop support links ApplicationServices and CoreGraphics
from the macOS SDK. RDP and VNC support on macOS still require FreeRDP and
LibVNCClient development files visible to `pkg-config`, for example from
Homebrew or another local package manager.
