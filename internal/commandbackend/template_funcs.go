package commandbackend

import (
	"fmt"
	"math"

	"headlessdesk/internal/inputcode"
)

func ydotoolButton(button string) (int, error) {
	name, err := inputcode.ParseMouseButtonName(button)
	if err != nil {
		return 0, err
	}
	return inputcode.MouseButtonNameIndex(name)
}

func ydotoolButtonEvent(button string, down bool) (string, error) {
	code, err := ydotoolButton(button)
	if err != nil {
		return "", err
	}
	if down {
		return fmt.Sprintf("0x%X", 0x40|code), nil
	}
	return fmt.Sprintf("0x%X", 0x80|code), nil
}

func ydotoolWheelX(delta int, horizontal bool) int {
	if !horizontal {
		return 0
	}
	return wheelSteps(delta)
}

func ydotoolWheelY(delta int, horizontal bool) int {
	if horizontal {
		return 0
	}
	return wheelSteps(delta)
}

func wheelSteps(delta int) int {
	if delta == 0 {
		return 0
	}
	steps := delta / 120
	if steps != 0 {
		return steps
	}
	return int(math.Copysign(1, float64(delta)))
}

func ydotoolKeyEvent(key string, down bool) (string, error) {
	code, err := ydotoolKey(key)
	if err != nil {
		return "", err
	}
	if down {
		return fmt.Sprintf("%d:1", code), nil
	}
	return fmt.Sprintf("%d:0", code), nil
}

func ydotoolKey(key string) (int, error) {
	code, err := inputcode.Key(key)
	if err != nil {
		return 0, err
	}
	return int(code), nil
}
