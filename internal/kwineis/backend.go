//go:build linux

package kwineis

/*
#cgo pkg-config: libei-1.0
#include <libei.h>
#include <stdbool.h>
#include <stdlib.h>

static void hd_ei_bind_all(struct ei_seat *seat) {
	ei_seat_bind_capabilities(seat,
		EI_DEVICE_CAP_POINTER,
		EI_DEVICE_CAP_POINTER_ABSOLUTE,
		EI_DEVICE_CAP_BUTTON,
		EI_DEVICE_CAP_SCROLL,
		EI_DEVICE_CAP_KEYBOARD,
		NULL);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"

	"headlessdesk/internal/desktop"
	"headlessdesk/internal/inputcode"
)

const (
	serviceName   = "org.kde.KWin"
	objectPath    = "/org/kde/KWin/EIS/RemoteDesktop"
	interfaceName = "org.kde.KWin.EIS.RemoteDesktop"

	eisCapabilities = int32(C.EI_DEVICE_CAP_POINTER |
		C.EI_DEVICE_CAP_POINTER_ABSOLUTE |
		C.EI_DEVICE_CAP_BUTTON |
		C.EI_DEVICE_CAP_SCROLL |
		C.EI_DEVICE_CAP_KEYBOARD)
)

type Backend struct {
	conn   *dbus.Conn
	obj    dbus.BusObject
	cookie int32
	ei     *C.struct_ei

	stop      chan struct{}
	done      chan struct{}
	ready     chan struct{}
	closeOnce sync.Once
	doneOnce  sync.Once
	readyOnce sync.Once
	wg        sync.WaitGroup

	mu         sync.Mutex
	sequence   uint32
	pointerAbs *C.struct_ei_device
	button     *C.struct_ei_device
	scroll     *C.struct_ei_device
	keyboard   *C.struct_ei_device
	regions    []desktop.Region

	statusMu sync.RWMutex
	status   desktop.Status
	runErr   error
	closing  bool
}

func New() (*Backend, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect session bus: %w", err)
	}

	obj := conn.Object(serviceName, dbus.ObjectPath(objectPath))
	var fd dbus.UnixFD
	var cookie int32
	call := obj.Call(interfaceName+".connectToEIS", 0, eisCapabilities)
	if call.Err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("connect to KWin EIS: %w", call.Err)
	}
	if err := call.Store(&fd, &cookie); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("decode KWin EIS response: %w", err)
	}

	ei := C.ei_new_sender(nil)
	if ei == nil {
		_ = unix.Close(int(fd))
		_ = conn.Close()
		return nil, errors.New("create libei sender")
	}
	C.ei_log_set_priority(ei, C.EI_LOG_PRIORITY_ERROR)
	name := C.CString("headlessdesk")
	C.ei_configure_name(ei, name)
	C.free(unsafe.Pointer(name))
	if rc := C.ei_setup_backend_fd(ei, C.int(fd)); rc < 0 {
		C.ei_unref(ei)
		_ = conn.Close()
		return nil, fmt.Errorf("set up libei backend fd: %d", int(rc))
	}

	b := &Backend{
		conn:   conn,
		obj:    obj,
		cookie: cookie,
		ei:     ei,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		ready:  make(chan struct{}),
		status: desktop.Status{
			Protocol:  "eis",
			Connected: true,
			Active:    true,
			State:     "CONNECTING",
		},
	}
	b.wg.Add(1)
	go b.run()

	select {
	case <-b.ready:
		return b, nil
	case <-b.done:
		if err := b.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("KWin EIS backend closed before ready")
	case <-time.After(5 * time.Second):
		_ = b.Close()
		return nil, errors.New("timed out waiting for KWin EIS devices")
	}
}

func (b *Backend) Status() desktop.Status {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	status := b.status
	if b.runErr != nil {
		status.Error = b.runErr.Error()
	}
	return status
}

func (b *Backend) Done() <-chan struct{} {
	return b.done
}

func (b *Backend) Err() error {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	return b.runErr
}

func (b *Backend) Close() error {
	b.closeOnce.Do(func() {
		b.statusMu.Lock()
		b.closing = true
		b.statusMu.Unlock()
		close(b.stop)
		b.wg.Wait()
		if b.cookie != 0 {
			_ = b.obj.Call(interfaceName+".disconnect", 0, b.cookie).Err
			b.cookie = 0
		}
			_ = b.conn.Close()
			b.statusMu.Lock()
		b.status.Connected = false
		b.status.Active = false
		b.status.Ready = false
		b.status.State = "CLOSED"
		b.statusMu.Unlock()
		b.closeDone()
	})
	return b.Err()
}

func (b *Backend) MapInputPoint(outputWidth int, outputHeight int, x int, y int) (int, int, error) {
	if x < 0 || y < 0 {
		return 0, 0, fmt.Errorf("mouse coordinates must be non-negative: %d,%d", x, y)
	}
	if outputWidth <= 0 || outputHeight <= 0 {
		return x, y, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	union, ok := regionUnion(b.regions)
	if !ok {
		return x, y, nil
	}

	mappedX := union.X + int(float64(x)*float64(union.W)/float64(outputWidth))
	mappedY := union.Y + int(float64(y)*float64(union.H)/float64(outputHeight))
	return clamp(mappedX, union.X, union.X+union.W-1), clamp(mappedY, union.Y, union.Y+union.H-1), nil
}

func (b *Backend) SendKey(name string, down bool, repeat bool) error {
	code, err := inputcode.Key(name)
	if err != nil {
		return err
	}
	return b.SendKeyScancode(code, down, repeat)
}

func (b *Backend) SendKeyScancode(scancode uint32, down bool, repeat bool) error {
	if scancode == 0 {
		return errors.New("scancode must be non-zero")
	}
	return b.withDevice("keyboard", func() *C.struct_ei_device { return b.keyboard }, func(device *C.struct_ei_device) {
		C.ei_device_keyboard_key(device, C.uint32_t(scancode), C.bool(down))
	})
}

func (b *Backend) TypeText(text string) error {
	events, err := inputcode.Text(text)
	if err != nil {
		return err
	}
	return b.withDevice("keyboard", func() *C.struct_ei_device { return b.keyboard }, func(device *C.struct_ei_device) {
		for _, event := range events {
			C.ei_device_keyboard_key(device, C.uint32_t(event.Code), C.bool(event.Down))
		}
	})
}

func (b *Backend) MoveMouse(x int, y int) error {
	if x < 0 || y < 0 {
		return fmt.Errorf("mouse coordinates must be non-negative: %d,%d", x, y)
	}
	return b.withDevice("absolute pointer", func() *C.struct_ei_device { return b.pointerAbs }, func(device *C.struct_ei_device) {
		C.ei_device_pointer_motion_absolute(device, C.double(x), C.double(y))
	})
}

func (b *Backend) SendMouseButton(button string, x int, y int, down bool) error {
	code, err := inputcode.MouseButton(button)
	if err != nil {
		return err
	}
	if err := b.MoveMouse(x, y); err != nil {
		return err
	}
	return b.withDevice("button", func() *C.struct_ei_device { return b.button }, func(device *C.struct_ei_device) {
		C.ei_device_button_button(device, C.uint32_t(code), C.bool(down))
	})
}

func (b *Backend) SendMouseWheel(x int, y int, delta int, horizontal bool) error {
	if delta == 0 {
		return errors.New("wheel delta must be non-zero")
	}
	if err := b.MoveMouse(x, y); err != nil {
		return err
	}
	scrollX := C.int32_t(0)
	scrollY := C.int32_t(0)
	if horizontal {
		scrollX = C.int32_t(delta)
	} else {
		scrollY = C.int32_t(delta)
	}
	return b.withDevice("scroll", func() *C.struct_ei_device { return b.scroll }, func(device *C.struct_ei_device) {
		C.ei_device_scroll_discrete(device, scrollX, scrollY)
	})
}

func (b *Backend) run() {
	defer b.wg.Done()
	fd := int32(C.ei_get_fd(b.ei))
	pollFDs := []unix.PollFd{{Fd: fd, Events: unix.POLLIN | unix.POLLHUP | unix.POLLERR}}

	for {
		select {
		case <-b.stop:
			b.cleanup()
			return
		default:
		}

		n, err := unix.Poll(pollFDs, 100)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			b.finish(fmt.Errorf("poll KWin EIS fd: %w", err))
			b.cleanup()
			return
		}
		if n == 0 {
			continue
		}

		b.mu.Lock()
		C.ei_dispatch(b.ei)
		for {
			event := C.ei_get_event(b.ei)
			if event == nil {
				break
			}
			b.handleEventLocked(event)
			C.ei_event_unref(event)
		}
		b.mu.Unlock()

		select {
		case <-b.done:
			b.cleanup()
			return
		default:
		}
		if pollFDs[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			b.finish(errors.New("KWin EIS connection closed"))
			b.cleanup()
			return
		}
	}
}

func (b *Backend) handleEventLocked(event *C.struct_ei_event) {
	switch C.ei_event_get_type(event) {
	case C.EI_EVENT_CONNECT:
		b.setState("CONNECTED", nil)
	case C.EI_EVENT_DISCONNECT:
		if b.isClosing() {
			b.finish(nil)
		} else {
			b.finish(errors.New("KWin EIS disconnected"))
		}
	case C.EI_EVENT_SEAT_ADDED:
		seat := C.ei_event_get_seat(event)
		if seat != nil {
			C.hd_ei_bind_all(seat)
		}
	case C.EI_EVENT_DEVICE_RESUMED:
		device := C.ei_event_get_device(event)
		if device != nil {
			b.addDeviceLocked(device)
		}
	case C.EI_EVENT_DEVICE_PAUSED, C.EI_EVENT_DEVICE_REMOVED:
		device := C.ei_event_get_device(event)
		if device != nil {
			b.removeDeviceLocked(device)
		}
	}
}

func (b *Backend) isClosing() bool {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	return b.closing
}

func (b *Backend) addDeviceLocked(device *C.struct_ei_device) {
	if C.ei_device_has_capability(device, C.EI_DEVICE_CAP_POINTER_ABSOLUTE) {
		b.replaceDeviceLocked(&b.pointerAbs, device)
		b.setRegionsLocked(deviceRegions(device))
	}
	if C.ei_device_has_capability(device, C.EI_DEVICE_CAP_BUTTON) {
		b.replaceDeviceLocked(&b.button, device)
	}
	if C.ei_device_has_capability(device, C.EI_DEVICE_CAP_SCROLL) {
		b.replaceDeviceLocked(&b.scroll, device)
	}
	if C.ei_device_has_capability(device, C.EI_DEVICE_CAP_KEYBOARD) {
		b.replaceDeviceLocked(&b.keyboard, device)
	}
	if b.pointerAbs != nil && b.button != nil && b.scroll != nil && b.keyboard != nil {
		b.statusMu.Lock()
		b.status.Ready = true
		b.status.State = "READY"
		b.runErr = nil
		b.status.Error = ""
		b.statusMu.Unlock()
		b.readyOnce.Do(func() { close(b.ready) })
	}
}

func (b *Backend) replaceDeviceLocked(slot **C.struct_ei_device, device *C.struct_ei_device) {
	if *slot == device {
		return
	}
	if *slot != nil {
		C.ei_device_unref(*slot)
	}
	*slot = C.ei_device_ref(device)
}

func (b *Backend) removeDeviceLocked(device *C.struct_ei_device) {
	for _, slot := range []**C.struct_ei_device{&b.pointerAbs, &b.button, &b.scroll, &b.keyboard} {
		if *slot == device {
			C.ei_device_unref(*slot)
			*slot = nil
		}
	}
	if b.pointerAbs == nil {
		b.setRegionsLocked(nil)
	}
	b.statusMu.Lock()
	b.status.Ready = false
	b.status.State = "WAITING_FOR_DEVICES"
	b.statusMu.Unlock()
}

func (b *Backend) withDevice(name string, pick func() *C.struct_ei_device, send func(*C.struct_ei_device)) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ei == nil {
		return errors.New("KWin EIS backend is closed")
	}
	device := pick()
	if device == nil {
		return fmt.Errorf("KWin EIS %s device is not ready", name)
	}
	b.sequence++
	C.ei_device_start_emulating(device, C.uint32_t(b.sequence))
	send(device)
	C.ei_device_frame(device, C.ei_now(b.ei))
	return nil
}

func (b *Backend) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unrefDevicesLocked()
	if b.ei != nil {
		C.ei_unref(b.ei)
		b.ei = nil
	}
	b.closeDone()
}

func (b *Backend) unrefDevicesLocked() {
	for _, device := range []*C.struct_ei_device{b.pointerAbs, b.button, b.scroll, b.keyboard} {
		if device != nil {
			C.ei_device_unref(device)
		}
	}
	b.pointerAbs = nil
	b.button = nil
	b.scroll = nil
	b.keyboard = nil
	b.regions = nil
}

func (b *Backend) setState(state string, err error) {
	b.statusMu.Lock()
	defer b.statusMu.Unlock()
	b.status.State = state
	b.runErr = err
	if err == nil {
		b.status.Error = ""
	} else {
		b.status.Error = err.Error()
	}
}

func (b *Backend) finish(err error) {
	b.statusMu.Lock()
	if err != nil {
		b.runErr = err
		b.status.Error = err.Error()
	} else {
		b.runErr = nil
		b.status.Error = ""
	}
	b.status.Connected = false
	b.status.Active = false
	b.status.Ready = false
	b.status.State = "CLOSED"
	b.statusMu.Unlock()
	b.closeDone()
}

func (b *Backend) closeDone() {
	b.doneOnce.Do(func() {
		close(b.done)
	})
}

func (b *Backend) setRegionsLocked(regions []desktop.Region) {
	b.regions = append([]desktop.Region(nil), regions...)
	union, ok := regionUnion(b.regions)

	b.statusMu.Lock()
	b.status.Regions = append([]desktop.Region(nil), b.regions...)
	if ok {
		b.status.Width = union.W
		b.status.Height = union.H
	} else {
		b.status.Width = 0
		b.status.Height = 0
	}
	b.statusMu.Unlock()
}

func deviceRegions(device *C.struct_ei_device) []desktop.Region {
	var regions []desktop.Region
	for i := 0; ; i++ {
		region := C.ei_device_get_region(device, C.size_t(i))
		if region == nil {
			break
		}
		regions = append(regions, desktop.Region{
			X: int(C.ei_region_get_x(region)),
			Y: int(C.ei_region_get_y(region)),
			W: int(C.ei_region_get_width(region)),
			H: int(C.ei_region_get_height(region)),
		})
	}
	return regions
}

func regionUnion(regions []desktop.Region) (desktop.Region, bool) {
	if len(regions) == 0 {
		return desktop.Region{}, false
	}
	minX := regions[0].X
	minY := regions[0].Y
	maxX := regions[0].X + regions[0].W
	maxY := regions[0].Y + regions[0].H
	for _, region := range regions[1:] {
		minX = min(minX, region.X)
		minY = min(minY, region.Y)
		maxX = max(maxX, region.X+region.W)
		maxY = max(maxY, region.Y+region.H)
	}
	if maxX <= minX || maxY <= minY {
		return desktop.Region{}, false
	}
	return desktop.Region{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}, true
}

func clamp(value int, low int, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
