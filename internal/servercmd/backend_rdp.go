//go:build !headlessdesk_no_rdp

package servercmd

import (
	"fmt"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/freerdp"
)

const rdpBackendIncluded = true

func startRDPBackend(name string, cfg backendConfig) (desktop.Session, error) {
	session, err := freerdp.StartSession(freerdp.Config{
		Host:               cfg.Host,
		Port:               backendPort(cfg, 3389),
		Username:           cfg.Username,
		Password:           cfg.Password,
		Domain:             cfg.RDP.Domain,
		DesktopWidth:       uint32(cfg.Width),
		DesktopHeight:      uint32(cfg.Height),
		KeyboardLayout:     uint32(cfg.RDP.KeyboardLayout),
		InsecureSkipVerify: cfg.Insecure,
		GraphicsMode:       cfg.RDP.GraphicsMode,
	})
	if err != nil {
		return nil, fmt.Errorf("start RDP backend %q: %w", name, err)
	}
	return session, nil
}
