package ui

import (
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// autosaveSlot is grout's canonical default slot. It's shown by its real name in slot
// pickers (not relabeled "Default") so it can't be confused with a server slot literally
// named "default"/"Default".
const autosaveSlot = settings.DefaultSaveSlot

// SlotOptionsResult holds the built slot options and the pre-selected index.
type SlotOptionsResult struct {
	Options     []gaba.Option
	SelectedIdx int
}

// BuildSlotOptions builds the list of slot options for a game's slot selector.
// It includes all existing slot names (shown by their real names, including the
// canonical "autosave" slot) and a "New Slot..." keyboard option for creating new slots.
// The selected index is set to the current slot preference.
func BuildSlotOptions(config *settings.Config, romID int, slotNames []string) SlotOptionsResult {
	newSlotLabel := localize("game_options_new_slot", "New Slot...")

	options := make([]gaba.Option, 0, len(slotNames)+2)

	if len(slotNames) == 0 {
		options = append(options, gaba.Option{DisplayName: autosaveSlot, Value: autosaveSlot})
	} else {
		for _, name := range slotNames {
			options = append(options, gaba.Option{DisplayName: name, Value: name})
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
