//go:build headlessdesk_no_vnc

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
)

const vncBackendIncluded = false

func startVNCBackend(name string, cfg backendConfig) (desktop.Session, error) {
	return nil, fmt.Errorf("backend %q type vnc is not included in this build", name)
}
