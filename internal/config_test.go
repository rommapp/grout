package internal

import "testing"

func TestSlotPreference_DefaultsToAutosave(t *testing.T) {
	c := Config{}
	if got := c.GetSlotPreference(1); got != "autosave" {
		t.Errorf("default slot = %q, want autosave", got)
	}
}

func TestSetSlotPreference_AutosaveClears(t *testing.T) {
	c := Config{}
	c.SetSlotPreference(1, "quicksave")
	if got := c.GetSlotPreference(1); got != "quicksave" {
		t.Fatalf("got %q after set", got)
	}
	c.SetSlotPreference(1, "autosave") // setting back to default clears the entry
	if _, ok := c.SlotPreferences["1"]; ok {
		t.Errorf("expected autosave to clear the stored preference")
	}
	if got := c.GetSlotPreference(1); got != "autosave" {
		t.Errorf("after clear, default = %q, want autosave", got)
	}
}
