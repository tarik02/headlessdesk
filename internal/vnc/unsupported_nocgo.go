//go:build !cgo

package vnc

import (
	"errors"

	"headlessdesk/internal/desktop"
)

type Config struct {
	Host          string
	Port          uint16
	Username      string
	Password      string
	DesktopWidth  uint32
	DesktopHeight uint32
	Shared        bool
	ViewOnly      bool
}

var errCGODisabled = errors.New("VNC backend requires cgo")

func StartSession(cfg Config) (desktop.Session, error) {
	return nil, errCGODisabled
}
