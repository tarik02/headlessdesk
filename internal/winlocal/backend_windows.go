//go:build windows

package winlocal

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/zzl/go-win32api/v2/win32"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

const (
	protocolName = "windows"

	extendedScancode16Prefix uint32 = 0xe000
	extendedScancode24Prefix uint32 = 0xe00000
	extendedScancode16Mask   uint32 = 0xff00
	extendedScancode24Mask   uint32 = 0xffff0000
	scancodeByteMask         uint32 = 0xff
	scancodeWordMask         uint32 = 0xffff

	screenshotBitDepth = 32
	opaqueAlpha        = 0xff

	absoluteMouseCoordinateMax = 65535
)

var dpiOnce sync.Once

type Backend struct {
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	status    desktop.Status
	runErr    error
}

type screenBounds struct {
	left   int
	top    int
	width  int
	height int
}

type virtualKey struct {
	code     win32.VIRTUAL_KEY
	extended bool
}

func New() (*Backend, error) {
	dpiOnce.Do(func() {
		// Best effort: without DPI awareness Windows can virtualize screen metrics,
		// which breaks screenshot/input coordinate parity on scaled desktops.
		_ = win32.SetProcessDPIAware()
	})

	bounds, err := virtualScreenBounds()
	if err != nil {
		return nil, err
	}

	return &Backend{
		done: make(chan struct{}),
		status: desktop.Status{
			Protocol:  protocolName,
			Connected: true,
			Active:    true,
			Ready:     true,
			State:     "READY",
			Width:     bounds.width,
			Height:    bounds.height,
			Regions: []desktop.Region{{
				X: 0,
				Y: 0,
				W: bounds.width,
				H: bounds.height,
			}},
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
	bounds, err := virtualScreenBounds()
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	img, err := captureScreen(bounds.left, bounds.top, bounds.width, bounds.height)
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	b.setReady(bounds)
	return img, nil
}

func (b *Backend) ScreenshotCrop(crop desktop.Crop) (image.Image, error) {
	bounds, err := virtualScreenBounds()
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	rect, err := cropRect(bounds.width, bounds.height, crop)
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	img, err := captureScreen(bounds.left+rect.Min.X, bounds.top+rect.Min.Y, rect.Dx(), rect.Dy())
	if err != nil {
		b.setErr(err)
		return nil, err
	}
	b.setReady(bounds)
	return img, nil
}

func (b *Backend) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	key, err := inputcode.WindowsVirtualKey(name)
	if err != nil {
		b.setErr(err)
		return err
	}
	if err := sendVirtualKey(virtualKey{code: key.VirtualKey, extended: key.Extended}, down); err != nil {
		b.setErr(err)
		return err
	}
	b.clearErr()
	return nil
}

func (b *Backend) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	value := scancode.Uint32()
	if value > math.MaxUint16 && value&extendedScancode24Mask != extendedScancode24Prefix {
		err := fmt.Errorf("Windows scancode is out of range: %d", value)
		b.setErr(err)
		return err
	}
	extended := value&extendedScancode16Mask == extendedScancode16Prefix ||
		value&extendedScancode24Mask == extendedScancode24Prefix
	scan := uint16(value & scancodeByteMask)
	if scan == 0 {
		scan = uint16(value & scancodeWordMask)
	}
	if err := sendKeyboardInput(win32.KEYBDINPUT{
		WScan:   scan,
		DwFlags: scancodeFlags(down, extended),
	}); err != nil {
		b.setErr(err)
		return err
	}
	b.clearErr()
	return nil
}

func (b *Backend) TypeText(text string) error {
	if text == "" {
		err := errors.New("text is required")
		b.setErr(err)
		return err
	}
	for _, unit := range utf16.Encode([]rune(text)) {
		if err := sendUnicodeUnit(unit, true); err != nil {
			b.setErr(err)
			return err
		}
		if err := sendUnicodeUnit(unit, false); err != nil {
			b.setErr(err)
			return err
		}
	}
	b.clearErr()
	return nil
}

func (b *Backend) MoveMouse(x float64, y float64) error {
	if err := b.setCursorPos(x, y); err != nil {
		b.setErr(err)
		return err
	}
	b.clearErr()
	return nil
}

func (b *Backend) SendMouseButton(button inputcode.MouseButtonName, x float64, y float64, down bool) error {
	if err := b.setCursorPos(x, y); err != nil {
		b.setErr(err)
		return err
	}
	flags, data, err := mouseButtonEvent(button, down)
	if err != nil {
		b.setErr(err)
		return err
	}
	if err := sendMouseInput(win32.MOUSEINPUT{MouseData: data, DwFlags: flags}); err != nil {
		b.setErr(err)
		return err
	}
	b.clearErr()
	return nil
}

func (b *Backend) SendMouseWheel(x float64, y float64, delta int, horizontal bool) error {
	if delta == 0 {
		err := errors.New("wheel delta must be non-zero")
		b.setErr(err)
		return err
	}
	if err := b.setCursorPos(x, y); err != nil {
		b.setErr(err)
		return err
	}
	flags := win32.MOUSEEVENTF_WHEEL
	if horizontal {
		flags = win32.MOUSEEVENTF_HWHEEL
	}
	if err := sendMouseInput(win32.MOUSEINPUT{MouseData: uint32(int32(delta)), DwFlags: flags}); err != nil {
		b.setErr(err)
		return err
	}
	b.clearErr()
	return nil
}

func (b *Backend) Done() <-chan struct{} {
	return b.done
}

func (b *Backend) Err() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.runErr
}

func (b *Backend) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.status.Connected = false
		b.status.Active = false
		b.status.Ready = false
		b.status.State = "CLOSED"
		b.mu.Unlock()
		close(b.done)
	})
	return b.Err()
}

func (b *Backend) setCursorPos(x float64, y float64) error {
	bounds, err := virtualScreenBounds()
	if err != nil {
		return err
	}
	roundedX, err := desktop.RoundCoordinate(x)
	if err != nil {
		return err
	}
	roundedY, err := desktop.RoundCoordinate(y)
	if err != nil {
		return err
	}
	if roundedX < 0 || roundedY < 0 || roundedX >= bounds.width || roundedY >= bounds.height {
		return fmt.Errorf("mouse coordinates %d,%d are outside screen bounds %dx%d", roundedX, roundedY, bounds.width, bounds.height)
	}
	ok, winErr := win32.SetCursorPos(int32(bounds.left+roundedX), int32(bounds.top+roundedY))
	if ok == 0 {
		if fallbackErr := sendAbsoluteMouseMove(roundedX, roundedY, bounds); fallbackErr != nil {
			return errors.Join(callError("SetCursorPos", winErr), fallbackErr)
		}
	}
	b.setReady(bounds)
	return nil
}

func (b *Backend) setReady(bounds screenBounds) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status.Ready = true
	b.status.Width = bounds.width
	b.status.Height = bounds.height
	b.status.Regions = []desktop.Region{{X: 0, Y: 0, W: bounds.width, H: bounds.height}}
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

func (b *Backend) clearErr() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runErr = nil
	b.status.Error = ""
}

func captureScreen(left int, top int, width int, height int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid screenshot size: %dx%d", width, height)
	}

	screenDC := win32.GetDC(0)
	if screenDC == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer win32.ReleaseDC(0, screenDC)

	memDC := win32.CreateCompatibleDC(screenDC)
	if memDC == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer win32.DeleteDC(memDC)

	bitmap := win32.CreateCompatibleBitmap(screenDC, int32(width), int32(height))
	if bitmap == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer win32.DeleteObject(win32.HGDIOBJ(bitmap))

	oldObject := win32.SelectObject(memDC, win32.HGDIOBJ(bitmap))
	if oldObject == 0 {
		return nil, errors.New("SelectObject failed")
	}
	defer win32.SelectObject(memDC, oldObject)

	ok, err := win32.BitBlt(
		memDC,
		0,
		0,
		int32(width),
		int32(height),
		screenDC,
		int32(left),
		int32(top),
		win32.SRCCOPY|win32.CAPTUREBLT,
	)
	if ok == 0 {
		return nil, callError("BitBlt", err)
	}

	pixels := make([]byte, width*height*4)
	info := win32.BITMAPINFO{
		BmiHeader: win32.BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(win32.BITMAPINFOHEADER{})),
			BiWidth:       int32(width),
			BiHeight:      -int32(height),
			BiPlanes:      1,
			BiBitCount:    screenshotBitDepth,
			BiCompression: win32.BI_RGB,
			BiSizeImage:   uint32(len(pixels)),
		},
	}

	if rows := win32.GetDIBits(
		memDC,
		bitmap,
		0,
		uint32(height),
		unsafe.Pointer(&pixels[0]),
		&info,
		win32.DIB_RGB_COLORS,
	); rows == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := pixels[y*width*4:]
		dstRow := img.Pix[y*img.Stride:]
		for x := 0; x < width; x++ {
			src := x * 4
			dst := x * 4
			dstRow[dst+0] = srcRow[src+2]
			dstRow[dst+1] = srcRow[src+1]
			dstRow[dst+2] = srcRow[src+0]
			dstRow[dst+3] = opaqueAlpha
		}
	}
	return img, nil
}

func virtualScreenBounds() (screenBounds, error) {
	left := int(win32.GetSystemMetrics(win32.SM_XVIRTUALSCREEN))
	top := int(win32.GetSystemMetrics(win32.SM_YVIRTUALSCREEN))
	width := int(win32.GetSystemMetrics(win32.SM_CXVIRTUALSCREEN))
	height := int(win32.GetSystemMetrics(win32.SM_CYVIRTUALSCREEN))
	if width <= 0 || height <= 0 {
		return screenBounds{}, fmt.Errorf("invalid virtual screen bounds: %d,%d %dx%d", left, top, width, height)
	}
	return screenBounds{left: left, top: top, width: width, height: height}, nil
}

func cropRect(width int, height int, crop desktop.Crop) (image.Rectangle, error) {
	bounds := image.Rect(0, 0, width, height)
	left := bounds.Min.X
	top := bounds.Min.Y
	right := bounds.Max.X
	bottom := bounds.Max.Y

	if crop.X != nil {
		left = *crop.X
	}
	if crop.Y != nil {
		top = *crop.Y
	}
	if crop.W != nil {
		right = left + *crop.W
	}
	if crop.H != nil {
		bottom = top + *crop.H
	}

	rect := image.Rect(left, top, right, bottom)
	if !rect.In(bounds) {
		return image.Rectangle{}, fmt.Errorf("crop rectangle (%d,%d,%d,%d) is outside screenshot bounds %s", left, top, right, bottom, bounds)
	}
	if rect.Empty() {
		return image.Rectangle{}, errors.New("crop rectangle must have positive width and height")
	}
	return rect, nil
}

func sendVirtualKey(key virtualKey, down bool) error {
	flags := win32.KEYBD_EVENT_FLAGS(0)
	if !down {
		flags |= win32.KEYEVENTF_KEYUP
	}
	if key.extended {
		flags |= win32.KEYEVENTF_EXTENDEDKEY
	}
	return sendKeyboardInput(win32.KEYBDINPUT{WVk: key.code, DwFlags: flags})
}

func sendUnicodeUnit(unit uint16, down bool) error {
	flags := win32.KEYEVENTF_UNICODE
	if !down {
		flags |= win32.KEYEVENTF_KEYUP
	}
	return sendKeyboardInput(win32.KEYBDINPUT{WScan: unit, DwFlags: flags})
}

func sendKeyboardInput(data win32.KEYBDINPUT) error {
	in := win32.INPUT{Type_: win32.INPUT_KEYBOARD}
	*in.Ki() = data
	return sendInput(in)
}

func sendMouseInput(data win32.MOUSEINPUT) error {
	in := win32.INPUT{Type_: win32.INPUT_MOUSE}
	*in.Mi() = data
	return sendInput(in)
}

func sendAbsoluteMouseMove(x int, y int, bounds screenBounds) error {
	return sendMouseInput(win32.MOUSEINPUT{
		Dx:      normalizedAbsoluteMouseCoordinate(x, bounds.width),
		Dy:      normalizedAbsoluteMouseCoordinate(y, bounds.height),
		DwFlags: win32.MOUSEEVENTF_MOVE | win32.MOUSEEVENTF_ABSOLUTE | win32.MOUSEEVENTF_VIRTUALDESK,
	})
}

func normalizedAbsoluteMouseCoordinate(value int, size int) int32 {
	if size <= 1 {
		return 0
	}
	return int32(value * absoluteMouseCoordinateMax / (size - 1))
}

func sendInput(in win32.INPUT) error {
	count, err := win32.SendInput(
		1,
		&in,
		int32(unsafe.Sizeof(in)),
	)
	if count != 1 {
		return callError("SendInput", err)
	}
	return nil
}

func scancodeFlags(down bool, extended bool) win32.KEYBD_EVENT_FLAGS {
	flags := win32.KEYEVENTF_SCANCODE
	if !down {
		flags |= win32.KEYEVENTF_KEYUP
	}
	if extended {
		flags |= win32.KEYEVENTF_EXTENDEDKEY
	}
	return flags
}

func mouseButtonEvent(button inputcode.MouseButtonName, down bool) (win32.MOUSE_EVENT_FLAGS, uint32, error) {
	event, err := inputcode.WindowsMouseButtonEvent(button)
	if err != nil {
		return 0, 0, err
	}
	if down {
		return event.DownFlags, event.Data, nil
	}
	return event.UpFlags, event.Data, nil
}

func callError(name string, err win32.WIN32_ERROR) error {
	if err != win32.NO_ERROR {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("%s failed", name)
}
