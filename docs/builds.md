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
backends. macOS builds include RDP, VNC, and command backends. Windows builds
are cross-compiled from Linux with posix MinGW and need FreeRDP and LibVNCClient
target libraries in `pkg-config`. Binary packaging is configured in
[`.goreleaser.yaml`](../.goreleaser.yaml).

Windows release zips include both `headlessdesk.exe` for console use and
`headlessdeskw.exe` for no-console background `serve` usage. The executable is
linked with static MinGW compiler/thread runtimes where possible, while FreeRDP
and LibVNCClient are still packaged as runtime DLLs.

## Dependencies

For RDP, FreeRDP development files must be visible to `pkg-config`:

- `freerdp3`
- `freerdp-client3`
- `winpr3`

For VNC, LibVNCClient development files must be visible to `pkg-config`:

- `libvncclient`

Linux KWin EIS input also requires libei development files:

- `libei-1.0`
