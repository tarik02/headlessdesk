//go:build linux && !headlessdesk_no_eis

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/kwineis"
)

const eisBackendIncluded = true

func startKWinEISBackend(name string) (desktop.InputBackend, error) {
	backend, err := kwineis.New()
	if err != nil {
		return nil, fmt.Errorf("start KWin EIS backend %q: %w", name, err)
	}
	return backend, nil
}
