package muos

import (
	"embed"
	"encoding/json"
	"strings"

	"grout/core/firmware/host"
)

//go:embed input/anbernic.json
var embeddedInput embed.FS

const sdlMapFile = "/opt/muos/device/config/board/sdl_map"

const (
	buttonUp = iota + 1
	buttonDown
	buttonLeft
	buttonRight
	buttonA
	buttonB
	buttonX
	buttonY
	buttonL1
	buttonL2
	buttonR1
	buttonR2
	buttonStart
	buttonSelect
	buttonMenu
)

var sdlButtons = map[string]int{
	"a": buttonA, "b": buttonB, "x": buttonX, "y": buttonY,
	"leftshoulder": buttonL1, "rightshoulder": buttonR1,
	"lefttrigger": buttonL2, "righttrigger": buttonR2,
	"start": buttonStart, "back": buttonSelect, "guide": buttonMenu,
}

var sdlHats = map[string]int{"dpup": buttonUp, "dpdown": buttonDown, "dpleft": buttonLeft, "dpright": buttonRight}

func SDLMapInput(h host.Host) ([]byte, error) {
	if line, err := h.ReadFile(sdlMapFile); err == nil {
		if mapping, ok := mappingFromSDLMap(strings.TrimSpace(string(line))); ok {
			return mapping, nil
		}
	}
	return embeddedInput.ReadFile("input/anbernic.json")
}

func mappingFromSDLMap(line string) ([]byte, bool) {
	fields := strings.Split(line, ",")
	if len(fields) < 3 {
		return nil, false
	}
	buttons := map[string]int{}
	hats := map[string]int{}
	for _, field := range fields[2:] {
		name, value, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		if button, known := sdlButtons[name]; known && strings.HasPrefix(value, "b") {
			buttons[strings.TrimPrefix(value, "b")] = button
		}
		if button, known := sdlHats[name]; known && strings.HasPrefix(value, "h0.") {
			hats[strings.TrimPrefix(value, "h0.")] = button
		}
	}
	if len(buttons) == 0 {
		return nil, false
	}
	mapping, err := json.Marshal(map[string]any{
		"keyboard_map":          map[string]int{},
		"controller_button_map": map[string]int{},
		"controller_hat_map":    map[string]int{},
		"joystick_axis_map":     map[string]int{},
		"joystick_button_map":   buttons,
		"joystick_hat_map":      hats,
	})
	return mapping, err == nil
}
