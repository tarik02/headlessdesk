set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
    @just --list

build:
    mkdir -p bin
    go build -o bin/headlessdesk ./cmd/headlessdesk

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
