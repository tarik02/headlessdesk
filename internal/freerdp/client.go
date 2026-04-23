package freerdp

/*
#cgo pkg-config: freerdp3 freerdp-client3 winpr3

#include <stdlib.h>
#include "client_bridge.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"
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

const defaultKeyboardLayout = 0x0409
const inputSyncSettleDelay = 50 * time.Millisecond
const interKeyDelay = 35 * time.Millisecond
const keyPressDuration = 20 * time.Millisecond
const modifierTransitionDelay = 20 * time.Millisecond
const modifierSettleDelay = 80 * time.Millisecond
const startupSnapshotTimeout = 8 * time.Second
const startupPollInterval = 250 * time.Millisecond
const blankSnapshotLimit = 4

type ProbeResult struct {
	Connected bool
	Active    bool
	AuthOnly  bool
	State     string
	Version   string
	Warning   string
}

type Status struct {
	Connected bool   `json:"connected"`
	Active    bool   `json:"active"`
	Ready     bool   `json:"ready"`
	State     string `json:"state"`
	Version   string `json:"freerdp"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Error     string `json:"error,omitempty"`
}

type Session struct {
	mu        sync.RWMutex
	client    *C.gofreerdp_client
	done      chan struct{}
	errMu     sync.RWMutex
	inputMu   sync.Mutex
	runErr    error
	closeOnce sync.Once
}

func Probe(cfg Config) (ProbeResult, error) {
	cfg = normalizeConfig(cfg)
	if err := validateAuthConfig(cfg); err != nil {
		return ProbeResult{}, err
	}

	client := C.gofreerdp_client_new_instance()
	if client == nil {
		return ProbeResult{}, errors.New("failed to allocate freerdp client")
	}
	defer C.gofreerdp_client_free_instance(client)

	host, username, password, domain, freeAll := cStrings(cfg)
	defer freeAll()

	graphicsPipeline, gfxH264, gfxAVC444 := graphicsModeSettings(cfg.GraphicsMode)
	if C.gofreerdp_configure(
		client,
		host,
		C.uint16_t(cfg.Port),
		username,
		password,
		domain,
		C.uint32_t(cfg.DesktopWidth),
		C.uint32_t(cfg.DesktopHeight),
		C.uint32_t(cfg.KeyboardLayout),
		cBool(cfg.InsecureSkipVerify),
		C.TRUE,
		cBool(graphicsPipeline),
		cBool(gfxH264),
		cBool(gfxAVC444),
	) == 0 {
		return ProbeResult{}, fmt.Errorf("freerdp configure failed: %s", goErrorString(client))
	}

	status := C.gofreerdp_probe_auth_only(client)
	if status == C.GOFREERDP_CONNECT_FAILED {
		return ProbeResult{}, fmt.Errorf("freerdp connect failed: %s", goErrorString(client))
	}

	result := ProbeResult{
		Connected: true,
		Active:    C.gofreerdp_is_active(client) != 0,
		AuthOnly:  status == C.GOFREERDP_CONNECT_AUTH_ONLY_OK,
		State:     C.GoString(C.gofreerdp_state(client)),
		Version:   C.GoString(C.gofreerdp_version()),
	}
	if result.AuthOnly {
		result.Warning = "auth-only succeeded, but freerdp reported a transport failure while tearing down the non-interactive session"
	}
	return result, nil
}

func StartSession(cfg Config) (*Session, error) {
	cfg = normalizeConfig(cfg)
	if err := validateAuthConfig(cfg); err != nil {
		return nil, err
	}

	modes := startupGraphicsModes(cfg.GraphicsMode)
	var startupErrs []string
	for i, mode := range modes {
		session, err := startSessionOnce(withGraphicsMode(cfg, mode))
		if err != nil {
			startupErrs = append(startupErrs, fmt.Sprintf("%s: %v", mode, err))
			continue
		}
		if i == len(modes)-1 {
			return session, nil
		}

		usable, probeErr := waitForUsableStartupSnapshot(session, startupSnapshotTimeout)
		if usable {
			return session, nil
		}
		startupErrs = append(startupErrs, fmt.Sprintf("%s: %v", mode, probeErr))
		_ = session.Close()
	}
	return nil, fmt.Errorf("unable to start usable RDP session (%s)", strings.Join(startupErrs, "; "))
}

func startSessionOnce(cfg Config) (*Session, error) {
	client, err := newConfiguredClient(cfg, false)
	if err != nil {
		return nil, err
	}

	s := &Session{
		client: client,
		done:   make(chan struct{}),
	}
	go s.run()
	return s, nil
}

func (s *Session) run() {
	defer close(s.done)

	client := s.getClient()
	if client == nil {
		s.setErr(errors.New("session client is not initialized"))
		return
	}

	if status := C.gofreerdp_run_persistent(client); status == 0 {
		s.setErr(fmt.Errorf("freerdp session ended: %s", goErrorString(client)))
	}
}

func (s *Session) Status() Status {
	client := s.getClient()
	status := Status{Version: C.GoString(C.gofreerdp_version()), Error: s.ErrString()}
	if client == nil {
		status.State = "CLOSED"
		return status
	}

	var width C.uint32_t
	var height C.uint32_t
	var stride C.uint32_t
	size := C.gofreerdp_snapshot_size(client, &width, &height, &stride)
	status.Connected = C.gofreerdp_is_connected(client) != 0
	status.Active = C.gofreerdp_is_active(client) != 0
	status.Ready = size > 0
	status.State = C.GoString(C.gofreerdp_state(client))
	status.Width = int(width)
	status.Height = int(height)
	return status
}

func (s *Session) ScreenshotPNG() ([]byte, error) {
	client := s.getClient()
	if client == nil {
		return nil, errors.New("session is closed")
	}

	var width C.uint32_t
	var height C.uint32_t
	var stride C.uint32_t
	size := C.gofreerdp_snapshot_size(client, &width, &height, &stride)
	if size == 0 {
		return nil, errors.New("no framebuffer snapshot available yet")
	}

	buf := make([]byte, int(size))
	if C.gofreerdp_copy_snapshot(
		client,
		(*C.BYTE)(unsafe.Pointer(&buf[0])),
		C.size_t(len(buf)),
		&width,
		&height,
		&stride,
	) == 0 {
		return nil, errors.New("failed to copy framebuffer snapshot")
	}

	w := int(width)
	h := int(height)
	rowStride := int(stride)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcRow := buf[y*rowStride:]
		dstRow := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			src := x * 4
			dst := x * 4
			dstRow[dst+0] = srcRow[src+2]
			dstRow[dst+1] = srcRow[src+1]
			dstRow[dst+2] = srcRow[src+0]
			dstRow[dst+3] = srcRow[src+3]
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode screenshot: %w", err)
	}
	return out.Bytes(), nil
}

func (s *Session) SendKey(name string, down bool, repeat bool) error {
	normalized := normalizeKeyName(name)
	if normalized == "" {
		return errors.New("key name is required")
	}

	scancode, ok := lookupKeyScancode(normalized)
	if !ok {
		return fmt.Errorf("unsupported key name: %s", name)
	}
	return s.SendKeyScancode(scancode, down, repeat)
}

func (s *Session) SendKeyScancode(scancode uint32, down bool, repeat bool) error {
	client := s.getClient()
	if client == nil {
		return errors.New("session is closed")
	}
	if scancode == 0 {
		return errors.New("scancode must be non-zero")
	}

	s.inputMu.Lock()
	defer s.inputMu.Unlock()

	if err := sendKeyboardScancodeLocked(client, scancode, down, repeat); err != nil {
		return err
	}
	return nil
}

func (s *Session) TypeText(text string) error {
	client := s.getClient()
	if client == nil {
		return errors.New("session is closed")
	}
	if text == "" {
		return errors.New("text is required")
	}

	s.inputMu.Lock()
	defer s.inputMu.Unlock()

	useUnicode := C.gofreerdp_unicode_input_available(client) != 0
	runes := []rune(text)

	if err := syncKeyboardStateLocked(client); err != nil {
		return err
	}
	time.Sleep(inputSyncSettleDelay)
	releaseModifiersLocked(client)
	time.Sleep(modifierSettleDelay)

	for i, r := range runes {
		if useUnicode {
			switch r {
			case '\n', '\r':
				if err := sendNamedKeyLocked(client, "RETURN", false); err != nil {
					return err
				}
			case '\t':
				if err := sendNamedKeyLocked(client, "TAB", false); err != nil {
					return err
				}
			default:
				if err := sendRuneUnicodeLocked(client, r); err != nil {
					return err
				}
			}
		} else if keyName, shift, ok := asciiKeyNameForRune(r); ok {
			if err := sendASCIIKeyLocked(client, keyName, shift); err != nil {
				return err
			}
		} else {
			if err := sendRuneUnicodeLocked(client, r); err != nil {
				return err
			}
		}

		if i < len(runes)-1 {
			time.Sleep(interKeyDelay)
		}
	}
	return nil
}

func sendNamedKeyLocked(client *C.gofreerdp_client, keyName string, repeat bool) error {
	scancode, found := lookupKeyScancode(keyName)
	if !found {
		return fmt.Errorf("unsupported key name: %s", keyName)
	}
	if err := sendKeyboardScancodeLocked(client, scancode, true, repeat); err != nil {
		return err
	}
	time.Sleep(keyPressDuration)
	return sendKeyboardScancodeLocked(client, scancode, false, false)
}

func sendASCIIKeyLocked(client *C.gofreerdp_client, keyName string, shift bool) error {
	scancode, found := lookupKeyScancode(keyName)
	if !found {
		return fmt.Errorf("unsupported text rune key: %s", keyName)
	}
	if !shift {
		return sendNamedKeyLocked(client, keyName, false)
	}

	shiftCode, found := lookupKeyScancode("LSHIFT")
	if !found {
		return errors.New("left shift scancode is unavailable")
	}
	if err := sendKeyboardScancodeLocked(client, shiftCode, true, false); err != nil {
		return err
	}
	time.Sleep(modifierTransitionDelay)
	if err := sendKeyboardScancodeLocked(client, scancode, true, false); err != nil {
		return err
	}
	time.Sleep(keyPressDuration)
	if err := sendKeyboardScancodeLocked(client, scancode, false, false); err != nil {
		return err
	}
	time.Sleep(modifierTransitionDelay)
	return sendKeyboardScancodeLocked(client, shiftCode, false, false)
}

func syncKeyboardStateLocked(client *C.gofreerdp_client) error {
	if C.gofreerdp_send_input_synchronize(client, 0) == 0 {
		return errors.New(goErrorString(client))
	}
	if C.gofreerdp_send_focus_in(client, 0) == 0 {
		return errors.New(goErrorString(client))
	}
	return nil
}

func releaseModifiersLocked(client *C.gofreerdp_client) {
	for _, keyName := range []string{"LSHIFT", "RSHIFT", "LCONTROL", "RCONTROL", "LMENU", "RMENU", "LWIN", "RWIN"} {
		scancode, found := lookupKeyScancode(keyName)
		if !found {
			continue
		}
		_ = sendKeyboardScancodeLocked(client, scancode, false, false)
	}
}

func sendRuneUnicodeLocked(client *C.gofreerdp_client, r rune) error {
	units := utf16.Encode([]rune{r})
	for _, unit := range units {
		if err := sendUnicodeLocked(client, uint16(unit), false); err != nil {
			return err
		}
		time.Sleep(keyPressDuration)
		if err := sendUnicodeLocked(client, uint16(unit), true); err != nil {
			return err
		}
	}
	return nil
}

func sendKeyboardScancodeLocked(client *C.gofreerdp_client, scancode uint32, down bool, repeat bool) error {
	if C.gofreerdp_send_keyboard_scancode(
		client,
		C.uint32_t(scancode),
		cBool(down),
		cBool(repeat),
	) == 0 {
		return errors.New(goErrorString(client))
	}
	return nil
}

func sendUnicodeLocked(client *C.gofreerdp_client, code uint16, release bool) error {
	if C.gofreerdp_send_unicode_keyboard(client, C.uint16_t(code), cBool(release)) == 0 {
		return errors.New(goErrorString(client))
	}
	return nil
}

func (s *Session) MoveMouse(x int, y int) error {
	client := s.getClient()
	if client == nil {
		return errors.New("session is closed")
	}

	mouseX, mouseY, err := validateMousePosition(x, y)
	if err != nil {
		return err
	}

	s.inputMu.Lock()
	defer s.inputMu.Unlock()

	if C.gofreerdp_send_mouse(client, C.uint16_t(C.PTR_FLAGS_MOVE), mouseX, mouseY) == 0 {
		return errors.New(goErrorString(client))
	}
	return nil
}

func (s *Session) SendMouseButton(button string, x int, y int, down bool) error {
	client := s.getClient()
	if client == nil {
		return errors.New("session is closed")
	}

	mouseX, mouseY, err := validateMousePosition(x, y)
	if err != nil {
		return err
	}

	flags, extended, err := mouseButtonFlags(button, down)
	if err != nil {
		return err
	}

	s.inputMu.Lock()
	defer s.inputMu.Unlock()

	if extended {
		if C.gofreerdp_send_extended_mouse(client, flags, mouseX, mouseY) == 0 {
			return errors.New(goErrorString(client))
		}
		return nil
	}

	if C.gofreerdp_send_mouse(client, flags, mouseX, mouseY) == 0 {
		return errors.New(goErrorString(client))
	}
	return nil
}

func (s *Session) SendMouseWheel(x int, y int, delta int, horizontal bool) error {
	client := s.getClient()
	if client == nil {
		return errors.New("session is closed")
	}

	mouseX, mouseY, err := validateMousePosition(x, y)
	if err != nil {
		return err
	}
	if delta == 0 {
		return errors.New("wheel delta must be non-zero")
	}

	amount := delta
	flags := uint16(0)
	if horizontal {
		flags |= uint16(C.PTR_FLAGS_HWHEEL)
	} else {
		flags |= uint16(C.PTR_FLAGS_WHEEL)
	}
	if amount < 0 {
		flags |= uint16(C.PTR_FLAGS_WHEEL_NEGATIVE)
		amount = -amount
	}
	if amount > int(C.WheelRotationMask) {
		return fmt.Errorf("wheel delta must be between -%d and %d", int(C.WheelRotationMask), int(C.WheelRotationMask))
	}
	flags |= uint16(amount)

	s.inputMu.Lock()
	defer s.inputMu.Unlock()

	if C.gofreerdp_send_mouse(client, C.uint16_t(flags), mouseX, mouseY) == 0 {
		return errors.New(goErrorString(client))
	}
	return nil
}

func (s *Session) Err() error {
	s.errMu.RLock()
	defer s.errMu.RUnlock()
	return s.runErr
}

func (s *Session) ErrString() string {
	if err := s.Err(); err != nil {
		return err.Error()
	}
	return ""
}

func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		client := s.getClient()
		if client != nil {
			C.gofreerdp_abort(client)
		}
		<-s.done
		if client != nil {
			s.clearClient(client)
			C.gofreerdp_client_free_instance(client)
		}
	})
	return s.Err()
}

func (s *Session) getClient() *C.gofreerdp_client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *Session) setClient(client *C.gofreerdp_client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = client
}

func (s *Session) clearClient(client *C.gofreerdp_client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == client {
		s.client = nil
	}
}

func (s *Session) setErr(err error) {
	if err == nil {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.runErr == nil {
		s.runErr = err
	}
}

func validateAuthConfig(cfg Config) error {
	if cfg.Host == "" {
		return errors.New("host is required")
	}
	if cfg.Username == "" {
		return errors.New("username is required")
	}
	if cfg.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Port == 0 {
		cfg.Port = 3389
	}
	if cfg.DesktopWidth == 0 {
		cfg.DesktopWidth = 1280
	}
	if cfg.DesktopHeight == 0 {
		cfg.DesktopHeight = 720
	}
	if cfg.KeyboardLayout == 0 {
		cfg.KeyboardLayout = defaultKeyboardLayout
	}
	if cfg.GraphicsMode == "" {
		cfg.GraphicsMode = "auto"
	}
	return cfg
}

func withGraphicsMode(cfg Config, mode string) Config {
	cfg.GraphicsMode = mode
	return cfg
}

func startupGraphicsModes(mode string) []string {
	switch normalizeGraphicsMode(mode) {
	case "bitmap":
		return []string{"bitmap"}
	case "gfx":
		return []string{"gfx"}
	case "avc":
		return []string{"avc"}
	default:
		return []string{"avc", "gfx", "bitmap"}
	}
}

func graphicsModeSettings(mode string) (graphicsPipeline bool, gfxH264 bool, gfxAVC444 bool) {
	switch normalizeGraphicsMode(mode) {
	case "bitmap":
		return false, false, false
	case "gfx":
		return true, false, false
	default:
		return true, true, true
	}
}

func normalizeGraphicsMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		return "auto"
	case "avc", "h264":
		return "avc"
	case "gfx", "graphics":
		return "gfx"
	case "bitmap", "legacy":
		return "bitmap"
	default:
		return "auto"
	}
}

func cStrings(cfg Config) (host, username, password, domain *C.char, freeAll func()) {
	host = C.CString(cfg.Host)
	username = C.CString(cfg.Username)
	password = C.CString(cfg.Password)
	domain = C.CString(cfg.Domain)
	freeAll = func() {
		C.free(unsafe.Pointer(host))
		C.free(unsafe.Pointer(username))
		C.free(unsafe.Pointer(password))
		C.free(unsafe.Pointer(domain))
	}
	return
}

func newConfiguredClient(cfg Config, authOnly bool) (*C.gofreerdp_client, error) {
	client := C.gofreerdp_client_new_instance()
	if client == nil {
		return nil, errors.New("failed to allocate freerdp client")
	}

	host, username, password, domain, freeAll := cStrings(cfg)
	defer freeAll()
	graphicsPipeline, gfxH264, gfxAVC444 := graphicsModeSettings(cfg.GraphicsMode)

	if C.gofreerdp_configure(
		client,
		host,
		C.uint16_t(cfg.Port),
		username,
		password,
		domain,
		C.uint32_t(cfg.DesktopWidth),
		C.uint32_t(cfg.DesktopHeight),
		C.uint32_t(cfg.KeyboardLayout),
		cBool(cfg.InsecureSkipVerify),
		cBool(authOnly),
		cBool(graphicsPipeline),
		cBool(gfxH264),
		cBool(gfxAVC444),
	) == 0 {
		err := fmt.Errorf("freerdp configure failed: %s", goErrorString(client))
		C.gofreerdp_client_free_instance(client)
		return nil, err
	}

	return client, nil
}

func waitForUsableStartupSnapshot(session *Session, timeout time.Duration) (bool, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(startupPollInterval)
	defer ticker.Stop()

	blankCount := 0
	for {
		select {
		case <-session.Done():
			if err := session.Err(); err != nil {
				return false, err
			}
			return false, errors.New("session ended before first usable snapshot")
		case <-deadline.C:
			status := session.Status()
			return false, fmt.Errorf("no usable startup snapshot within %s (state=%s ready=%t size=%dx%d)", timeout, status.State, status.Ready, status.Width, status.Height)
		case <-ticker.C:
			status := session.Status()
			if !status.Ready {
				continue
			}

			pngData, err := session.ScreenshotPNG()
			if err != nil {
				continue
			}
			blank, err := screenshotLooksBlank(pngData)
			if err != nil {
				return false, err
			}
			if !blank {
				return true, nil
			}
			blankCount++
			if blankCount >= blankSnapshotLimit {
				return false, errors.New("startup snapshot stayed blank while graphics pipeline was active")
			}
		}
	}
}

func screenshotLooksBlank(pngData []byte) (bool, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return false, fmt.Errorf("decode startup screenshot: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return true, nil
	}

	samples := 0
	whiteSamples := 0
	stepX := maxInt(bounds.Dx()/12, 1)
	stepY := maxInt(bounds.Dy()/12, 1)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			r, g, b, a := img.At(x, y).RGBA()
			samples++
			if a >= 0xF000 && r >= 0xF000 && g >= 0xF000 && b >= 0xF000 {
				whiteSamples++
			}
		}
	}
	return samples > 0 && whiteSamples == samples, nil
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func goErrorString(client *C.gofreerdp_client) string {
	if client == nil {
		return "unknown error"
	}
	msg := C.gofreerdp_error(client)
	if msg == nil || C.GoString(msg) == "" {
		return "unknown error"
	}
	return C.GoString(msg)
}

func cBool(v bool) C.BOOL {
	if v {
		return C.TRUE
	}
	return C.FALSE
}

func normalizeKeyName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	if alias, ok := keyNameAliases[strings.ToLower(trimmed)]; ok {
		return alias
	}
	if len(trimmed) == 1 {
		return strings.ToUpper(trimmed)
	}

	upper := strings.ToUpper(trimmed)
	return strings.TrimPrefix(upper, "VK_")
}

func lookupKeyScancode(name string) (uint32, bool) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	scancode := uint32(C.gofreerdp_rdp_scancode_from_name(cname))
	if scancode != 0 {
		return scancode, true
	}
	scancode, ok := keyNameScancodes[name]
	return scancode, ok
}

func validateMousePosition(x int, y int) (C.uint16_t, C.uint16_t, error) {
	if x < 0 || x > 0xFFFF {
		return 0, 0, fmt.Errorf("x must be between 0 and %d", 0xFFFF)
	}
	if y < 0 || y > 0xFFFF {
		return 0, 0, fmt.Errorf("y must be between 0 and %d", 0xFFFF)
	}
	return C.uint16_t(x), C.uint16_t(y), nil
}

func mouseButtonFlags(button string, down bool) (C.uint16_t, bool, error) {
	key := strings.ToLower(strings.TrimSpace(button))
	switch key {
	case "left":
		return mouseFlags(C.PTR_FLAGS_BUTTON1, down), false, nil
	case "right":
		return mouseFlags(C.PTR_FLAGS_BUTTON2, down), false, nil
	case "middle":
		return mouseFlags(C.PTR_FLAGS_BUTTON3, down), false, nil
	case "x1", "back":
		return mouseFlags(C.PTR_XFLAGS_BUTTON1, down), true, nil
	case "x2", "forward":
		return mouseFlags(C.PTR_XFLAGS_BUTTON2, down), true, nil
	default:
		return 0, false, fmt.Errorf("unsupported mouse button: %s", button)
	}
}

func mouseFlags(base C.int, down bool) C.uint16_t {
	flags := C.uint16_t(base)
	if down {
		if base == C.PTR_XFLAGS_BUTTON1 || base == C.PTR_XFLAGS_BUTTON2 {
			flags |= C.uint16_t(C.PTR_XFLAGS_DOWN)
		} else {
			flags |= C.uint16_t(C.PTR_FLAGS_DOWN)
		}
	}
	return flags
}

var keyNameAliases = map[string]string{
	"esc":         "ESCAPE",
	"escape":      "ESCAPE",
	"enter":       "RETURN",
	"return":      "RETURN",
	"ctrl":        "LCONTROL",
	"control":     "LCONTROL",
	"lctrl":       "LCONTROL",
	"rctrl":       "RCONTROL",
	"alt":         "LMENU",
	"lalt":        "LMENU",
	"ralt":        "RMENU",
	"shift":       "LSHIFT",
	"lshift":      "LSHIFT",
	"rshift":      "RSHIFT",
	"super":       "LWIN",
	"win":         "LWIN",
	"meta":        "LWIN",
	"cmd":         "LWIN",
	"menu":        "APPS",
	"apps":        "APPS",
	"space":       "SPACE",
	"spacebar":    "SPACE",
	"backspace":   "BACK",
	"tab":         "TAB",
	"capslock":    "CAPITAL",
	"caps":        "CAPITAL",
	"numlock":     "NUMLOCK",
	"scrolllock":  "SCROLL",
	"printscreen": "SNAPSHOT",
	"prtsc":       "SNAPSHOT",
	"insert":      "INSERT",
	"delete":      "DELETE",
	"del":         "DELETE",
	"home":        "HOME",
	"end":         "END",
	"pageup":      "PRIOR",
	"pgup":        "PRIOR",
	"pagedown":    "NEXT",
	"pgdn":        "NEXT",
	"left":        "LEFT",
	"right":       "RIGHT",
	"up":          "UP",
	"down":        "DOWN",
	"-":           "OEM_MINUS",
	"=":           "OEM_PLUS",
	"[":           "OEM_4",
	"]":           "OEM_6",
	";":           "OEM_1",
	"'":           "OEM_7",
	"`":           "OEM_3",
	"\\":          "OEM_5",
	",":           "OEM_COMMA",
	".":           "OEM_PERIOD",
	"/":           "OEM_2",
}

var keyNameScancodes = map[string]uint32{
	"0":          uint32(C.RDP_SCANCODE_KEY_0),
	"1":          uint32(C.RDP_SCANCODE_KEY_1),
	"2":          uint32(C.RDP_SCANCODE_KEY_2),
	"3":          uint32(C.RDP_SCANCODE_KEY_3),
	"4":          uint32(C.RDP_SCANCODE_KEY_4),
	"5":          uint32(C.RDP_SCANCODE_KEY_5),
	"6":          uint32(C.RDP_SCANCODE_KEY_6),
	"7":          uint32(C.RDP_SCANCODE_KEY_7),
	"8":          uint32(C.RDP_SCANCODE_KEY_8),
	"9":          uint32(C.RDP_SCANCODE_KEY_9),
	"A":          uint32(C.RDP_SCANCODE_KEY_A),
	"B":          uint32(C.RDP_SCANCODE_KEY_B),
	"C":          uint32(C.RDP_SCANCODE_KEY_C),
	"D":          uint32(C.RDP_SCANCODE_KEY_D),
	"E":          uint32(C.RDP_SCANCODE_KEY_E),
	"F":          uint32(C.RDP_SCANCODE_KEY_F),
	"G":          uint32(C.RDP_SCANCODE_KEY_G),
	"H":          uint32(C.RDP_SCANCODE_KEY_H),
	"I":          uint32(C.RDP_SCANCODE_KEY_I),
	"J":          uint32(C.RDP_SCANCODE_KEY_J),
	"K":          uint32(C.RDP_SCANCODE_KEY_K),
	"L":          uint32(C.RDP_SCANCODE_KEY_L),
	"M":          uint32(C.RDP_SCANCODE_KEY_M),
	"N":          uint32(C.RDP_SCANCODE_KEY_N),
	"O":          uint32(C.RDP_SCANCODE_KEY_O),
	"P":          uint32(C.RDP_SCANCODE_KEY_P),
	"Q":          uint32(C.RDP_SCANCODE_KEY_Q),
	"R":          uint32(C.RDP_SCANCODE_KEY_R),
	"S":          uint32(C.RDP_SCANCODE_KEY_S),
	"T":          uint32(C.RDP_SCANCODE_KEY_T),
	"U":          uint32(C.RDP_SCANCODE_KEY_U),
	"V":          uint32(C.RDP_SCANCODE_KEY_V),
	"W":          uint32(C.RDP_SCANCODE_KEY_W),
	"X":          uint32(C.RDP_SCANCODE_KEY_X),
	"Y":          uint32(C.RDP_SCANCODE_KEY_Y),
	"Z":          uint32(C.RDP_SCANCODE_KEY_Z),
	"ESCAPE":     uint32(C.RDP_SCANCODE_ESCAPE),
	"RETURN":     uint32(C.RDP_SCANCODE_RETURN),
	"BACK":       uint32(C.RDP_SCANCODE_BACKSPACE),
	"TAB":        uint32(C.RDP_SCANCODE_TAB),
	"SPACE":      uint32(C.RDP_SCANCODE_SPACE),
	"LSHIFT":     uint32(C.RDP_SCANCODE_LSHIFT),
	"RSHIFT":     uint32(C.RDP_SCANCODE_RSHIFT),
	"LCONTROL":   uint32(C.RDP_SCANCODE_LCONTROL),
	"RCONTROL":   uint32(C.RDP_SCANCODE_RCONTROL),
	"LMENU":      uint32(C.RDP_SCANCODE_LMENU),
	"RMENU":      uint32(C.RDP_SCANCODE_RMENU),
	"CAPITAL":    uint32(C.RDP_SCANCODE_CAPSLOCK),
	"NUMLOCK":    uint32(C.RDP_SCANCODE_NUMLOCK),
	"SCROLL":     uint32(C.RDP_SCANCODE_SCROLLLOCK),
	"SNAPSHOT":   uint32(C.RDP_SCANCODE_PRINTSCREEN),
	"INSERT":     uint32(C.RDP_SCANCODE_INSERT),
	"DELETE":     uint32(C.RDP_SCANCODE_DELETE),
	"F1":         uint32(C.RDP_SCANCODE_F1),
	"F2":         uint32(C.RDP_SCANCODE_F2),
	"F3":         uint32(C.RDP_SCANCODE_F3),
	"F4":         uint32(C.RDP_SCANCODE_F4),
	"F5":         uint32(C.RDP_SCANCODE_F5),
	"F6":         uint32(C.RDP_SCANCODE_F6),
	"F7":         uint32(C.RDP_SCANCODE_F7),
	"F8":         uint32(C.RDP_SCANCODE_F8),
	"F9":         uint32(C.RDP_SCANCODE_F9),
	"F10":        uint32(C.RDP_SCANCODE_F10),
	"F11":        uint32(C.RDP_SCANCODE_F11),
	"F12":        uint32(C.RDP_SCANCODE_F12),
	"HOME":       uint32(C.RDP_SCANCODE_HOME),
	"END":        uint32(C.RDP_SCANCODE_END),
	"PRIOR":      uint32(C.RDP_SCANCODE_PRIOR),
	"NEXT":       uint32(C.RDP_SCANCODE_NEXT),
	"LEFT":       uint32(C.RDP_SCANCODE_LEFT),
	"RIGHT":      uint32(C.RDP_SCANCODE_RIGHT),
	"UP":         uint32(C.RDP_SCANCODE_UP),
	"DOWN":       uint32(C.RDP_SCANCODE_DOWN),
	"LWIN":       uint32(C.RDP_SCANCODE_LWIN),
	"RWIN":       uint32(C.RDP_SCANCODE_RWIN),
	"APPS":       uint32(C.RDP_SCANCODE_APPS),
	"OEM_MINUS":  uint32(C.RDP_SCANCODE_OEM_MINUS),
	"OEM_PLUS":   uint32(C.RDP_SCANCODE_OEM_PLUS),
	"OEM_4":      uint32(C.RDP_SCANCODE_OEM_4),
	"OEM_6":      uint32(C.RDP_SCANCODE_OEM_6),
	"OEM_1":      uint32(C.RDP_SCANCODE_OEM_1),
	"OEM_7":      uint32(C.RDP_SCANCODE_OEM_7),
	"OEM_3":      uint32(C.RDP_SCANCODE_OEM_3),
	"OEM_5":      uint32(C.RDP_SCANCODE_OEM_5),
	"OEM_COMMA":  uint32(C.RDP_SCANCODE_OEM_COMMA),
	"OEM_PERIOD": uint32(C.RDP_SCANCODE_OEM_PERIOD),
	"OEM_2":      uint32(C.RDP_SCANCODE_OEM_2),
}

func asciiKeyNameForRune(r rune) (string, bool, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return string(r - 'a' + 'A'), false, true
	case r >= 'A' && r <= 'Z':
		return string(r), true, true
	case r >= '0' && r <= '9':
		return string(r), false, true
	}

	switch r {
	case ' ':
		return "SPACE", false, true
	case '\n', '\r':
		return "RETURN", false, true
	case '\t':
		return "TAB", false, true
	case '-':
		return "OEM_MINUS", false, true
	case '_':
		return "OEM_MINUS", true, true
	case '=':
		return "OEM_PLUS", false, true
	case '+':
		return "OEM_PLUS", true, true
	case '[':
		return "OEM_4", false, true
	case '{':
		return "OEM_4", true, true
	case ']':
		return "OEM_6", false, true
	case '}':
		return "OEM_6", true, true
	case ';':
		return "OEM_1", false, true
	case ':':
		return "OEM_1", true, true
	case '\'':
		return "OEM_7", false, true
	case '"':
		return "OEM_7", true, true
	case '`':
		return "OEM_3", false, true
	case '~':
		return "OEM_3", true, true
	case '\\':
		return "OEM_5", false, true
	case '|':
		return "OEM_5", true, true
	case ',':
		return "OEM_COMMA", false, true
	case '<':
		return "OEM_COMMA", true, true
	case '.':
		return "OEM_PERIOD", false, true
	case '>':
		return "OEM_PERIOD", true, true
	case '/':
		return "OEM_2", false, true
	case '?':
		return "OEM_2", true, true
	case '!':
		return "1", true, true
	case '@':
		return "2", true, true
	case '#':
		return "3", true, true
	case '$':
		return "4", true, true
	case '%':
		return "5", true, true
	case '^':
		return "6", true, true
	case '&':
		return "7", true, true
	case '*':
		return "8", true, true
	case '(':
		return "9", true, true
	case ')':
		return "0", true, true
	default:
		return "", false, false
	}
}
