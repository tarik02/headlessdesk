//go:build darwin

package macoslocal

import (
	"fmt"

	"headlessdesk/internal/inputcode"
)

func macVirtualKey(code uint32) (uint16, error) {
	key, ok := macVirtualKeys[code]
	if !ok {
		return 0, fmt.Errorf("unsupported macOS key: %d", code)
	}
	return key, nil
}

func mustKeyCode(name string) uint32 {
	code, err := inputcode.Key(name)
	if err != nil {
		panic(err)
	}
	return code
}

var macVirtualKeys = map[uint32]uint16{
	mustKeyCode("a"):              0x00,
	mustKeyCode("s"):              0x01,
	mustKeyCode("d"):              0x02,
	mustKeyCode("f"):              0x03,
	mustKeyCode("h"):              0x04,
	mustKeyCode("g"):              0x05,
	mustKeyCode("z"):              0x06,
	mustKeyCode("x"):              0x07,
	mustKeyCode("c"):              0x08,
	mustKeyCode("v"):              0x09,
	mustKeyCode("b"):              0x0b,
	mustKeyCode("q"):              0x0c,
	mustKeyCode("w"):              0x0d,
	mustKeyCode("e"):              0x0e,
	mustKeyCode("r"):              0x0f,
	mustKeyCode("y"):              0x10,
	mustKeyCode("t"):              0x11,
	mustKeyCode("1"):              0x12,
	mustKeyCode("2"):              0x13,
	mustKeyCode("3"):              0x14,
	mustKeyCode("4"):              0x15,
	mustKeyCode("6"):              0x16,
	mustKeyCode("5"):              0x17,
	mustKeyCode("="):              0x18,
	mustKeyCode("9"):              0x19,
	mustKeyCode("7"):              0x1a,
	mustKeyCode("-"):              0x1b,
	mustKeyCode("8"):              0x1c,
	mustKeyCode("0"):              0x1d,
	mustKeyCode("]"):              0x1e,
	mustKeyCode("o"):              0x1f,
	mustKeyCode("u"):              0x20,
	mustKeyCode("["):              0x21,
	mustKeyCode("i"):              0x22,
	mustKeyCode("p"):              0x23,
	mustKeyCode("enter"):          0x24,
	mustKeyCode("l"):              0x25,
	mustKeyCode("j"):              0x26,
	mustKeyCode("'"):              0x27,
	mustKeyCode("k"):              0x28,
	mustKeyCode(";"):              0x29,
	mustKeyCode("\\"):             0x2a,
	mustKeyCode(","):              0x2b,
	mustKeyCode("/"):              0x2c,
	mustKeyCode("n"):              0x2d,
	mustKeyCode("m"):              0x2e,
	mustKeyCode("."):              0x2f,
	mustKeyCode("tab"):            0x30,
	mustKeyCode("space"):          0x31,
	mustKeyCode("`"):              0x32,
	mustKeyCode("backspace"):      0x33,
	mustKeyCode("esc"):            0x35,
	mustKeyCode("rightmeta"):      0x36,
	mustKeyCode("leftmeta"):       0x37,
	mustKeyCode("leftshift"):      0x38,
	mustKeyCode("capslock"):       0x39,
	mustKeyCode("leftalt"):        0x3a,
	mustKeyCode("leftctrl"):       0x3b,
	mustKeyCode("rightshift"):     0x3c,
	mustKeyCode("rightalt"):       0x3d,
	mustKeyCode("rightctrl"):      0x3e,
	mustKeyCode("KEY_F17"):        0x40,
	mustKeyCode("KEY_KPDOT"):      0x41,
	mustKeyCode("KEY_KPASTERISK"): 0x43,
	mustKeyCode("KEY_KPPLUS"):     0x45,
	mustKeyCode("volumeup"):       0x48,
	mustKeyCode("volumedown"):     0x49,
	mustKeyCode("mute"):           0x4a,
	mustKeyCode("KEY_KPSLASH"):    0x4b,
	mustKeyCode("KEY_KPENTER"):    0x4c,
	mustKeyCode("KEY_KPMINUS"):    0x4e,
	mustKeyCode("KEY_F18"):        0x4f,
	mustKeyCode("KEY_F19"):        0x50,
	mustKeyCode("KEY_KPEQUAL"):    0x51,
	mustKeyCode("KEY_KP0"):        0x52,
	mustKeyCode("KEY_KP1"):        0x53,
	mustKeyCode("KEY_KP2"):        0x54,
	mustKeyCode("KEY_KP3"):        0x55,
	mustKeyCode("KEY_KP4"):        0x56,
	mustKeyCode("KEY_KP5"):        0x57,
	mustKeyCode("KEY_KP6"):        0x58,
	mustKeyCode("KEY_KP7"):        0x59,
	mustKeyCode("KEY_F20"):        0x5a,
	mustKeyCode("KEY_KP8"):        0x5b,
	mustKeyCode("KEY_KP9"):        0x5c,
	mustKeyCode("KEY_F5"):         0x60,
	mustKeyCode("KEY_F6"):         0x61,
	mustKeyCode("KEY_F7"):         0x62,
	mustKeyCode("KEY_F3"):         0x63,
	mustKeyCode("KEY_F8"):         0x64,
	mustKeyCode("KEY_F9"):         0x65,
	mustKeyCode("KEY_F11"):        0x67,
	mustKeyCode("KEY_F13"):        0x69,
	mustKeyCode("KEY_F16"):        0x6a,
	mustKeyCode("KEY_F14"):        0x6b,
	mustKeyCode("KEY_F10"):        0x6d,
	mustKeyCode("KEY_F12"):        0x6f,
	mustKeyCode("KEY_F15"):        0x71,
	mustKeyCode("insert"):         0x72,
	mustKeyCode("home"):           0x73,
	mustKeyCode("pageup"):         0x74,
	mustKeyCode("delete"):         0x75,
	mustKeyCode("KEY_F4"):         0x76,
	mustKeyCode("end"):            0x77,
	mustKeyCode("KEY_F2"):         0x78,
	mustKeyCode("pagedown"):       0x79,
	mustKeyCode("KEY_F1"):         0x7a,
	mustKeyCode("left"):           0x7b,
	mustKeyCode("right"):          0x7c,
	mustKeyCode("down"):           0x7d,
	mustKeyCode("up"):             0x7e,
}
