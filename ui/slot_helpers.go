package ui

import (
	"grout/internal"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// SlotOptionsResult holds the built slot options and the pre-selected index.
type SlotOptionsResult struct {
	Options     []gaba.Option
	SelectedIdx int
}

// BuildSlotOptions builds the list of slot options for a game's slot selector.
// It includes all existing slot names, a "Default" label for the default slot,
// and a "New Slot..." keyboard option for creating new slots.
// The selected index is set to the current slot preference.
func BuildSlotOptions(config *internal.Config, romID int, slotNames []string) SlotOptionsResult {
	defaultLabel := i18n.Localize(&goi18n.Message{ID: "common_default", Other: "Default"}, nil)
	newSlotLabel := i18n.Localize(&goi18n.Message{ID: "game_options_new_slot", Other: "New Slot..."}, nil)

	options := make([]gaba.Option, 0, len(slotNames)+2)

	if len(slotNames) == 0 {
		options = append(options, gaba.Option{DisplayName: defaultLabel, Value: "default"})
	} else {
		for _, name := range slotNames {
			displayName := name
			if name == "default" {
				displayName = defaultLabel
			}
			options = append(options, gaba.Option{DisplayName: displayName, Value: name})
		}
	}

	options = append(options, gaba.Option{
		DisplayName:    newSlotLabel,
		Value:          "",
		Type:           gaba.OptionTypeKeyboard,
		KeyboardPrompt: "",
	})

	currentPref := config.GetSlotPreference(romID)
	selectedIdx := 0
	for i, opt := range options {
		if val, ok := opt.Value.(string); ok && val == currentPref {
			selectedIdx = i
			break
		}
	}

	return SlotOptionsResult{
		Options:     options,
		SelectedIdx: selectedIdx,
	}
}
