set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
    @just --list

build:
    mkdir -p bin
    go build -o bin/headlessdesk ./cmd/headlessdesk

goreleaser := "go run github.com/goreleaser/goreleaser/v2@v2.15.0"
changeset := "npm --prefix .changeset exec --package @changesets/cli -- changeset"

binary target="linux-amd64":
    case "{{target}}" in \
        linux-*) id=headlessdesk-linux; output=dist/headlessdesk-{{target}} ;; \
        darwin-arm64) id=headlessdesk-darwin; output=dist/headlessdesk-{{target}} ;; \
        windows-*) id=headlessdesk-windows-mingw; output=dist/headlessdesk-{{target}}.exe ;; \
        *) echo "unsupported target: {{target}}" >&2; exit 2 ;; \
    esac; \
    TARGET={{replace(target, "-", "_")}} {{goreleaser}} build --snapshot --clean --single-target --id "$id" --output "$output"

snapshot:
    {{goreleaser}} release --snapshot --clean --skip=publish

changesets-install:
    npm install --package-lock-only --prefix .changeset

changeset: changesets-install
    {{changeset}}

changeset-version: changesets-install
    {{changeset}} version

build-linux-amd64:
    just binary linux-amd64

build-linux-arm64:
    just binary linux-arm64

build-darwin-arm64:
    just binary darwin-arm64

build-windows-amd64:
    just binary windows-amd64

install-bin: build
    install -Dm755 bin/headlessdesk "$HOME/.local/bin/headlessdesk"

install-config:
    install -Dm644 config.yaml "$HOME/.config/headlessdesk/config.yml"

install-desktop-entry: install-bin
    install -d "$HOME/.local/share/applications"
    sed "s|[{][{]HOME[}][}]|$HOME|g" deploy/applications/headlessdesk.desktop > "$HOME/.local/share/applications/headlessdesk.desktop"
    if command -v kbuildsycoca6 >/dev/null 2>&1; then kbuildsycoca6 --noincremental >/dev/null 2>&1 || true; fi

install-systemd: install-bin install-config install-desktop-entry
    install -d "$HOME/.config/systemd/user"
    install -m 0644 deploy/systemd/headlessdesk.service "$HOME/.config/systemd/user/headlessdesk.service"
    rm -rf "$HOME/.config/systemd/user/headlessdesk.service.d"
    systemctl --user daemon-reload
    systemctl --user enable headlessdesk.service

install: install-systemd

start:
    systemctl --user start headlessdesk.service

restart:
    systemctl --user restart headlessdesk.service

stop:
    systemctl --user stop headlessdesk.service

status:
    systemctl --user --no-pager --lines=30 status headlessdesk.service

health:
    curl -fsS http://127.0.0.1:4103/healthz
