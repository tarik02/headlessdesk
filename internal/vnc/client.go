package vnc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	kwardvnc "github.com/kward/go-vnc"
	"github.com/kward/go-vnc/buttons"
	"github.com/kward/go-vnc/keys"
	"github.com/kward/go-vnc/rfbflags"

	"libfreerdp-golang-poc/internal/desktop"
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
	connectTimeout              = 10 * time.Second
	frameRequestInterval        = 100 * time.Millisecond
)

// Session is a VNC/RFB-backed remote desktop session.
type Session struct {
	conn     *kwardvnc.ClientConn
	messages chan kwardvnc.ServerMessage
	done     chan struct{}

	mu        sync.RWMutex
	inputMu   sync.Mutex
	closeOnce sync.Once
	runErr    error
	status    desktop.Status
	frame     *image.NRGBA
}

// StartSession connects to a VNC server using github.com/kward/go-vnc and
// starts the framebuffer/message pump required by the shared desktop.Session
// interface.
func StartSession(cfg Config) (desktop.Session, error) {
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))
	dialer := net.Dialer{Timeout: connectTimeout}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connect to VNC server %s: %w", addr, err)
	}

	messageCh := make(chan kwardvnc.ServerMessage, 16)
	clientCfg := kwardvnc.NewClientConfig(cfg.Password)
	clientCfg.Exclusive = !cfg.Shared
	clientCfg.ServerMessageCh = messageCh

	client, err := kwardvnc.Connect(ctx, netConn, clientCfg)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("negotiate VNC session: %w", err)
	}

	if err := client.SetEncodings(kwardvnc.Encodings{
		&kwardvnc.RawEncoding{},
		&kwardvnc.DesktopSizePseudoEncoding{},
	}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("configure VNC encodings: %w", err)
	}

	s := &Session{
		conn:     client,
		messages: messageCh,
		done:     make(chan struct{}),
		status: desktop.Status{
			Protocol:  "vnc",
			Connected: true,
			Active:    true,
			State:     "CONNECTED",
			Version:   client.DesktopName(),
			Width:     int(client.FramebufferWidth()),
			Height:    int(client.FramebufferHeight()),
		},
		frame: image.NewNRGBA(image.Rect(0, 0, int(client.FramebufferWidth()), int(client.FramebufferHeight()))),
	}

	go s.run()
	_ = s.requestFrame(false)
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
	if cfg.ViewOnly {
		return fmt.Errorf("vnc view_only cannot satisfy control APIs that require input")
	}
	return nil
}

func (s *Session) run() {
	defer close(s.done)

	listenErrCh := make(chan error, 1)
	go func() {
		listenErrCh <- s.conn.ListenAndHandle()
	}()

	ticker := time.NewTicker(frameRequestInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-s.messages:
			if !ok {
				s.setDisconnected()
				return
			}
			s.handleServerMessage(msg)
		case <-ticker.C:
			if err := s.requestFrame(true); err != nil {
				s.setErr(err)
				s.setDisconnected()
				return
			}
		case err := <-listenErrCh:
			if err != nil {
				s.setErr(fmt.Errorf("VNC read loop ended: %w", err))
			}
			s.setDisconnected()
			return
		}
	}
}

func (s *Session) handleServerMessage(msg kwardvnc.ServerMessage) {
	switch update := msg.(type) {
	case *kwardvnc.FramebufferUpdate:
		s.applyFramebufferUpdate(update)
	}
}

func (s *Session) applyFramebufferUpdate(update *kwardvnc.FramebufferUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rect := range update.Rects {
		s.ensureFrameSizeLocked(int(s.conn.FramebufferWidth()), int(s.conn.FramebufferHeight()))
		raw, ok := rect.Enc.(*kwardvnc.RawEncoding)
		if !ok {
			continue
		}
		s.copyRawRectLocked(rect, raw)
	}

	s.status.Ready = s.frame != nil && !s.frame.Bounds().Empty()
	s.status.Width = s.frame.Bounds().Dx()
	s.status.Height = s.frame.Bounds().Dy()
	s.status.State = "ACTIVE"
}

func (s *Session) ensureFrameSizeLocked(width int, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	if s.frame != nil && s.frame.Bounds().Dx() == width && s.frame.Bounds().Dy() == height {
		return
	}
	s.frame = image.NewNRGBA(image.Rect(0, 0, width, height))
}

func (s *Session) copyRawRectLocked(rect kwardvnc.Rectangle, raw *kwardvnc.RawEncoding) {
	if s.frame == nil {
		return
	}
	for y := 0; y < int(rect.Height); y++ {
		for x := 0; x < int(rect.Width); x++ {
			src := y*int(rect.Width) + x
			if src >= len(raw.Colors) {
				return
			}
			dx := int(rect.X) + x
			dy := int(rect.Y) + y
			if !image.Pt(dx, dy).In(s.frame.Bounds()) {
				continue
			}
			dst := s.frame.PixOffset(dx, dy)
			color := raw.Colors[src]
			s.frame.Pix[dst+0] = scaleColor(color.R)
			s.frame.Pix[dst+1] = scaleColor(color.G)
			s.frame.Pix[dst+2] = scaleColor(color.B)
			s.frame.Pix[dst+3] = 0xff
		}
	}
}

func scaleColor(v uint16) byte {
	if v <= 0xff {
		return byte(v)
	}
	return byte(v >> 8)
}

func (s *Session) requestFrame(incremental bool) error {
	flag := rfbflags.RFBFalse
	if incremental {
		flag = rfbflags.RFBTrue
	}
	return s.withInputLock(func() error {
		return s.conn.FramebufferUpdateRequest(flag, 0, 0, s.conn.FramebufferWidth(), s.conn.FramebufferHeight())
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

func (s *Session) ScreenshotPNG() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.frame == nil || !s.status.Ready {
		return nil, errors.New("no framebuffer snapshot available yet")
	}

	clone := image.NewNRGBA(s.frame.Bounds())
	copy(clone.Pix, s.frame.Pix)
	var out bytes.Buffer
	if err := png.Encode(&out, clone); err != nil {
		return nil, fmt.Errorf("encode screenshot: %w", err)
	}
	return out.Bytes(), nil
}

func (s *Session) SendKey(name string, down bool, repeat bool) error {
	key, err := keyFromName(name)
	if err != nil {
		return err
	}
	return s.withInputLock(func() error {
		return s.conn.KeyEvent(key, down)
	})
}

func (s *Session) SendKeyScancode(scancode uint32, down bool, repeat bool) error {
	return fmt.Errorf("VNC does not support RDP scancode input: %d", scancode)
}

func (s *Session) TypeText(text string) error {
	if text == "" {
		return errors.New("text is required")
	}
	sequence, err := keys.TextToKeys(text)
	if err != nil {
		return err
	}
	return s.withInputLock(func() error {
		for _, key := range sequence {
			if err := s.conn.KeyEvent(key, kwardvnc.PressKey); err != nil {
				return err
			}
			if err := s.conn.KeyEvent(key, kwardvnc.ReleaseKey); err != nil {
				return err
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
	return s.withInputLock(func() error {
		return s.conn.PointerEvent(buttons.None, mouseX, mouseY)
	})
}

func (s *Session) SendMouseButton(button string, x int, y int, down bool) error {
	mouseX, mouseY, err := validatePointerPosition(x, y)
	if err != nil {
		return err
	}
	mask, err := buttonMask(button)
	if err != nil {
		return err
	}
	if !down {
		mask = buttons.None
	}
	return s.withInputLock(func() error {
		return s.conn.PointerEvent(mask, mouseX, mouseY)
	})
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
		if err := s.conn.PointerEvent(mask, mouseX, mouseY); err != nil {
			return err
		}
		return s.conn.PointerEvent(buttons.None, mouseX, mouseY)
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
		_ = s.conn.Close()
		<-s.done
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

func validatePointerPosition(x int, y int) (uint16, uint16, error) {
	if x < 0 || y < 0 || x > int(^uint16(0)) || y > int(^uint16(0)) {
		return 0, 0, fmt.Errorf("pointer position out of range: %d,%d", x, y)
	}
	return uint16(x), uint16(y), nil
}

func buttonMask(button string) (buttons.Button, error) {
	switch strings.ToLower(strings.TrimSpace(button)) {
	case "left", "button1", "1":
		return buttons.Left, nil
	case "middle", "button2", "2":
		return buttons.Middle, nil
	case "right", "button3", "3":
		return buttons.Right, nil
	case "back", "button8", "8":
		return buttons.Eight, nil
	case "forward", "button9", "9":
		return buttons.Seven, nil
	default:
		return buttons.None, fmt.Errorf("unsupported mouse button: %s", button)
	}
}

func wheelMask(delta int, horizontal bool) buttons.Button {
	if horizontal {
		if delta < 0 {
			return buttons.Six
		}
		return buttons.Seven
	}
	if delta < 0 {
		return buttons.Five
	}
	return buttons.Four
}

func keyFromName(name string) (keys.Key, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 0, errors.New("key name is required")
	}
	trimmedRunes := []rune(trimmed)
	if len(trimmedRunes) == 1 {
		if key, ok := keys.FromRune(trimmedRunes[0]); ok {
			return key, nil
		}
	}
	normalized := strings.ToLower(trimmed)
	if key, ok := namedKeys[normalized]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unsupported key name: %s", name)
}

var namedKeys = map[string]keys.Key{
	"alt":       keys.AltLeft,
	"backspace": keys.BackSpace,
	"capslock":  keys.CapsLock,
	"ctrl":      keys.ControlLeft,
	"control":   keys.ControlLeft,
	"delete":    keys.Delete,
	"down":      keys.Down,
	"end":       keys.End,
	"enter":     keys.Return,
	"esc":       keys.Escape,
	"escape":    keys.Escape,
	"f1":        keys.F1,
	"f2":        keys.F2,
	"f3":        keys.F3,
	"f4":        keys.F4,
	"f5":        keys.F5,
	"f6":        keys.F6,
	"f7":        keys.F7,
	"f8":        keys.F8,
	"f9":        keys.F9,
	"f10":       keys.F10,
	"f11":       keys.F11,
	"f12":       keys.F12,
	"home":      keys.Home,
	"insert":    keys.Insert,
	"left":      keys.Left,
	"pagedown":  keys.PageDown,
	"page_down": keys.PageDown,
	"pageup":    keys.PageUp,
	"page_up":   keys.PageUp,
	"return":    keys.Return,
	"right":     keys.Right,
	"shift":     keys.ShiftLeft,
	"space":     keys.Space,
	"super":     keys.SuperLeft,
	"tab":       keys.Tab,
	"up":        keys.Up,
	"win":       keys.SuperLeft,
	"windows":   keys.SuperLeft,
}
