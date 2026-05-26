//go:build windows

package inputcode

import (
	"fmt"

	"github.com/zzl/go-win32api/v2/win32"
)

const (
	windowsXButton1 = uint32(win32.XBUTTON1)
	windowsXButton2 = uint32(win32.XBUTTON2)
)

type WindowsKey struct {
	VirtualKey win32.VIRTUAL_KEY
	Extended   bool
}

type WindowsMouseButton struct {
	DownFlags win32.MOUSE_EVENT_FLAGS
	UpFlags   win32.MOUSE_EVENT_FLAGS
	Data      uint32
}

func WindowsVirtualKey(name KeyName) (WindowsKey, error) {
	key, ok := windowsVirtualKeys[name.Code()]
	if !ok {
		return WindowsKey{}, fmt.Errorf("unsupported Windows key: %s", name)
	}
	return key, nil
}

func WindowsMouseButtonEvent(button MouseButtonName) (WindowsMouseButton, error) {
	event, ok := windowsMouseButtons[button.Code()]
	if !ok {
		return WindowsMouseButton{}, fmt.Errorf("unsupported Windows mouse button: %s", button)
	}
	return event, nil
}

func windowsDigitKey(digit rune) win32.VIRTUAL_KEY {
	return win32.VIRTUAL_KEY(digit)
}

func windowsLetterKey(letter rune) win32.VIRTUAL_KEY {
	return win32.VIRTUAL_KEY(letter)
}

func mustKeyCode(name string) uint32 {
	code, err := Key(name)
	if err != nil {
		panic(err)
	}
	return code
}

var windowsMouseButtons = map[uint32]WindowsMouseButton{
	mustKeyCode("BTN_LEFT"): {
		DownFlags: win32.MOUSEEVENTF_LEFTDOWN,
		UpFlags:   win32.MOUSEEVENTF_LEFTUP,
	},
	mustKeyCode("BTN_RIGHT"): {
		DownFlags: win32.MOUSEEVENTF_RIGHTDOWN,
		UpFlags:   win32.MOUSEEVENTF_RIGHTUP,
	},
	mustKeyCode("BTN_MIDDLE"): {
		DownFlags: win32.MOUSEEVENTF_MIDDLEDOWN,
		UpFlags:   win32.MOUSEEVENTF_MIDDLEUP,
	},
	mustKeyCode("BTN_SIDE"): {
		DownFlags: win32.MOUSEEVENTF_XDOWN,
		UpFlags:   win32.MOUSEEVENTF_XUP,
		Data:      windowsXButton1,
	},
	mustKeyCode("BTN_BACK"): {
		DownFlags: win32.MOUSEEVENTF_XDOWN,
		UpFlags:   win32.MOUSEEVENTF_XUP,
		Data:      windowsXButton1,
	},
	mustKeyCode("BTN_EXTRA"): {
		DownFlags: win32.MOUSEEVENTF_XDOWN,
		UpFlags:   win32.MOUSEEVENTF_XUP,
		Data:      windowsXButton2,
	},
	mustKeyCode("BTN_FORWARD"): {
		DownFlags: win32.MOUSEEVENTF_XDOWN,
		UpFlags:   win32.MOUSEEVENTF_XUP,
		Data:      windowsXButton2,
	},
}

var windowsVirtualKeys = map[uint32]WindowsKey{
	mustKeyCode("esc"):            {VirtualKey: win32.VK_ESCAPE},
	mustKeyCode("1"):              {VirtualKey: windowsDigitKey('1')},
	mustKeyCode("2"):              {VirtualKey: windowsDigitKey('2')},
	mustKeyCode("3"):              {VirtualKey: windowsDigitKey('3')},
	mustKeyCode("4"):              {VirtualKey: windowsDigitKey('4')},
	mustKeyCode("5"):              {VirtualKey: windowsDigitKey('5')},
	mustKeyCode("6"):              {VirtualKey: windowsDigitKey('6')},
	mustKeyCode("7"):              {VirtualKey: windowsDigitKey('7')},
	mustKeyCode("8"):              {VirtualKey: windowsDigitKey('8')},
	mustKeyCode("9"):              {VirtualKey: windowsDigitKey('9')},
	mustKeyCode("0"):              {VirtualKey: windowsDigitKey('0')},
	mustKeyCode("-"):              {VirtualKey: win32.VK_OEM_MINUS},
	mustKeyCode("="):              {VirtualKey: win32.VK_OEM_PLUS},
	mustKeyCode("backspace"):      {VirtualKey: win32.VK_BACK},
	mustKeyCode("tab"):            {VirtualKey: win32.VK_TAB},
	mustKeyCode("q"):              {VirtualKey: windowsLetterKey('Q')},
	mustKeyCode("w"):              {VirtualKey: windowsLetterKey('W')},
	mustKeyCode("e"):              {VirtualKey: windowsLetterKey('E')},
	mustKeyCode("r"):              {VirtualKey: windowsLetterKey('R')},
	mustKeyCode("t"):              {VirtualKey: windowsLetterKey('T')},
	mustKeyCode("y"):              {VirtualKey: windowsLetterKey('Y')},
	mustKeyCode("u"):              {VirtualKey: windowsLetterKey('U')},
	mustKeyCode("i"):              {VirtualKey: windowsLetterKey('I')},
	mustKeyCode("o"):              {VirtualKey: windowsLetterKey('O')},
	mustKeyCode("p"):              {VirtualKey: windowsLetterKey('P')},
	mustKeyCode("["):              {VirtualKey: win32.VK_OEM_4},
	mustKeyCode("]"):              {VirtualKey: win32.VK_OEM_6},
	mustKeyCode("enter"):          {VirtualKey: win32.VK_RETURN},
	mustKeyCode("leftctrl"):       {VirtualKey: win32.VK_LCONTROL},
	mustKeyCode("a"):              {VirtualKey: windowsLetterKey('A')},
	mustKeyCode("s"):              {VirtualKey: windowsLetterKey('S')},
	mustKeyCode("d"):              {VirtualKey: windowsLetterKey('D')},
	mustKeyCode("f"):              {VirtualKey: windowsLetterKey('F')},
	mustKeyCode("g"):              {VirtualKey: windowsLetterKey('G')},
	mustKeyCode("h"):              {VirtualKey: windowsLetterKey('H')},
	mustKeyCode("j"):              {VirtualKey: windowsLetterKey('J')},
	mustKeyCode("k"):              {VirtualKey: windowsLetterKey('K')},
	mustKeyCode("l"):              {VirtualKey: windowsLetterKey('L')},
	mustKeyCode(";"):              {VirtualKey: win32.VK_OEM_1},
	mustKeyCode("'"):              {VirtualKey: win32.VK_OEM_7},
	mustKeyCode("`"):              {VirtualKey: win32.VK_OEM_3},
	mustKeyCode("leftshift"):      {VirtualKey: win32.VK_LSHIFT},
	mustKeyCode("\\"):             {VirtualKey: win32.VK_OEM_5},
	mustKeyCode("z"):              {VirtualKey: windowsLetterKey('Z')},
	mustKeyCode("x"):              {VirtualKey: windowsLetterKey('X')},
	mustKeyCode("c"):              {VirtualKey: windowsLetterKey('C')},
	mustKeyCode("v"):              {VirtualKey: windowsLetterKey('V')},
	mustKeyCode("b"):              {VirtualKey: windowsLetterKey('B')},
	mustKeyCode("n"):              {VirtualKey: windowsLetterKey('N')},
	mustKeyCode("m"):              {VirtualKey: windowsLetterKey('M')},
	mustKeyCode(","):              {VirtualKey: win32.VK_OEM_COMMA},
	mustKeyCode("."):              {VirtualKey: win32.VK_OEM_PERIOD},
	mustKeyCode("/"):              {VirtualKey: win32.VK_OEM_2},
	mustKeyCode("rightshift"):     {VirtualKey: win32.VK_RSHIFT},
	mustKeyCode("KEY_KPASTERISK"): {VirtualKey: win32.VK_MULTIPLY},
	mustKeyCode("leftalt"):        {VirtualKey: win32.VK_LMENU},
	mustKeyCode("space"):          {VirtualKey: win32.VK_SPACE},
	mustKeyCode("capslock"):       {VirtualKey: win32.VK_CAPITAL},
	mustKeyCode("KEY_F1"):         {VirtualKey: win32.VK_F1},
	mustKeyCode("KEY_F2"):         {VirtualKey: win32.VK_F2},
	mustKeyCode("KEY_F3"):         {VirtualKey: win32.VK_F3},
	mustKeyCode("KEY_F4"):         {VirtualKey: win32.VK_F4},
	mustKeyCode("KEY_F5"):         {VirtualKey: win32.VK_F5},
	mustKeyCode("KEY_F6"):         {VirtualKey: win32.VK_F6},
	mustKeyCode("KEY_F7"):         {VirtualKey: win32.VK_F7},
	mustKeyCode("KEY_F8"):         {VirtualKey: win32.VK_F8},
	mustKeyCode("KEY_F9"):         {VirtualKey: win32.VK_F9},
	mustKeyCode("KEY_F10"):        {VirtualKey: win32.VK_F10},
	mustKeyCode("numlock"):        {VirtualKey: win32.VK_NUMLOCK},
	mustKeyCode("scrolllock"):     {VirtualKey: win32.VK_SCROLL},
	mustKeyCode("KEY_KP7"):        {VirtualKey: win32.VK_NUMPAD7},
	mustKeyCode("KEY_KP8"):        {VirtualKey: win32.VK_NUMPAD8},
	mustKeyCode("KEY_KP9"):        {VirtualKey: win32.VK_NUMPAD9},
	mustKeyCode("KEY_KPMINUS"):    {VirtualKey: win32.VK_SUBTRACT},
	mustKeyCode("KEY_KP4"):        {VirtualKey: win32.VK_NUMPAD4},
	mustKeyCode("KEY_KP5"):        {VirtualKey: win32.VK_NUMPAD5},
	mustKeyCode("KEY_KP6"):        {VirtualKey: win32.VK_NUMPAD6},
	mustKeyCode("KEY_KPPLUS"):     {VirtualKey: win32.VK_ADD},
	mustKeyCode("KEY_KP1"):        {VirtualKey: win32.VK_NUMPAD1},
	mustKeyCode("KEY_KP2"):        {VirtualKey: win32.VK_NUMPAD2},
	mustKeyCode("KEY_KP3"):        {VirtualKey: win32.VK_NUMPAD3},
	mustKeyCode("KEY_KP0"):        {VirtualKey: win32.VK_NUMPAD0},
	mustKeyCode("KEY_KPDOT"):      {VirtualKey: win32.VK_DECIMAL},
	mustKeyCode("KEY_F11"):        {VirtualKey: win32.VK_F11},
	mustKeyCode("KEY_F12"):        {VirtualKey: win32.VK_F12},
	mustKeyCode("KEY_KPENTER"):    {VirtualKey: win32.VK_RETURN, Extended: true},
	mustKeyCode("rightctrl"):      {VirtualKey: win32.VK_RCONTROL, Extended: true},
	mustKeyCode("KEY_KPSLASH"):    {VirtualKey: win32.VK_DIVIDE, Extended: true},
	mustKeyCode("printscreen"):    {VirtualKey: win32.VK_SNAPSHOT, Extended: true},
	mustKeyCode("rightalt"):       {VirtualKey: win32.VK_RMENU, Extended: true},
	mustKeyCode("home"):           {VirtualKey: win32.VK_HOME, Extended: true},
	mustKeyCode("up"):             {VirtualKey: win32.VK_UP, Extended: true},
	mustKeyCode("pageup"):         {VirtualKey: win32.VK_PRIOR, Extended: true},
	mustKeyCode("left"):           {VirtualKey: win32.VK_LEFT, Extended: true},
	mustKeyCode("right"):          {VirtualKey: win32.VK_RIGHT, Extended: true},
	mustKeyCode("end"):            {VirtualKey: win32.VK_END, Extended: true},
	mustKeyCode("down"):           {VirtualKey: win32.VK_DOWN, Extended: true},
	mustKeyCode("pagedown"):       {VirtualKey: win32.VK_NEXT, Extended: true},
	mustKeyCode("insert"):         {VirtualKey: win32.VK_INSERT, Extended: true},
	mustKeyCode("delete"):         {VirtualKey: win32.VK_DELETE, Extended: true},
	mustKeyCode("mute"):           {VirtualKey: win32.VK_VOLUME_MUTE},
	mustKeyCode("volumedown"):     {VirtualKey: win32.VK_VOLUME_DOWN},
	mustKeyCode("volumeup"):       {VirtualKey: win32.VK_VOLUME_UP},
	mustKeyCode("pause"):          {VirtualKey: win32.VK_PAUSE},
	mustKeyCode("leftmeta"):       {VirtualKey: win32.VK_LWIN, Extended: true},
	mustKeyCode("rightmeta"):      {VirtualKey: win32.VK_RWIN, Extended: true},
	mustKeyCode("menu"):           {VirtualKey: win32.VK_APPS, Extended: true},
	mustKeyCode("sleep"):          {VirtualKey: win32.VK_SLEEP},
	mustKeyCode("nextsong"):       {VirtualKey: win32.VK_MEDIA_NEXT_TRACK},
	mustKeyCode("playpause"):      {VirtualKey: win32.VK_MEDIA_PLAY_PAUSE},
	mustKeyCode("previoussong"):   {VirtualKey: win32.VK_MEDIA_PREV_TRACK},
	mustKeyCode("stopcd"):         {VirtualKey: win32.VK_MEDIA_STOP},
	mustKeyCode("KEY_F13"):        {VirtualKey: win32.VK_F13},
	mustKeyCode("KEY_F14"):        {VirtualKey: win32.VK_F14},
	mustKeyCode("KEY_F15"):        {VirtualKey: win32.VK_F15},
	mustKeyCode("KEY_F16"):        {VirtualKey: win32.VK_F16},
	mustKeyCode("KEY_F17"):        {VirtualKey: win32.VK_F17},
	mustKeyCode("KEY_F18"):        {VirtualKey: win32.VK_F18},
	mustKeyCode("KEY_F19"):        {VirtualKey: win32.VK_F19},
	mustKeyCode("KEY_F20"):        {VirtualKey: win32.VK_F20},
	mustKeyCode("KEY_F21"):        {VirtualKey: win32.VK_F21},
	mustKeyCode("KEY_F22"):        {VirtualKey: win32.VK_F22},
	mustKeyCode("KEY_F23"):        {VirtualKey: win32.VK_F23},
	mustKeyCode("KEY_F24"):        {VirtualKey: win32.VK_F24},
}
