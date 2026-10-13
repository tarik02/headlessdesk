//go:build !linux || headlessdesk_no_eis

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
)

const eisBackendIncluded = false

func startKWinEISBackend(name string) (desktop.InputBackend, error) {
	return nil, fmt.Errorf("backend %q type eis is not included in this build", name)
}
