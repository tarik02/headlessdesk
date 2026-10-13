//go:build !headlessdesk_no_rdp || !headlessdesk_no_vnc

package servercmd

func backendPort(cfg backendConfig, fallback uint16) uint16 {
	if cfg.Port == 0 {
		return fallback
	}
	return uint16(cfg.Port)
}
