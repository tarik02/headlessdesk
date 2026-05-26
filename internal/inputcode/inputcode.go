package inputcode

import (
	"fmt"
	"strings"
)

type KeyEvent struct {
	Code uint32
	Down bool
}

type Scancode uint32

func (s Scancode) Uint32() uint32 {
	return uint32(s)
}

func ParseScancode(scancode uint32) (Scancode, error) {
	if scancode == 0 {
		return 0, fmt.Errorf("scancode must be non-zero")
	}
	return Scancode(scancode), nil
}

type KeyName struct {
	canonical string
	code      uint32
}

func ParseKeyName(name string) (KeyName, error) {
	canonical, ok := canonicalKeyName(name)
	if !ok {
		return KeyName{}, fmt.Errorf("unsupported key: %s", name)
	}
	code, ok := keyCodes[canonical]
	if !ok {
		return KeyName{}, fmt.Errorf("unsupported key: %s", name)
	}
	return KeyName{canonical: canonical, code: code}, nil
}

func (k KeyName) String() string {
	return k.canonical
}

func (k KeyName) Code() uint32 {
	return k.code
}

func (k KeyName) Scancode() Scancode {
	return Scancode(k.code)
}

type MouseButtonName struct {
	canonical string
	code      uint32
}

func ParseMouseButtonName(button string) (MouseButtonName, error) {
	canonical, ok := mouseButtonNames[normalize(button)]
	if !ok {
		return MouseButtonName{}, fmt.Errorf("unsupported mouse button: %s", button)
	}
	code, ok := keyCodes[canonical]
	if !ok {
		return MouseButtonName{}, fmt.Errorf("unsupported mouse button: %s", button)
	}
	return MouseButtonName{canonical: canonical, code: code}, nil
}

func (b MouseButtonName) String() string {
	return b.canonical
}

func (b MouseButtonName) Code() uint32 {
	return b.code
}

func Key(name string) (uint32, error) {
	key, err := ParseKeyName(name)
	if err != nil {
		return 0, err
	}
	return key.Code(), nil
}

func MouseButton(button string) (uint32, error) {
	mouseButton, err := ParseMouseButtonName(button)
	if err != nil {
		return 0, err
	}
	return mouseButton.Code(), nil
}

func MouseButtonIndex(button string) (int, error) {
	mouseButton, err := ParseMouseButtonName(button)
	if err != nil {
		return 0, err
	}
	return MouseButtonNameIndex(mouseButton)
}

func MouseButtonNameIndex(button MouseButtonName) (int, error) {
	switch button.Code() {
	case keyCodes["BTN_LEFT"]:
		return 0, nil
	case keyCodes["BTN_RIGHT"]:
		return 1, nil
	case keyCodes["BTN_MIDDLE"]:
		return 2, nil
	case keyCodes["BTN_SIDE"]:
		return 3, nil
	case keyCodes["BTN_EXTRA"]:
		return 4, nil
	case keyCodes["BTN_FORWARD"]:
		return 5, nil
	case keyCodes["BTN_BACK"]:
		return 6, nil
	case keyCodes["BTN_TASK"]:
		return 7, nil
	default:
		return 0, fmt.Errorf("unsupported mouse button code: %d", button.Code())
	}
}

func Text(text string) ([]KeyEvent, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	shift, err := Key("leftshift")
	if err != nil {
		return nil, err
	}

	var events []KeyEvent
	for _, r := range text {
		code, shifted, ok := keyForRune(r)
		if !ok {
			return nil, fmt.Errorf("unsupported text rune: %q", r)
		}
		if shifted {
			events = append(events, KeyEvent{Code: shift, Down: true})
		}
		events = append(events, KeyEvent{Code: code, Down: true}, KeyEvent{Code: code, Down: false})
		if shifted {
			events = append(events, KeyEvent{Code: shift, Down: false})
		}
	}
	return events, nil
}

func canonicalKeyName(name string) (string, bool) {
	normalized := normalize(name)
	if normalized == "" {
		return "", false
	}
	if canonical, ok := keyNames[normalized]; ok {
		return canonical, true
	}
	if strings.HasPrefix(normalized, "key_") || strings.HasPrefix(normalized, "btn_") {
		return strings.ToUpper(normalized), true
	}
	runes := []rune(normalized)
	if len(runes) == 1 {
		r := runes[0]
		switch {
		case r >= 'a' && r <= 'z':
			return "KEY_" + strings.ToUpper(string(r)), true
		case r >= '0' && r <= '9':
			return "KEY_" + string(r), true
		}
	}
	return "", false
}

func keyForRune(r rune) (uint32, bool, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		code, err := Key(string(r))
		return code, false, err == nil
	case r >= 'A' && r <= 'Z':
		code, err := Key(string(r + 'a' - 'A'))
		return code, true, err == nil
	case r >= '0' && r <= '9':
		code, err := Key(string(r))
		return code, false, err == nil
	}

	mapping, ok := runeKeys[r]
	if !ok {
		return 0, false, false
	}
	code, err := Key(mapping.key)
	return code, mapping.shifted, err == nil
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

var mouseButtonNames = map[string]string{
	"left":     "BTN_LEFT",
	"button1":  "BTN_LEFT",
	"1":        "BTN_LEFT",
	"right":    "BTN_RIGHT",
	"button3":  "BTN_RIGHT",
	"3":        "BTN_RIGHT",
	"middle":   "BTN_MIDDLE",
	"button2":  "BTN_MIDDLE",
	"2":        "BTN_MIDDLE",
	"side":     "BTN_SIDE",
	"button4":  "BTN_SIDE",
	"4":        "BTN_SIDE",
	"extra":    "BTN_EXTRA",
	"button5":  "BTN_EXTRA",
	"5":        "BTN_EXTRA",
	"forward":  "BTN_FORWARD",
	"button9":  "BTN_FORWARD",
	"9":        "BTN_FORWARD",
	"back":     "BTN_BACK",
	"button8":  "BTN_BACK",
	"8":        "BTN_BACK",
	"task":     "BTN_TASK",
	"button10": "BTN_TASK",
	"10":       "BTN_TASK",
}

var keyNames = map[string]string{
	"esc":          "KEY_ESC",
	"escape":       "KEY_ESC",
	"minus":        "KEY_MINUS",
	"-":            "KEY_MINUS",
	"equal":        "KEY_EQUAL",
	"equals":       "KEY_EQUAL",
	"=":            "KEY_EQUAL",
	"backspace":    "KEY_BACKSPACE",
	"back":         "KEY_BACKSPACE",
	"tab":          "KEY_TAB",
	"leftbrace":    "KEY_LEFTBRACE",
	"leftbracket":  "KEY_LEFTBRACE",
	"[":            "KEY_LEFTBRACE",
	"rightbrace":   "KEY_RIGHTBRACE",
	"rightbracket": "KEY_RIGHTBRACE",
	"]":            "KEY_RIGHTBRACE",
	"enter":        "KEY_ENTER",
	"return":       "KEY_ENTER",
	"newline":      "KEY_ENTER",
	"semicolon":    "KEY_SEMICOLON",
	";":            "KEY_SEMICOLON",
	"apostrophe":   "KEY_APOSTROPHE",
	"quote":        "KEY_APOSTROPHE",
	"'":            "KEY_APOSTROPHE",
	"grave":        "KEY_GRAVE",
	"backtick":     "KEY_GRAVE",
	"`":            "KEY_GRAVE",
	"backslash":    "KEY_BACKSLASH",
	"\\":           "KEY_BACKSLASH",
	"comma":        "KEY_COMMA",
	",":            "KEY_COMMA",
	"dot":          "KEY_DOT",
	"period":       "KEY_DOT",
	".":            "KEY_DOT",
	"slash":        "KEY_SLASH",
	"/":            "KEY_SLASH",
	"space":        "KEY_SPACE",
	"spacebar":     "KEY_SPACE",
	"capslock":     "KEY_CAPSLOCK",
	"caps":         "KEY_CAPSLOCK",
	"capital":      "KEY_CAPSLOCK",
	"numlock":      "KEY_NUMLOCK",
	"scrolllock":   "KEY_SCROLLLOCK",
	"scroll":       "KEY_SCROLLLOCK",
	"printscreen":  "KEY_SYSRQ",
	"prtsc":        "KEY_SYSRQ",
	"print":        "KEY_SYSRQ",
	"sysrq":        "KEY_SYSRQ",
	"snapshot":     "KEY_SYSRQ",
	"home":         "KEY_HOME",
	"up":           "KEY_UP",
	"arrowup":      "KEY_UP",
	"pageup":       "KEY_PAGEUP",
	"page_up":      "KEY_PAGEUP",
	"pgup":         "KEY_PAGEUP",
	"prior":        "KEY_PAGEUP",
	"left":         "KEY_LEFT",
	"arrowleft":    "KEY_LEFT",
	"right":        "KEY_RIGHT",
	"arrowright":   "KEY_RIGHT",
	"end":          "KEY_END",
	"down":         "KEY_DOWN",
	"arrowdown":    "KEY_DOWN",
	"pagedown":     "KEY_PAGEDOWN",
	"page_down":    "KEY_PAGEDOWN",
	"pgdn":         "KEY_PAGEDOWN",
	"next":         "KEY_PAGEDOWN",
	"insert":       "KEY_INSERT",
	"ins":          "KEY_INSERT",
	"delete":       "KEY_DELETE",
	"del":          "KEY_DELETE",
	"pause":        "KEY_PAUSE",
	"leftmeta":     "KEY_LEFTMETA",
	"leftsuper":    "KEY_LEFTMETA",
	"leftwin":      "KEY_LEFTMETA",
	"lwin":         "KEY_LEFTMETA",
	"super":        "KEY_LEFTMETA",
	"meta":         "KEY_LEFTMETA",
	"win":          "KEY_LEFTMETA",
	"windows":      "KEY_LEFTMETA",
	"cmd":          "KEY_LEFTMETA",
	"command":      "KEY_LEFTMETA",
	"rightmeta":    "KEY_RIGHTMETA",
	"rightsuper":   "KEY_RIGHTMETA",
	"rightwin":     "KEY_RIGHTMETA",
	"rwin":         "KEY_RIGHTMETA",
	"compose":      "KEY_COMPOSE",
	"menu":         "KEY_MENU",
	"apps":         "KEY_MENU",
	"application":  "KEY_MENU",
	"contextmenu":  "KEY_MENU",
	"shift":        "KEY_LEFTSHIFT",
	"leftshift":    "KEY_LEFTSHIFT",
	"lshift":       "KEY_LEFTSHIFT",
	"rightshift":   "KEY_RIGHTSHIFT",
	"rshift":       "KEY_RIGHTSHIFT",
	"leftctrl":     "KEY_LEFTCTRL",
	"leftcontrol":  "KEY_LEFTCTRL",
	"lctrl":        "KEY_LEFTCTRL",
	"lcontrol":     "KEY_LEFTCTRL",
	"ctrl":         "KEY_LEFTCTRL",
	"control":      "KEY_LEFTCTRL",
	"rightctrl":    "KEY_RIGHTCTRL",
	"rightcontrol": "KEY_RIGHTCTRL",
	"rctrl":        "KEY_RIGHTCTRL",
	"rcontrol":     "KEY_RIGHTCTRL",
	"leftalt":      "KEY_LEFTALT",
	"lalt":         "KEY_LEFTALT",
	"lmenu":        "KEY_LEFTALT",
	"alt":          "KEY_LEFTALT",
	"option":       "KEY_LEFTALT",
	"rightalt":     "KEY_RIGHTALT",
	"ralt":         "KEY_RIGHTALT",
	"rmenu":        "KEY_RIGHTALT",
	"rightoption":  "KEY_RIGHTALT",
	"kpenter":      "KEY_KPENTER",
	"numenter":     "KEY_KPENTER",
	"keypadenter":  "KEY_KPENTER",
	"kpslash":      "KEY_KPSLASH",
	"kpdivide":     "KEY_KPSLASH",
	"kpasterisk":   "KEY_KPASTERISK",
	"kpmultiply":   "KEY_KPASTERISK",
	"kpminus":      "KEY_KPMINUS",
	"kpsubtract":   "KEY_KPMINUS",
	"kpplus":       "KEY_KPPLUS",
	"kpadd":        "KEY_KPPLUS",
	"kpdot":        "KEY_KPDOT",
	"kpdecimal":    "KEY_KPDOT",
	"kpcomma":      "KEY_KPCOMMA",
	"kpequal":      "KEY_KPEQUAL",
	"mute":         "KEY_MUTE",
	"volumemute":   "KEY_MUTE",
	"volumedown":   "KEY_VOLUMEDOWN",
	"volume_down":  "KEY_VOLUMEDOWN",
	"volumeup":     "KEY_VOLUMEUP",
	"volume_up":    "KEY_VOLUMEUP",
	"power":        "KEY_POWER",
	"sleep":        "KEY_SLEEP",
	"wakeup":       "KEY_WAKEUP",
	"nextsong":     "KEY_NEXTSONG",
	"nexttrack":    "KEY_NEXTSONG",
	"playpause":    "KEY_PLAYPAUSE",
	"mediaplay":    "KEY_PLAYPAUSE",
	"previoussong": "KEY_PREVIOUSSONG",
	"prevsong":     "KEY_PREVIOUSSONG",
	"prevtrack":    "KEY_PREVIOUSSONG",
	"stopcd":       "KEY_STOPCD",
	"mediastop":    "KEY_STOPCD",
	"ejectcd":      "KEY_EJECTCD",
	"eject":        "KEY_EJECTCD",
}

var runeKeys = map[rune]struct {
	key     string
	shifted bool
}{
	' ':  {"space", false},
	'\n': {"enter", false},
	'\r': {"enter", false},
	'\t': {"tab", false},
	'-':  {"-", false},
	'_':  {"-", true},
	'=':  {"=", false},
	'+':  {"=", true},
	'[':  {"[", false},
	'{':  {"[", true},
	']':  {"]", false},
	'}':  {"]", true},
	';':  {";", false},
	':':  {";", true},
	'\'': {"'", false},
	'"':  {"'", true},
	'`':  {"`", false},
	'~':  {"`", true},
	'\\': {"\\", false},
	'|':  {"\\", true},
	',':  {",", false},
	'<':  {",", true},
	'.':  {".", false},
	'>':  {".", true},
	'/':  {"/", false},
	'?':  {"/", true},
	'!':  {"1", true},
	'@':  {"2", true},
	'#':  {"3", true},
	'$':  {"4", true},
	'%':  {"5", true},
	'^':  {"6", true},
	'&':  {"7", true},
	'*':  {"8", true},
	'(':  {"9", true},
	')':  {"0", true},
}

func init() {
	for i := 1; i <= 24; i++ {
		keyNames[fmt.Sprintf("f%d", i)] = fmt.Sprintf("KEY_F%d", i)
	}
	for i := 0; i <= 9; i++ {
		keyNames[fmt.Sprintf("kp%d", i)] = fmt.Sprintf("KEY_KP%d", i)
		keyNames[fmt.Sprintf("num%d", i)] = fmt.Sprintf("KEY_KP%d", i)
		keyNames[fmt.Sprintf("numpad%d", i)] = fmt.Sprintf("KEY_KP%d", i)
	}
}
