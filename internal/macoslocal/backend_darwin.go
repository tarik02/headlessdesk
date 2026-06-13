//go:build darwin

package macoslocal

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <dlfcn.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	double x;
	double y;
	double width;
	double height;
	size_t pixel_width;
	size_t pixel_height;
	double scale_x;
	double scale_y;
	bool valid;
} hd_display_info;

typedef struct {
	uint8_t* pixels;
	size_t len;
	size_t width;
	size_t height;
	int code;
} hd_capture_result;

typedef CGImageRef (*hd_window_list_create_image_fn)(CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);

static hd_display_info hd_main_display_info(void) {
	CGDirectDisplayID display = CGMainDisplayID();
	CGRect bounds = CGDisplayBounds(display);
	size_t pixel_width = CGDisplayPixelsWide(display);
	size_t pixel_height = CGDisplayPixelsHigh(display);
	CGDisplayModeRef mode = CGDisplayCopyDisplayMode(display);
	if (mode != NULL) {
		size_t mode_width = CGDisplayModeGetPixelWidth(mode);
		size_t mode_height = CGDisplayModeGetPixelHeight(mode);
		if (mode_width > 0 && mode_height > 0) {
			pixel_width = mode_width;
			pixel_height = mode_height;
		}
		CFRelease(mode);
	}

	hd_display_info info = {
		.x = bounds.origin.x,
		.y = bounds.origin.y,
		.width = bounds.size.width,
		.height = bounds.size.height,
		.pixel_width = pixel_width,
		.pixel_height = pixel_height,
		.scale_x = bounds.size.width > 0 ? (double)pixel_width / bounds.size.width : 1,
		.scale_y = bounds.size.height > 0 ? (double)pixel_height / bounds.size.height : 1,
		.valid = bounds.size.width > 0 && bounds.size.height > 0 && pixel_width > 0 && pixel_height > 0,
	};
	return info;
}

static hd_capture_result hd_capture_main_display_rect(double x, double y, double width, double height) {
	hd_capture_result result = {0};
	if (width <= 0 || height <= 0) {
		result.code = 1;
		return result;
	}

	hd_window_list_create_image_fn create_image = (hd_window_list_create_image_fn)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
	if (create_image == NULL) {
		result.code = 7;
		return result;
	}

	CGImageRef image = create_image(
		CGRectMake(x, y, width, height),
		kCGWindowListOptionOnScreenOnly,
		kCGNullWindowID,
		kCGWindowImageDefault
	);
	if (image == NULL) {
		result.code = 2;
		return result;
	}

	size_t image_width = CGImageGetWidth(image);
	size_t image_height = CGImageGetHeight(image);
	size_t len = image_width * image_height * 4;
	if (image_width == 0 || image_height == 0 || len / 4 / image_width != image_height) {
		CGImageRelease(image);
		result.code = 3;
		return result;
	}

	uint8_t* pixels = (uint8_t*)calloc(len, 1);
	if (pixels == NULL) {
		CGImageRelease(image);
		result.code = 4;
		return result;
	}

	CGColorSpaceRef color_space = CGColorSpaceCreateDeviceRGB();
	if (color_space == NULL) {
		free(pixels);
		CGImageRelease(image);
		result.code = 5;
		return result;
	}

	CGContextRef ctx = CGBitmapContextCreate(
		pixels,
		image_width,
		image_height,
		8,
		image_width * 4,
		color_space,
		kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big
	);
	CGColorSpaceRelease(color_space);
	if (ctx == NULL) {
		free(pixels);
		CGImageRelease(image);
		result.code = 6;
		return result;
	}

	CGContextDrawImage(ctx, CGRectMake(0, 0, image_width, image_height), image);
	CGContextRelease(ctx);
	CGImageRelease(image);

	result.pixels = pixels;
	result.len = len;
	result.width = image_width;
	result.height = image_height;
	return result;
}

static void hd_free(void* ptr) {
	free(ptr);
}

static bool hd_screen_capture_allowed(void) {
	return CGPreflightScreenCaptureAccess();
}

static bool hd_request_screen_capture_access(void) {
	return CGRequestScreenCaptureAccess();
}

static bool hd_post_event_allowed(void) {
	return CGPreflightPostEventAccess();
}

static bool hd_request_post_event_access(void) {
	return CGRequestPostEventAccess();
}

static bool hd_accessibility_trusted(void) {
	return AXIsProcessTrustedWithOptions(NULL);
}

static bool hd_request_accessibility_trust(void) {
	const void* keys[] = { kAXTrustedCheckOptionPrompt };
	const void* values[] = { kCFBooleanTrue };
	CFDictionaryRef options = CFDictionaryCreate(
		kCFAllocatorDefault,
		keys,
		values,
		1,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks
	);
	if (options == NULL) {
		return AXIsProcessTrustedWithOptions(NULL);
	}
	Boolean trusted = AXIsProcessTrustedWithOptions(options);
	CFRelease(options);
	return trusted;
}

static bool hd_post_key(uint16_t keycode, bool down, bool repeat) {
	CGEventRef event = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keycode, down);
	if (event == NULL) {
		return false;
	}
	CGEventSetIntegerValueField(event, kCGKeyboardEventAutorepeat, repeat ? 1 : 0);
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);
	return true;
}

static bool hd_post_unicode(const UniChar* chars, size_t len, bool down) {
	CGEventRef event = CGEventCreateKeyboardEvent(NULL, 0, down);
	if (event == NULL) {
		return false;
	}
	CGEventKeyboardSetUnicodeString(event, len, chars);
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);
	return true;
}

static bool hd_move_mouse(double x, double y) {
	CGEventRef event = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, CGPointMake(x, y), kCGMouseButtonLeft);
	if (event == NULL) {
		return false;
	}
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);
	return true;
}

static bool hd_post_mouse_button(double x, double y, int button, bool down) {
	CGEventType type;
	CGMouseButton mouse_button;
	switch (button) {
	case 0:
		type = down ? kCGEventLeftMouseDown : kCGEventLeftMouseUp;
		mouse_button = kCGMouseButtonLeft;
		break;
	case 1:
		type = down ? kCGEventRightMouseDown : kCGEventRightMouseUp;
		mouse_button = kCGMouseButtonRight;
		break;
	case 2:
		type = down ? kCGEventOtherMouseDown : kCGEventOtherMouseUp;
		mouse_button = kCGMouseButtonCenter;
		break;
	default:
		type = down ? kCGEventOtherMouseDown : kCGEventOtherMouseUp;
		mouse_button = (CGMouseButton)button;
		break;
	}

	CGEventRef event = CGEventCreateMouseEvent(NULL, type, CGPointMake(x, y), mouse_button);
	if (event == NULL) {
		return false;
	}
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);
	return true;
}

static bool hd_post_scroll(int32_t vertical, int32_t horizontal) {
	CGEventRef event = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitPixel, 2, vertical, horizontal);
	if (event == NULL) {
		return false;
	}
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);
	return true;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"unicode/utf16"
	"unsafe"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

const protocolName = "macos"

type Backend struct {
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	status    splitStatus
	runErr    error
}

type splitStatus struct {
	output desktop.Status
	input  desktop.Status
}

type displayInfo struct {
	x           float64
	y           float64
	width       float64
	height      float64
	pixelWidth  int
	pixelHeight int
	scaleX      float64
	scaleY      float64
}

func New() (*Backend, error) {
	b := &Backend{done: make(chan struct{})}
	requestPermissions()
	b.refreshStatus(nil)
	return b, nil
}

func (b *Backend) Status() desktop.Status {
	return b.OutputStatus()
}

func (b *Backend) OutputStatus() desktop.Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	status := b.status.output
	if b.runErr != nil && status.Error == "" {
		status.Error = b.runErr.Error()
	}
	return status
}

func (b *Backend) InputStatus() desktop.Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	status := b.status.input
	if b.runErr != nil && status.Error == "" {
		status.Error = b.runErr.Error()
	}
	return status
}

func (b *Backend) Screenshot() (image.Image, error) {
	display, err := currentDisplay()
	if err != nil {
		b.refreshStatus(err)
		return nil, err
	}
	img, err := captureRect(display, 0, 0, display.pixelWidth, display.pixelHeight)
	if err != nil {
		b.refreshStatus(err)
		return nil, err
	}
	b.refreshStatus(nil)
	return img, nil
}

func (b *Backend) ScreenshotCrop(crop desktop.Crop) (image.Image, error) {
	display, err := currentDisplay()
	if err != nil {
		b.refreshStatus(err)
		return nil, err
	}
	rect, err := cropRect(display.pixelWidth, display.pixelHeight, crop)
	if err != nil {
		b.refreshStatus(err)
		return nil, err
	}
	img, err := captureRect(display, rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())
	if err != nil {
		b.refreshStatus(err)
		return nil, err
	}
	b.refreshStatus(nil)
	return img, nil
}

func (b *Backend) SendKey(name inputcode.KeyName, down bool, repeat bool) error {
	key, err := macVirtualKey(name.Code())
	if err != nil {
		b.refreshStatus(err)
		return err
	}
	if err := postKey(key, down, repeat); err != nil {
		b.refreshStatus(err)
		return err
	}
	b.refreshStatus(nil)
	return nil
}

func (b *Backend) SendKeyScancode(scancode inputcode.Scancode, down bool, repeat bool) error {
	key, err := macVirtualKey(scancode.Uint32())
	if err != nil {
		b.refreshStatus(err)
		return err
	}
	if err := postKey(key, down, repeat); err != nil {
		b.refreshStatus(err)
		return err
	}
	b.refreshStatus(nil)
	return nil
}

func (b *Backend) TypeText(text string) error {
	if text == "" {
		err := errors.New("text is required")
		b.refreshStatus(err)
		return err
	}
	units := utf16.Encode([]rune(text))
	if err := postUnicode(units, true); err != nil {
		b.refreshStatus(err)
		return err
	}
	if err := postUnicode(units, false); err != nil {
		b.refreshStatus(err)
		return err
	}
	b.refreshStatus(nil)
	return nil
}

func (b *Backend) MoveMouse(x float64, y float64) error {
	display, px, py, err := b.displayPoint(x, y)
	if err != nil {
		b.refreshStatus(err)
		return err
	}
	if C.hd_move_mouse(C.double(px), C.double(py)) == false {
		err := errors.New("post macOS mouse move event failed")
		b.refreshStatus(err)
		return err
	}
	b.setReady(display)
	return nil
}

func (b *Backend) SendMouseButton(button inputcode.MouseButtonName, x float64, y float64, down bool) error {
	display, px, py, err := b.displayPoint(x, y)
	if err != nil {
		b.refreshStatus(err)
		return err
	}
	buttonIndex, err := inputcode.MouseButtonNameIndex(button)
	if err != nil {
		b.refreshStatus(err)
		return err
	}
	if C.hd_post_mouse_button(C.double(px), C.double(py), C.int(buttonIndex), C.bool(down)) == false {
		err := fmt.Errorf("post macOS mouse button event failed: %s", button)
		b.refreshStatus(err)
		return err
	}
	b.setReady(display)
	return nil
}

func (b *Backend) SendMouseWheel(x float64, y float64, delta int, horizontal bool) error {
	if delta == 0 {
		err := errors.New("wheel delta must be non-zero")
		b.refreshStatus(err)
		return err
	}
	display, px, py, err := b.displayPoint(x, y)
	if err != nil {
		b.refreshStatus(err)
		return err
	}
	if C.hd_move_mouse(C.double(px), C.double(py)) == false {
		err := errors.New("post macOS mouse move event failed")
		b.refreshStatus(err)
		return err
	}
	vertical := 0
	horizontalDelta := 0
	if horizontal {
		horizontalDelta = delta
	} else {
		vertical = delta
	}
	if C.hd_post_scroll(C.int32_t(vertical), C.int32_t(horizontalDelta)) == false {
		err := errors.New("post macOS scroll event failed")
		b.refreshStatus(err)
		return err
	}
	b.setReady(display)
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
		b.status.output.Connected = false
		b.status.output.Active = false
		b.status.output.Ready = false
		b.status.output.State = "CLOSED"
		b.status.input.Connected = false
		b.status.input.Active = false
		b.status.input.Ready = false
		b.status.input.State = "CLOSED"
		b.mu.Unlock()
		close(b.done)
	})
	return b.Err()
}

func (b *Backend) displayPoint(x float64, y float64) (displayInfo, float64, float64, error) {
	display, err := currentDisplay()
	if err != nil {
		return displayInfo{}, 0, 0, err
	}
	if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
		return displayInfo{}, 0, 0, errors.New("mouse coordinates must be finite")
	}
	if x < 0 || y < 0 || x >= float64(display.pixelWidth) || y >= float64(display.pixelHeight) {
		return displayInfo{}, 0, 0, fmt.Errorf("mouse coordinates %g,%g are outside screen bounds %dx%d", x, y, display.pixelWidth, display.pixelHeight)
	}
	return display, display.x + float64(x)/display.scaleX, display.y + float64(y)/display.scaleY, nil
}

func (b *Backend) refreshStatus(err error) {
	display, displayErr := currentDisplay()
	if displayErr != nil && err == nil {
		err = displayErr
	}
	outputReady := displayErr == nil && probeCapture(display) == nil
	inputReady := postEventAllowed() && accessibilityTrusted()
	outputErr := outputStatusError(displayErr, outputReady)
	inputErr := inputStatusError(inputReady)
	if err != nil && displayErr == nil && outputErr == "" && inputErr == "" {
		outputErr = err.Error()
		inputErr = err.Error()
	}

	output := baseStatus(display)
	output.Ready = outputReady
	output.State = readinessState(outputReady)
	output.Error = outputErr

	input := baseStatus(display)
	input.Ready = inputReady
	input.State = readinessState(inputReady)
	input.Error = inputErr

	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = splitStatus{output: output, input: input}
	b.runErr = err
}

func (b *Backend) setReady(display displayInfo) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status.output = baseStatus(display)
	b.status.output.Ready = screenCaptureAllowed()
	b.status.output.State = readinessState(b.status.output.Ready)
	b.status.output.Error = outputStatusError(nil, b.status.output.Ready)
	b.status.input = baseStatus(display)
	b.status.input.Ready = postEventAllowed() && accessibilityTrusted()
	b.status.input.State = readinessState(b.status.input.Ready)
	b.status.input.Error = inputStatusError(b.status.input.Ready)
	b.runErr = nil
}

func baseStatus(display displayInfo) desktop.Status {
	status := desktop.Status{
		Protocol:  protocolName,
		Connected: true,
		Active:    display.pixelWidth > 0 && display.pixelHeight > 0,
		Width:     display.pixelWidth,
		Height:    display.pixelHeight,
	}
	if display.pixelWidth > 0 && display.pixelHeight > 0 {
		status.Regions = []desktop.Region{{X: 0, Y: 0, W: display.pixelWidth, H: display.pixelHeight}}
	}
	return status
}

func readinessState(ready bool) string {
	if ready {
		return "READY"
	}
	return "NOT_READY"
}

func outputStatusError(displayErr error, ready bool) string {
	if displayErr != nil {
		return displayErr.Error()
	}
	if !screenCaptureAllowed() {
		return "macOS Screen Recording permission is required for screenshots"
	}
	if !ready {
		return "macOS screenshot probe failed"
	}
	return ""
}

func inputStatusError(ready bool) string {
	if !postEventAllowed() {
		return "macOS Input Monitoring permission is required to post input events"
	}
	if !accessibilityTrusted() {
		return "macOS Accessibility permission is required for desktop control"
	}
	if !ready {
		return "macOS input permissions are not ready"
	}
	return ""
}

func currentDisplay() (displayInfo, error) {
	info := C.hd_main_display_info()
	if !bool(info.valid) {
		return displayInfo{}, errors.New("macOS main display is unavailable; an active logged-in graphical session is required")
	}
	scaleX := float64(info.scale_x)
	scaleY := float64(info.scale_y)
	if scaleX <= 0 {
		scaleX = 1
	}
	if scaleY <= 0 {
		scaleY = 1
	}
	return displayInfo{
		x:           float64(info.x),
		y:           float64(info.y),
		width:       float64(info.width),
		height:      float64(info.height),
		pixelWidth:  int(info.pixel_width),
		pixelHeight: int(info.pixel_height),
		scaleX:      scaleX,
		scaleY:      scaleY,
	}, nil
}

func probeCapture(display displayInfo) error {
	_, err := captureRect(display, 0, 0, 1, 1)
	return err
}

func captureRect(display displayInfo, x int, y int, width int, height int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid screenshot size: %dx%d", width, height)
	}
	if x < 0 || y < 0 || x+width > display.pixelWidth || y+height > display.pixelHeight {
		return nil, fmt.Errorf("crop rectangle (%d,%d,%d,%d) is outside screenshot bounds %s", x, y, x+width, y+height, image.Rect(0, 0, display.pixelWidth, display.pixelHeight))
	}

	result := C.hd_capture_main_display_rect(
		C.double(display.x+float64(x)/display.scaleX),
		C.double(display.y+float64(y)/display.scaleY),
		C.double(float64(width)/display.scaleX),
		C.double(float64(height)/display.scaleY),
	)
	if result.pixels != nil {
		defer C.hd_free(unsafe.Pointer(result.pixels))
	}
	if result.code != 0 {
		return nil, captureError(int(result.code))
	}
	if result.pixels == nil || result.len == 0 || result.width == 0 || result.height == 0 {
		return nil, errors.New("macOS screenshot capture returned no pixels")
	}
	if result.width > C.size_t(maxInt) || result.height > C.size_t(maxInt) || result.len > C.size_t(maxCGoBytes) {
		return nil, errors.New("macOS screenshot capture is too large")
	}

	pixels := C.GoBytes(unsafe.Pointer(result.pixels), C.int(result.len))
	img := image.NewNRGBA(image.Rect(0, 0, int(result.width), int(result.height)))
	copy(img.Pix, pixels)
	return img, nil
}

const (
	maxInt      = int(^uint(0) >> 1)
	maxCGoBytes = 1<<31 - 1
)

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

func captureError(code int) error {
	switch code {
	case 1:
		return errors.New("invalid macOS screenshot rectangle")
	case 2:
		if !screenCaptureAllowed() {
			return errors.New("macOS Screen Recording permission is required for screenshots")
		}
		return errors.New("macOS screenshot capture failed")
	case 3:
		return errors.New("macOS screenshot capture returned invalid dimensions")
	case 4:
		return errors.New("allocate macOS screenshot buffer")
	case 5:
		return errors.New("create macOS screenshot color space")
	case 6:
		return errors.New("create macOS screenshot bitmap context")
	case 7:
		return errors.New("macOS legacy CoreGraphics screenshot capture is unavailable on this system")
	default:
		return fmt.Errorf("macOS screenshot capture failed with code %d", code)
	}
}

func postKey(key uint16, down bool, repeat bool) error {
	if !postEventAllowed() {
		return errors.New("macOS Input Monitoring permission is required to post input events")
	}
	if !accessibilityTrusted() {
		return errors.New("macOS Accessibility permission is required for desktop control")
	}
	if C.hd_post_key(C.uint16_t(key), C.bool(down), C.bool(repeat)) == false {
		return errors.New("post macOS keyboard event failed")
	}
	return nil
}

func postUnicode(units []uint16, down bool) error {
	if len(units) == 0 {
		return nil
	}
	if !postEventAllowed() {
		return errors.New("macOS Input Monitoring permission is required to post input events")
	}
	if !accessibilityTrusted() {
		return errors.New("macOS Accessibility permission is required for desktop control")
	}
	if C.hd_post_unicode((*C.UniChar)(unsafe.Pointer(&units[0])), C.size_t(len(units)), C.bool(down)) == false {
		return errors.New("post macOS unicode keyboard event failed")
	}
	return nil
}

func screenCaptureAllowed() bool {
	return bool(C.hd_screen_capture_allowed())
}

func requestPermissions() {
	if !screenCaptureAllowed() {
		_ = C.hd_request_screen_capture_access()
	}
	if !postEventAllowed() {
		_ = C.hd_request_post_event_access()
	}
	if !accessibilityTrusted() {
		_ = C.hd_request_accessibility_trust()
	}
}

func postEventAllowed() bool {
	return bool(C.hd_post_event_allowed())
}

func accessibilityTrusted() bool {
	return bool(C.hd_accessibility_trusted())
}
