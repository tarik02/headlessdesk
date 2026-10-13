//go:build !headlessdesk_no_vnc

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/vnc"
)

const vncBackendIncluded = true

func startVNCBackend(name string, cfg backendConfig) (desktop.Session, error) {
	session, err := vnc.StartSession(vnc.Config{
		Host:          cfg.Host,
		Port:          backendPort(cfg, 5900),
		Username:      cfg.Username,
		Password:      cfg.Password,
		DesktopWidth:  uint32(cfg.Width),
		DesktopHeight: uint32(cfg.Height),
		Shared:        cfg.VNC.Shared,
		ViewOnly:      cfg.VNC.ViewOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("start VNC backend %q: %w", name, err)
	}
	return session, nil
}
