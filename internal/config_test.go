package internal

import "testing"

func TestSlotPreference_DefaultsToAutosave(t *testing.T) {
	c := Config{}
	if got := c.GetSlotPreference(1); got != "autosave" {
		t.Errorf("default slot = %q, want autosave", got)
	}
}

// Picking "autosave" must persist as an EXPLICIT preference (not be discarded), so it
// can override a sticky recorded slot. Otherwise a user can never switch a ROM back to
// autosave once another slot has been recorded (issue #250).
func TestSetSlotPreference_AutosavePersistsAsExplicit(t *testing.T) {
	c := Config{}
	c.SetSlotPreference(1, "quicksave")
	c.SetSlotPreference(1, "autosave") // user explicitly chooses autosave

	slot, ok := c.SlotPreferenceExplicit(1)
	if !ok || slot != "autosave" {
		t.Errorf("explicit autosave should persist: got (%q, %v), want (\"autosave\", true)", slot, ok)
	}
	if got := c.GetSlotPreference(1); got != "autosave" {
		t.Errorf("GetSlotPreference = %q, want autosave", got)
	}
}
