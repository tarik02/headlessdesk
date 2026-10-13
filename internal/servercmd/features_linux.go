//go:build linux

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
)

var platformBackendTypes = []string{"rdp", "vnc", "command", "kwin", "eis"}

func validateBackendPlatform(name string, backendType string) error {
	switch backendType {
	case "windows":
		return fmt.Errorf("backends.%s.type windows is only supported on windows", name)
	case "macos":
		return fmt.Errorf("backends.%s.type macos is only supported on macos", name)
	}
	return nil
}

func startWindowsBackend(name string) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type windows is only supported on windows", name)
}

func startMacOSBackend(name string) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type macos is only supported on macos", name)
}
