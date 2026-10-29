//go:build darwin

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/macoslocal"
)

var platformBackendTypes = []string{"rdp", "vnc", "command", "nanokvm", "macos"}

func validateBackendPlatform(name string, backendType string) error {
	switch backendType {
	case "kwin", "eis":
		return fmt.Errorf("backends.%s.type %s is only supported on linux", name, backendType)
	case "windows":
		return fmt.Errorf("backends.%s.type windows is only supported on windows", name)
	default:
		return nil
	}
}

func startWindowsBackend(name string) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type windows is only supported on windows", name)
}

func startMacOSBackend(name string) (desktop.Session, error) {
	backend, err := macoslocal.New()
	if err != nil {
		return nil, fmt.Errorf("start macOS backend %q: %w", name, err)
	}
	return backend, nil
}
