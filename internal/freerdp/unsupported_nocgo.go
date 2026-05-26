//go:build !cgo

package freerdp

import (
	"errors"
	"image"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

type Config struct {
	Host               string
	Port               uint16
	Username           string
	Password           string
	Domain             string
	DesktopWidth       uint32
	DesktopHeight      uint32
	KeyboardLayout     uint32
	InsecureSkipVerify bool
	GraphicsMode       string
}

type ProbeResult struct {
	Connected bool
	Active    bool
	AuthOnly  bool
	State     string
	Version   string
	Warning   string
}

type Status = desktop.Status

type Session struct{}

var errCGODisabled = errors.New("FreeRDP backend requires cgo")

func Probe(cfg Config) (ProbeResult, error) {
	return ProbeResult{}, errCGODisabled
}

func StartSession(cfg Config) (*Session, error) {
	return nil, errCGODisabled
}

func (s *Session) Status() desktop.Status {
	return desktop.Status{Protocol: "rdp", State: "UNAVAILABLE", Error: errCGODisabled.Error()}
}

func (s *Session) Screenshot() (image.Image, error) {
	return nil, errCGODisabled
}

func (s *Session) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	return errCGODisabled
}

func (s *Session) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	return errCGODisabled
}

func (s *Session) TypeText(text string) error {
	return errCGODisabled
}

func (s *Session) MoveMouse(x int, y int) error {
	return errCGODisabled
}

func (s *Session) SendMouseButton(button inputcode.MouseButtonName, x int, y int, down bool) error {
	return errCGODisabled
}

func (s *Session) SendMouseWheel(x int, y int, delta int, horizontal bool) error {
	return errCGODisabled
}

func (s *Session) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func (s *Session) Err() error {
	return errCGODisabled
}

func (s *Session) Close() error {
	return nil
}
