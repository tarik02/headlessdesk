package commandbackend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

const (
	defaultTimeout = 30 * time.Second
	stderrLimit    = 4096
)

type Command struct {
	Argv    []string
	Script  string
	Timeout time.Duration
}

type Config struct {
	Timeout        time.Duration
	SSH            *SSHConfig
	Screenshot     Command
	ScreenshotCrop Command
	MoveMouse      Command
	MouseButton    Command
	MouseWheel     Command
	Key            Command
	KeyScancode    Command
	TypeText       Command
}

type Backend struct {
	cfg       Config
	sshClient *sshClient
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	status    desktop.Status
	runErr    error
}

func New(cfg Config) (*Backend, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	var client *sshClient
	var err error
	if cfg.SSH != nil {
		client, err = newSSHClient(*cfg.SSH)
		if err != nil {
			return nil, err
		}
	}

	protocol := "command"
	if client != nil {
		protocol = "command+ssh"
	}
	return &Backend{
		cfg:       cfg,
		sshClient: client,
		done:      make(chan struct{}),
		status: desktop.Status{
			Protocol:  protocol,
			Connected: true,
			Active:    true,
			Ready:     commandBackendReady(cfg),
			State:     "READY",
		},
	}, nil
}

func (b *Backend) Status() desktop.Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	status := b.status
	if b.runErr != nil {
		status.Error = b.runErr.Error()
	}
	return status
}

func (b *Backend) Screenshot() (image.Image, error) {
	out, err := b.run(b.cfg.Screenshot, nil)
	if err != nil {
		return nil, err
	}
	img, err := decodePNG(out)
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	b.setFrameSize(img.Bounds().Dx(), img.Bounds().Dy())
	return img, nil
}

func (b *Backend) ScreenshotCrop(crop desktop.Crop) (image.Image, error) {
	if !b.cfg.ScreenshotCrop.Configured() {
		return nil, desktop.ErrCroppedOutputUnavailable
	}
	if crop.X == nil || crop.Y == nil || crop.W == nil || crop.H == nil {
		return nil, desktop.ErrCroppedOutputUnavailable
	}
	out, err := b.run(b.cfg.ScreenshotCrop, cropData{
		X: *crop.X,
		Y: *crop.Y,
		W: *crop.W,
		H: *crop.H,
	})
	if err != nil {
		return nil, err
	}
	img, err := decodePNG(out)
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	return img, nil
}

func (b *Backend) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	_, err := b.run(b.cfg.Key, keyData{Key: name.String(), Down: down, Repeat: repeat})
	return err
}

func (b *Backend) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	if !b.cfg.KeyScancode.Configured() {
		return fmt.Errorf("command key_scancode is not configured: %d", scancode)
	}
	_, err := b.run(b.cfg.KeyScancode, scancodeData{Scancode: scancode.Uint32(), Down: down, Repeat: repeat})
	return err
}

func (b *Backend) TypeText(text string) error {
	_, err := b.run(b.cfg.TypeText, typeTextData{Text: text})
	return err
}

func (b *Backend) MoveMouse(x float64, y float64) error {
	_, err := b.run(b.cfg.MoveMouse, pointData{X: x, Y: y})
	return err
}

func (b *Backend) SendMouseButton(button inputcode.MouseButtonName, x float64, y float64, down bool) error {
	_, err := b.run(b.cfg.MouseButton, buttonData{Button: button.String(), X: x, Y: y, Down: down})
	return err
}

func (b *Backend) SendMouseWheel(x float64, y float64, delta int, horizontal bool) error {
	_, err := b.run(b.cfg.MouseWheel, wheelData{X: x, Y: y, Delta: delta, Horizontal: horizontal})
	return err
}

func (b *Backend) Done() <-chan struct{} {
	return b.done
}

func (b *Backend) Err() error {
	return nil
}

func (b *Backend) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.status.Connected = false
		b.status.Active = false
		b.status.State = "CLOSED"
		b.mu.Unlock()
		if b.sshClient != nil {
			_ = b.sshClient.Close()
		}
		close(b.done)
	})
	return nil
}

func (b *Backend) run(command Command, data any) ([]byte, error) {
	rendered, err := renderCommand(command, data)
	if err != nil {
		b.setErr(err)
		return nil, err
	}

	timeout := command.Timeout
	if timeout <= 0 {
		timeout = b.cfg.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var out []byte
	if rendered.script {
		if b.sshClient != nil {
			out, err = b.runSSHScript(ctx, timeout, rendered.value)
		} else {
			out, err = b.runLocalScript(ctx, timeout, rendered.value)
		}
	} else {
		if b.sshClient != nil {
			out, err = b.runSSH(ctx, timeout, rendered.argv)
		} else {
			out, err = b.runLocal(ctx, timeout, rendered.argv)
		}
	}
	if err != nil {
		return nil, err
	}
	b.clearErr()
	return out, nil
}

func (c Command) Configured() bool {
	return len(c.Argv) > 0 || strings.TrimSpace(c.Script) != ""
}

func (b *Backend) runLocal(ctx context.Context, timeout time.Duration, argv []string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("command timed out after %s", timeout)
		}
		if stderr.Len() > 0 {
			err = fmt.Errorf("%w: %s", err, truncate(stderr.String(), stderrLimit))
		}
		b.setErr(err)
		return nil, err
	}
	return stdout.Bytes(), nil
}

func (b *Backend) runLocalScript(ctx context.Context, timeout time.Duration, script string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("command timed out after %s", timeout)
		}
		if stderr.Len() > 0 {
			err = fmt.Errorf("%w: %s", err, truncate(stderr.String(), stderrLimit))
		}
		b.setErr(err)
		return nil, err
	}
	return stdout.Bytes(), nil
}

func (b *Backend) runSSH(ctx context.Context, timeout time.Duration, argv []string) ([]byte, error) {
	stdout, stderr, err := b.sshClient.Run(ctx, shellQuoteArgv(argv), nil)
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("command timed out after %s", timeout)
		}
		if stderr != "" {
			err = fmt.Errorf("%w: %s", err, truncate(stderr, stderrLimit))
		}
		b.setErr(err)
		return nil, err
	}
	return stdout, nil
}

func (b *Backend) runSSHScript(ctx context.Context, timeout time.Duration, script string) ([]byte, error) {
	stdout, stderr, err := b.sshClient.Run(ctx, "sh -s", []byte(script))
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("command timed out after %s", timeout)
		}
		if stderr != "" {
			err = fmt.Errorf("%w: %s", err, truncate(stderr, stderrLimit))
		}
		b.setErr(err)
		return nil, err
	}
	return stdout, nil
}

type renderedCommand struct {
	argv   []string
	value  string
	script bool
}

func renderCommand(command Command, data any) (renderedCommand, error) {
	if len(command.Argv) > 0 && strings.TrimSpace(command.Script) != "" {
		return renderedCommand{}, errors.New("command cannot define both argv and script")
	}
	if len(command.Argv) > 0 {
		argv, err := renderArgv(command.Argv, data)
		return renderedCommand{argv: argv}, err
	}
	if strings.TrimSpace(command.Script) != "" {
		script, err := renderTemplate("script", command.Script, data)
		return renderedCommand{value: script, script: true}, err
	}
	return renderedCommand{}, errors.New("command argv or script is required")
}

func renderArgv(argv []string, data any) ([]string, error) {
	if len(argv) == 0 {
		return nil, errors.New("command argv is required")
	}
	rendered := make([]string, 0, len(argv))
	for _, arg := range argv {
		value, err := renderTemplate("arg", arg, data)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, value)
	}
	return rendered, nil
}

func renderTemplate(name string, value string, data any) (string, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs()).Option("missingkey=error").Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse command %s template: %w", name, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render command %s template: %w", name, err)
	}
	return out.String(), nil
}

func templateFuncs() template.FuncMap {
	funcs := sprig.TxtFuncMap()
	funcs["ydotoolButton"] = ydotoolButton
	funcs["ydotoolButtonEvent"] = ydotoolButtonEvent
	funcs["ydotoolWheelX"] = ydotoolWheelX
	funcs["ydotoolWheelY"] = ydotoolWheelY
	funcs["ydotoolKey"] = ydotoolKey
	funcs["ydotoolKeyEvent"] = ydotoolKeyEvent
	return funcs
}

func decodePNG(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode command screenshot png: %w", err)
	}
	return img, nil
}

func (b *Backend) setFrameSize(width int, height int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status.Ready = true
	b.status.Width = width
	b.status.Height = height
	b.status.Error = ""
	b.runErr = nil
}

func (b *Backend) setErr(err error) {
	if err == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runErr = err
	b.status.Error = err.Error()
}

func commandBackendReady(cfg Config) bool {
	return cfg.Screenshot.Configured() ||
		(cfg.MoveMouse.Configured() &&
			cfg.MouseButton.Configured() &&
			cfg.MouseWheel.Configured() &&
			cfg.Key.Configured() &&
			cfg.TypeText.Configured())
}

func (b *Backend) clearErr() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runErr = nil
	b.status.Error = ""
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

type cropData struct {
	X int
	Y int
	W int
	H int
}

type pointData struct {
	X float64
	Y float64
}

type buttonData struct {
	Button string
	X      float64
	Y      float64
	Down   bool
}

type wheelData struct {
	X          float64
	Y          float64
	Delta      int
	Horizontal bool
}

type keyData struct {
	Key    string
	Down   bool
	Repeat bool
}

type scancodeData struct {
	Scancode uint32
	Down     bool
	Repeat   bool
}

type typeTextData struct {
	Text string
}
