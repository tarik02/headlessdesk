package vnc

/*
#cgo pkg-config: libvncclient
#cgo windows LDFLAGS: -lws2_32
#include <stdlib.h>
#include "client_bridge.h"
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"runtime/cgo"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

// Config contains protocol-specific options for the VNC backend.
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

const (
	defaultPort          uint16 = 5900
	frameRequestInterval        = 100 * time.Millisecond
	waitInterval                = 100 * time.Millisecond
)

// Session is a LibVNCClient-backed remote desktop session.
type Session struct {
	client *C.rfbClient
	handle cgo.Handle
	done   chan struct{}

	mu        sync.RWMutex
	inputMu   sync.Mutex
	closeOnce sync.Once
	runErr    error
	status    desktop.Status
	frame     *image.NRGBA
}

// StartSession connects to a VNC server with LibVNCClient.
func StartSession(cfg Config) (desktop.Session, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	s := &Session{
		done: make(chan struct{}),
		status: desktop.Status{
			Protocol:  "vnc",
			Connected: true,
			Active:    true,
			State:     "CONNECTING",
		},
	}
	s.handle = cgo.NewHandle(s)

	host := C.CString(cfg.Host)
	password := C.CString(cfg.Password)
	defer C.free(unsafe.Pointer(host))
	defer C.free(unsafe.Pointer(password))

	shared := C.int(0)
	if cfg.Shared {
		shared = 1
	}
	viewOnly := C.int(0)
	if cfg.ViewOnly {
		viewOnly = 1
	}
	client := C.govnc_new_client(host, C.int(cfg.Port), password, shared, viewOnly, C.uintptr_t(s.handle))
	if client == nil {
		s.handle.Delete()
		return nil, fmt.Errorf("connect to VNC server %s:%d", cfg.Host, cfg.Port)
	}
	s.client = client

	s.mu.Lock()
	s.status.State = "CONNECTED"
	s.status.Width = int(client.width)
	s.status.Height = int(client.height)
	if client.desktopName != nil {
		s.status.Version = C.GoString(client.desktopName)
	}
	s.mu.Unlock()

	go s.run()
	return s, nil
}

func normalizeConfig(cfg Config) Config {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.Host == "" {
		return errors.New("vnc host is required")
	}
	if cfg.DesktopWidth == 0 {
		return errors.New("vnc width must be greater than zero")
	}
	if cfg.DesktopHeight == 0 {
		return errors.New("vnc height must be greater than zero")
	}
	return nil
}

func (s *Session) run() {
	defer close(s.done)

	ticker := time.NewTicker(frameRequestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.requestFrame(true); err != nil {
				s.setErr(err)
				s.setDisconnected()
				return
			}
		default:
			if C.govnc_wait_for_message(s.client, C.uint(waitInterval/time.Microsecond)) == 0 {
				s.setErr(errors.New("VNC wait for message failed"))
				s.setDisconnected()
				return
			}
			if C.govnc_handle_server_message(s.client) == 0 {
				s.setErr(errors.New("VNC read loop ended"))
				s.setDisconnected()
				return
			}
		}
	}
}

func (s *Session) updateFramebuffer(data unsafe.Pointer, width int, height int, bytesPerPixel int, bigEndian bool, redShift int, greenShift int, blueShift int, redMax int, greenMax int, blueMax int) {
	if data == nil || width <= 0 || height <= 0 || bytesPerPixel <= 0 {
		return
	}

	size := width * height * bytesPerPixel
	src := unsafe.Slice((*byte)(data), size)
	frame := image.NewNRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := (y*width + x) * bytesPerPixel
			pixel := readPixel(src[offset:offset+bytesPerPixel], bigEndian)
			dst := frame.PixOffset(x, y)
			frame.Pix[dst+0] = scaleComponent((pixel>>redShift)&uint32(redMax), redMax)
			frame.Pix[dst+1] = scaleComponent((pixel>>greenShift)&uint32(greenMax), greenMax)
			frame.Pix[dst+2] = scaleComponent((pixel>>blueShift)&uint32(blueMax), blueMax)
			frame.Pix[dst+3] = 0xff
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.frame = frame
	s.status.Ready = true
	s.status.Width = width
	s.status.Height = height
	s.status.State = "ACTIVE"
}

func readPixel(data []byte, bigEndian bool) uint32 {
	switch len(data) {
	case 4:
		if bigEndian {
			return binary.BigEndian.Uint32(data)
		}
		return binary.LittleEndian.Uint32(data)
	case 3:
		if bigEndian {
			return uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
		}
		return uint32(data[2])<<16 | uint32(data[1])<<8 | uint32(data[0])
	case 2:
		if bigEndian {
			return uint32(binary.BigEndian.Uint16(data))
		}
		return uint32(binary.LittleEndian.Uint16(data))
	case 1:
		return uint32(data[0])
	default:
		return 0
	}
}

func scaleComponent(value uint32, max int) byte {
	if max <= 0 {
		return 0
	}
	if max == 255 {
		return byte(value)
	}
	return byte((value * 255) / uint32(max))
}

func (s *Session) requestFrame(incremental bool) error {
	value := C.int(0)
	if incremental {
		value = 1
	}
	return s.withInputLock(func() error {
		if C.govnc_send_frame_request(s.client, value) == 0 {
			return errors.New("send VNC framebuffer request")
		}
		return nil
	})
}

func (s *Session) Status() desktop.Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.status
	if s.runErr != nil {
		status.Error = s.runErr.Error()
	}
	return status
}

func (s *Session) Screenshot() (image.Image, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.frame == nil || !s.status.Ready {
		return nil, errors.New("no framebuffer snapshot available yet")
	}

	clone := image.NewNRGBA(s.frame.Bounds())
	copy(clone.Pix, s.frame.Pix)
	return clone, nil
}

func (s *Session) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	key, err := keyFromName(name.String())
	if err != nil {
		return err
	}
	return s.withInputLock(func() error {
		if C.govnc_send_key(s.client, C.uint32_t(key), boolToCInt(down)) == 0 {
			return fmt.Errorf("send VNC key event: %s", name)
		}
		return nil
	})
}

func (s *Session) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	return fmt.Errorf("VNC does not support RDP scancode input: %d", scancode)
}

func (s *Session) TypeText(text string) error {
	if text == "" {
		return errors.New("text is required")
	}
	return s.withInputLock(func() error {
		for _, r := range text {
			key := keysymForRune(r)
			if key == 0 {
				return fmt.Errorf("unsupported text rune: %s", strconv.QuoteRune(r))
			}
			if C.govnc_send_key(s.client, C.uint32_t(key), 1) == 0 {
				return fmt.Errorf("send VNC key down: %s", strconv.QuoteRune(r))
			}
			if C.govnc_send_key(s.client, C.uint32_t(key), 0) == 0 {
				return fmt.Errorf("send VNC key up: %s", strconv.QuoteRune(r))
			}
		}
		return nil
	})
}

func (s *Session) MoveMouse(x int, y int) error {
	mouseX, mouseY, err := validatePointerPosition(x, y)
	if err != nil {
		return err
	}
	return s.sendPointer(mouseX, mouseY, 0)
}

func (s *Session) SendMouseButton(button inputcode.MouseButtonName, x int, y int, down bool) error {
	mouseX, mouseY, err := validatePointerPosition(x, y)
	if err != nil {
		return err
	}
	mask, err := buttonMask(button.String())
	if err != nil {
		return err
	}
	if !down {
		mask = 0
	}
	return s.sendPointer(mouseX, mouseY, mask)
}

func (s *Session) SendMouseWheel(x int, y int, delta int, horizontal bool) error {
	mouseX, mouseY, err := validatePointerPosition(x, y)
	if err != nil {
		return err
	}
	if delta == 0 {
		return errors.New("wheel delta must be non-zero")
	}
	mask := wheelMask(delta, horizontal)
	return s.withInputLock(func() error {
		if C.govnc_send_pointer(s.client, C.int(mouseX), C.int(mouseY), C.int(mask)) == 0 {
			return errors.New("send VNC wheel event")
		}
		if C.govnc_send_pointer(s.client, C.int(mouseX), C.int(mouseY), 0) == 0 {
			return errors.New("release VNC wheel event")
		}
		return nil
	})
}

func (s *Session) sendPointer(x int, y int, mask int) error {
	return s.withInputLock(func() error {
		if C.govnc_send_pointer(s.client, C.int(x), C.int(y), C.int(mask)) == 0 {
			return fmt.Errorf("send VNC pointer event: %d,%d", x, y)
		}
		return nil
	})
}

func (s *Session) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runErr
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		if s.client != nil {
			C.govnc_close_client(s.client)
			<-s.done
			C.govnc_cleanup_client(s.client)
			s.client = nil
		}
		s.handle.Delete()
	})
	return s.Err()
}

func (s *Session) withInputLock(fn func() error) error {
	s.inputMu.Lock()
	defer s.inputMu.Unlock()
	return fn()
}

func (s *Session) setErr(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runErr == nil {
		s.runErr = err
		s.status.Error = err.Error()
	}
}

func (s *Session) setDisconnected() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Connected = false
	s.status.Active = false
	if s.runErr == nil {
		s.status.State = "CLOSED"
	} else {
		s.status.State = "ERROR"
	}
}

func validatePointerPosition(x int, y int) (int, int, error) {
	if x < 0 || y < 0 || x > int(^uint16(0)) || y > int(^uint16(0)) {
		return 0, 0, fmt.Errorf("pointer position out of range: %d,%d", x, y)
	}
	return x, y, nil
}

func buttonMask(button string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(button)) {
	case "left", "button1", "1":
		return 1, nil
	case "middle", "button2", "2":
		return 2, nil
	case "right", "button3", "3":
		return 4, nil
	case "back", "button8", "8":
		return 128, nil
	case "forward", "button9", "9":
		return 256, nil
	default:
		return 0, fmt.Errorf("unsupported mouse button: %s", button)
	}
}

func wheelMask(delta int, horizontal bool) int {
	if horizontal {
		if delta < 0 {
			return 32
		}
		return 64
	}
	if delta < 0 {
		return 16
	}
	return 8
}

func keyFromName(name string) (uint32, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 0, errors.New("key name is required")
	}
	trimmedRunes := []rune(trimmed)
	if len(trimmedRunes) == 1 {
		if key := keysymForRune(trimmedRunes[0]); key != 0 {
			return key, nil
		}
	}
	normalized := strings.ToLower(trimmed)
	if key, ok := namedKeys[normalized]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unsupported key name: %s", name)
}

func keysymForRune(r rune) uint32 {
	switch r {
	case '\n', '\r':
		return namedKeys["enter"]
	case '\t':
		return namedKeys["tab"]
	case '\b':
		return namedKeys["backspace"]
	}
	if r >= 0x20 && r <= 0xff {
		return uint32(r)
	}
	return 0
}

func boolToCInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

var namedKeys = map[string]uint32{
	"alt":       0xffe9,
	"backspace": 0xff08,
	"capslock":  0xffe5,
	"ctrl":      0xffe3,
	"control":   0xffe3,
	"delete":    0xffff,
	"down":      0xff54,
	"end":       0xff57,
	"enter":     0xff0d,
	"esc":       0xff1b,
	"escape":    0xff1b,
	"f1":        0xffbe,
	"f2":        0xffbf,
	"f3":        0xffc0,
	"f4":        0xffc1,
	"f5":        0xffc2,
	"f6":        0xffc3,
	"f7":        0xffc4,
	"f8":        0xffc5,
	"f9":        0xffc6,
	"f10":       0xffc7,
	"f11":       0xffc8,
	"f12":       0xffc9,
	"home":      0xff50,
	"insert":    0xff63,
	"left":      0xff51,
	"pagedown":  0xff56,
	"page_down": 0xff56,
	"pageup":    0xff55,
	"page_up":   0xff55,
	"return":    0xff0d,
	"right":     0xff53,
	"shift":     0xffe1,
	"space":     0x20,
	"super":     0xffeb,
	"tab":       0xff09,
	"up":        0xff52,
	"win":       0xffeb,
	"windows":   0xffeb,
}

//export goVNCFramebuffer
func goVNCFramebuffer(handle C.uintptr_t, data *C.uint8_t, width C.int, height C.int, bytesPerPixel C.int, bigEndian C.int, redShift C.int, greenShift C.int, blueShift C.int, redMax C.int, greenMax C.int, blueMax C.int) {
	sessionHandle := cgo.Handle(handle)
	session, ok := sessionHandle.Value().(*Session)
	if !ok {
		return
	}
	session.updateFramebuffer(
		unsafe.Pointer(data),
		int(width),
		int(height),
		int(bytesPerPixel),
		bigEndian != 0,
		int(redShift),
		int(greenShift),
		int(blueShift),
		int(redMax),
		int(greenMax),
		int(blueMax),
	)
}
