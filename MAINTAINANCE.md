# Maintainance

## Nix build

```bash
nix build .#
```

Git-backed flakes only see tracked files. Add new files before relying on a
normal flake build.

## Go dependencies

Go module hashes are maintained through `gomod2nix.toml`, not `vendorHash`.

After changing `go.mod` or `go.sum`:

```bash
gomod2nix generate
nix build .#
```

If local Nix is unavailable:

```bash
nix develop --command gomod2nix generate
```

Commit `go.mod`, `go.sum`, and `gomod2nix.toml` together.

## Flake inputs

After changing inputs in `flake.nix`:

```bash
nix flake lock
nix build .#
```

Commit `flake.nix` and `flake.lock` together.

## Native dependencies

CGO packages require matching Nix build inputs:

- `internal/freerdp`: `freerdp`
- `internal/vnc`: `libvncserver`
- `internal/kwineis`: `libei`

When adding a new `#cgo pkg-config:` dependency, add the matching package to
both the package `buildInputs` and `devShells.default.packages` in `flake.nix`.
Backends with native dependencies should have a `headlessdesk_no_<type>` build
tag in `internal/servercmd` and an entry in the flake's `optionalBackends`.

## Checks

Run before handing off Nix changes:

```bash
go test ./...
nix build .# .#headlessdesk-local
```
