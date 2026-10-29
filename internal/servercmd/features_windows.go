//go:build windows

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/winlocal"
)

var platformBackendTypes = []string{"rdp", "vnc", "command", "nanokvm", "windows"}

func validateBackendPlatform(name string, backendType string) error {
	switch backendType {
	case "kwin", "eis":
		return fmt.Errorf("backends.%s.type %s is only supported on linux", name, backendType)
	case "macos":
		return fmt.Errorf("backends.%s.type macos is only supported on macos", name)
	default:
		return nil
	}
}

func startWindowsBackend(name string) (desktop.Session, error) {
	backend, err := winlocal.New()
	if err != nil {
		return nil, fmt.Errorf("start Windows backend %q: %w", name, err)
	}
	return backend, nil
}

func startMacOSBackend(name string) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type macos is only supported on macos", name)
}
