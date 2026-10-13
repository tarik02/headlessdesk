//go:build linux && !headlessdesk_no_kwin

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/kwin"
)

const kwinBackendIncluded = true

func startKWinBackend(name string) (desktop.OutputBackend, error) {
	backend, err := kwin.New()
	if err != nil {
		return nil, fmt.Errorf("start KWin backend %q: %w", name, err)
	}
	return backend, nil
}
