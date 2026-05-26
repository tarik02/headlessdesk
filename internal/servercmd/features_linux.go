//go:build linux

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/kwin"
	"headlessdesk/internal/kwineis"
)

func supportedBackendTypesDescription() string {
	return "rdp, vnc, command, kwin, or eis"
}

func validateBackendPlatform(name string, backendType string) error {
	if backendType == "windows" {
		return fmt.Errorf("backends.%s.type windows is only supported on windows", name)
	}
	return nil
}

func startKWinBackend(name string) (desktop.OutputBackend, error) {
	backend, err := kwin.New()
	if err != nil {
		return nil, fmt.Errorf("start KWin backend %q: %w", name, err)
	}
	return backend, nil
}

func startKWinEISBackend(name string) (desktop.InputBackend, error) {
	backend, err := kwineis.New()
	if err != nil {
		return nil, fmt.Errorf("start KWin EIS backend %q: %w", name, err)
	}
	return backend, nil
}

func startWindowsBackend(name string) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type windows is only supported on windows", name)
}
