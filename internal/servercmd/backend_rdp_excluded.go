//go:build headlessdesk_no_rdp

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
)

const rdpBackendIncluded = false

func startRDPBackend(name string, cfg backendConfig) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type rdp is not included in this build", name)
}
