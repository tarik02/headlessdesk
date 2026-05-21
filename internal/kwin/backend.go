//go:build linux

package kwin

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"

	"headlessdesk/internal/desktop"
)

const (
	serviceName   = "org.kde.KWin.ScreenShot2"
	objectPath    = "/org/kde/KWin/ScreenShot2"
	interfaceName = "org.kde.KWin.ScreenShot2"
)

type Backend struct {
	conn      *dbus.Conn
	obj       dbus.BusObject
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	status    desktop.Status
	runErr    error
}

func New() (*Backend, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect session bus: %w", err)
	}

	b := &Backend{
		conn: conn,
		obj:  conn.Object(serviceName, dbus.ObjectPath(objectPath)),
		done: make(chan struct{}),
		status: desktop.Status{
			Protocol:  "kwin",
			Connected: true,
			Active:    true,
			Ready:     true,
			State:     "READY",
		},
	}
	return b, nil
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
	raw, err := b.capture(interfaceName+".CaptureWorkspace", map[string]dbus.Variant{})
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	img, err := raw.toImage()
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	b.setFrameSize(raw.width, raw.height)
	b.clearErr()
	return img, nil
}

func (b *Backend) ScreenshotCrop(crop desktop.Crop) (image.Image, error) {
	if crop.X == nil || crop.Y == nil || crop.W == nil || crop.H == nil {
		return nil, desktop.ErrCroppedOutputUnavailable
	}
	x := *crop.X
	y := *crop.Y
	w := *crop.W
	h := *crop.H
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("crop width and height must be greater than zero")
	}

	raw, err := b.capture(interfaceName+".CaptureArea", int32(x), int32(y), uint32(w), uint32(h), map[string]dbus.Variant{})
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	img, err := raw.toImage()
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	b.clearErr()
	return img, nil
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
		b.conn.Close()
		close(b.done)
	})
	return nil
}

func (b *Backend) capture(method string, args ...any) (rawImage, error) {
	readFile, writeFile, err := os.Pipe()
	if err != nil {
		return rawImage{}, fmt.Errorf("create screenshot pipe: %w", err)
	}
	defer readFile.Close()

	readCh := make(chan readResult, 1)
	go func() {
		out, err := io.ReadAll(readFile)
		readCh <- readResult{data: out, err: err}
	}()

	args = append(args, dbus.UnixFD(writeFile.Fd()))
	var result map[string]dbus.Variant
	call := b.obj.Call(method, 0, args...)
	closeErr := writeFile.Close()
	if call.Err != nil {
		<-readCh
		return rawImage{}, fmt.Errorf("kwin screenshot call: %w", call.Err)
	}
	if closeErr != nil {
		<-readCh
		return rawImage{}, fmt.Errorf("close screenshot pipe writer: %w", closeErr)
	}
	if err := call.Store(&result); err != nil {
		<-readCh
		return rawImage{}, fmt.Errorf("decode kwin screenshot response: %w", err)
	}

	read := <-readCh
	if read.err != nil {
		return rawImage{}, fmt.Errorf("read screenshot pipe: %w", read.err)
	}
	if len(read.data) == 0 {
		return rawImage{}, errors.New("kwin screenshot returned empty image")
	}

	raw, err := rawImageFromResult(result, read.data)
	if err != nil {
		return rawImage{}, err
	}
	return raw, nil
}

func (b *Backend) setFrameSize(width int, height int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status.Ready = true
	b.status.Width = width
	b.status.Height = height
}

func (b *Backend) setErr(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runErr = err
}

func (b *Backend) clearErr() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runErr = nil
}

func rawImageFromResult(result map[string]dbus.Variant, data []byte) (rawImage, error) {
	width, err := uintVariant(result, "width")
	if err != nil {
		return rawImage{}, err
	}
	height, err := uintVariant(result, "height")
	if err != nil {
		return rawImage{}, err
	}
	stride, err := uintVariant(result, "stride")
	if err != nil {
		return rawImage{}, err
	}
	format, err := uintVariant(result, "format")
	if err != nil {
		return rawImage{}, err
	}
	if width == 0 || height == 0 || stride == 0 {
		return rawImage{}, fmt.Errorf("invalid kwin image metadata: width=%d height=%d stride=%d", width, height, stride)
	}
	if uint64(len(data)) < uint64(stride)*uint64(height) {
		return rawImage{}, fmt.Errorf("kwin screenshot data too short: got %d bytes for %dx%d stride %d", len(data), width, height, stride)
	}
	return rawImage{width: int(width), height: int(height), stride: int(stride), format: uint32(format), data: data}, nil
}

func uintVariant(result map[string]dbus.Variant, name string) (uint32, error) {
	variant, ok := result[name]
	if !ok {
		return 0, fmt.Errorf("kwin screenshot response missing %q", name)
	}
	switch value := variant.Value().(type) {
	case uint32:
		return value, nil
	case int32:
		if value < 0 {
			return 0, fmt.Errorf("kwin screenshot response %q is negative: %d", name, value)
		}
		return uint32(value), nil
	case int:
		if value < 0 {
			return 0, fmt.Errorf("kwin screenshot response %q is negative: %d", name, value)
		}
		return uint32(value), nil
	case uint:
		return uint32(value), nil
	default:
		return 0, fmt.Errorf("kwin screenshot response %q has unsupported type %T", name, value)
	}
}

type rawImage struct {
	width  int
	height int
	stride int
	format uint32
	data   []byte
}

func (r rawImage) toImage() (image.Image, error) {
	bpp, err := bytesPerPixel(r.format)
	if err != nil {
		return nil, err
	}
	if r.stride < r.width*bpp {
		return nil, fmt.Errorf("kwin screenshot stride %d is too small for width %d and format %d", r.stride, r.width, r.format)
	}

	img := image.NewNRGBA(image.Rect(0, 0, r.width, r.height))
	for y := 0; y < r.height; y++ {
		row := r.data[y*r.stride:]
		for x := 0; x < r.width; x++ {
			c, err := r.pixel(row, x)
			if err != nil {
				return nil, err
			}
			img.SetNRGBA(x, y, c)
		}
	}

	return img, nil
}

func (r rawImage) pixel(row []byte, x int) (color.NRGBA, error) {
	switch r.format {
	case 4:
		i := x * 4
		return color.NRGBA{R: row[i+2], G: row[i+1], B: row[i], A: 0xff}, nil
	case 5:
		i := x * 4
		return color.NRGBA{R: row[i+2], G: row[i+1], B: row[i], A: row[i+3]}, nil
	case 6:
		i := x * 4
		return unpremultiply(row[i+2], row[i+1], row[i], row[i+3]), nil
	case 13:
		i := x * 3
		return color.NRGBA{R: row[i], G: row[i+1], B: row[i+2], A: 0xff}, nil
	case 16:
		i := x * 4
		return color.NRGBA{R: row[i], G: row[i+1], B: row[i+2], A: 0xff}, nil
	case 17:
		i := x * 4
		return color.NRGBA{R: row[i], G: row[i+1], B: row[i+2], A: row[i+3]}, nil
	case 18:
		i := x * 4
		return unpremultiply(row[i], row[i+1], row[i+2], row[i+3]), nil
	default:
		return color.NRGBA{}, fmt.Errorf("unsupported kwin QImage format: %d", r.format)
	}
}

func bytesPerPixel(format uint32) (int, error) {
	switch format {
	case 4, 5, 6, 16, 17, 18:
		return 4, nil
	case 13:
		return 3, nil
	default:
		return 0, fmt.Errorf("unsupported kwin QImage format: %d", format)
	}
}

func unpremultiply(r byte, g byte, b byte, a byte) color.NRGBA {
	if a == 0 || a == 0xff {
		return color.NRGBA{R: r, G: g, B: b, A: a}
	}
	return color.NRGBA{
		R: byte(min(255, int(r)*255/int(a))),
		G: byte(min(255, int(g)*255/int(a))),
		B: byte(min(255, int(b)*255/int(a))),
		A: a,
	}
}

type readResult struct {
	data []byte
	err  error
}
