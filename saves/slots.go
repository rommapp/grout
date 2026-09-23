package saves

import (
	"grout/romm"
	"grout/settings"
)

// SlotName is the slot a save belongs to.
//
// The server leaves the field unset for a game's only slot, and sometimes
// sends it empty, which means the same thing. Both read as grout's default
// slot. Deciding this in one place matters: the list of slots offered and the
// save each one resolves to are built by different code, and a slot that
// appears in one but not the other cannot be chosen.
func SlotName(slot *string) string {
	if slot != nil && *slot != "" {
		return *slot
	}
	return settings.DefaultSaveSlot
}

// HasSlot reports whether the server already holds saves in a named slot.
//
// A slot it does not know about has to be created by uploading to it, which is
// a different operation from syncing an existing one.
func HasSlot(summary romm.SaveSummary, slot string) bool {
	for _, held := range summary.Slots {
		if SlotName(held.Slot) == slot {
			return true
		}
	}
	return false
}

// SlotNames lists the slots a summary holds, in the order the server gave
// them.
func SlotNames(summary romm.SaveSummary) []string {
	names := make([]string, 0, len(summary.Slots))
	for _, held := range summary.Slots {
		names = append(names, SlotName(held.Slot))
	}
	return names
}
