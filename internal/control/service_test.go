package control

import (
	"errors"
	"image"
	"reflect"
	"testing"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

type recordedKey struct {
	name string
	down bool
}

type fakeDesktopBackend struct {
	keyEvents []recordedKey
	failDown  string
}

func (b *fakeDesktopBackend) Done() <-chan struct{} {
	return nil
}

func (b *fakeDesktopBackend) Err() error {
	return nil
}

func (b *fakeDesktopBackend) Close() error {
	return nil
}

func (b *fakeDesktopBackend) Status() desktop.Status {
	return desktop.Status{Connected: true, Active: true, Ready: true, Width: 1, Height: 1}
}

func (b *fakeDesktopBackend) Screenshot() (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

func (b *fakeDesktopBackend) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	b.keyEvents = append(b.keyEvents, recordedKey{name: name.String(), down: down})
	if down && name.String() == b.failDown {
		return errors.New("send failed")
	}
	return nil
}

func (b *fakeDesktopBackend) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	return nil
}

func (b *fakeDesktopBackend) TypeText(text string) error {
	return nil
}

func (b *fakeDesktopBackend) MoveMouse(x int, y int) error {
	return nil
}

func (b *fakeDesktopBackend) SendMouseButton(button inputcode.MouseButtonName, x int, y int, down bool) error {
	return nil
}

func (b *fakeDesktopBackend) SendMouseWheel(x int, y int, delta int, horizontal bool) error {
	return nil
}

func TestKeypressSendsChordInReverseReleaseOrder(t *testing.T) {
	backend := &fakeDesktopBackend{}
	service := NewService(backend, backend, backend)

	if err := service.Keypress(KeypressCommand{Key: "CTRL+L"}); err != nil {
		t.Fatalf("Keypress() error = %v", err)
	}

	want := []recordedKey{
		{name: "KEY_LEFTCTRL", down: true},
		{name: "KEY_L", down: true},
		{name: "KEY_L", down: false},
		{name: "KEY_LEFTCTRL", down: false},
	}
	if !reflect.DeepEqual(backend.keyEvents, want) {
		t.Fatalf("key events = %#v, want %#v", backend.keyEvents, want)
	}
}

func TestKeypressReleasesHeldKeysWhenChordFails(t *testing.T) {
	backend := &fakeDesktopBackend{failDown: "KEY_L"}
	service := NewService(backend, backend, backend)

	if err := service.Keypress(KeypressCommand{Key: "CTRL+L"}); err == nil {
		t.Fatal("Keypress() error = nil")
	}

	want := []recordedKey{
		{name: "KEY_LEFTCTRL", down: true},
		{name: "KEY_L", down: true},
		{name: "KEY_LEFTCTRL", down: false},
	}
	if !reflect.DeepEqual(backend.keyEvents, want) {
		t.Fatalf("key events = %#v, want %#v", backend.keyEvents, want)
	}
}
