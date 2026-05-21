//go:build !linux

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
)

func supportedBackendTypesDescription() string {
	return "rdp, vnc, or command"
}

func validateBackendPlatform(name string, backendType string) error {
	switch backendType {
	case "kwin", "eis":
		return fmt.Errorf("backends.%s.type %s is only supported on linux", name, backendType)
	default:
		return nil
	}
}

func startKWinBackend(name string) (desktop.OutputBackend, error) {
	return nil, fmt.Errorf("backend %q type kwin is only supported on linux", name)
}

func startKWinEISBackend(name string) (desktop.InputBackend, error) {
	return nil, fmt.Errorf("backend %q type eis is only supported on linux", name)
}
