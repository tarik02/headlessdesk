//go:build !linux || headlessdesk_no_kwin

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
)

const kwinBackendIncluded = false

func startKWinBackend(name string) (desktop.OutputBackend, error) {
	return nil, fmt.Errorf("backend %q type kwin is not included in this build", name)
}
