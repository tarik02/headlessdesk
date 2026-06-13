//go:build darwin && !cgo

package macoslocal

import (
	"errors"
	"image"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

type Backend struct{}

var errCGODisabled = errors.New("macOS backend requires cgo")

func New() (*Backend, error) {
	return nil, errCGODisabled
}

func (b *Backend) Status() desktop.Status {
	return desktop.Status{Protocol: "macos", State: "UNAVAILABLE", Error: errCGODisabled.Error()}
}

func (b *Backend) InputStatus() desktop.Status {
	return b.Status()
}

func (b *Backend) OutputStatus() desktop.Status {
	return b.Status()
}

func (b *Backend) Screenshot() (image.Image, error) {
	return nil, errCGODisabled
}

func (b *Backend) ScreenshotCrop(crop desktop.Crop) (image.Image, error) {
	return nil, errCGODisabled
}

func (b *Backend) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	return errCGODisabled
}

func (b *Backend) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	return errCGODisabled
}

func (b *Backend) TypeText(text string) error {
	return errCGODisabled
}

func (b *Backend) MoveMouse(x float64, y float64) error {
	return errCGODisabled
}

func (b *Backend) SendMouseButton(button inputcode.MouseButtonName, x float64, y float64, down bool) error {
	return errCGODisabled
}

func (b *Backend) SendMouseWheel(x float64, y float64, delta int, horizontal bool) error {
	return errCGODisabled
}

func (b *Backend) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func (b *Backend) Err() error {
	return errCGODisabled
}

func (b *Backend) Close() error {
	return nil
}
