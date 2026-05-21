package backendpreset

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed presets/*.yaml
var presets embed.FS

func Load(name string) ([]byte, bool, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil, false, nil
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, false, fmt.Errorf("invalid preset name %q", name)
	}

	data, err := presets.ReadFile("presets/" + name + ".yaml")
	if err != nil {
		return nil, false, nil
	}
	return data, true, nil
}
