package saves

import (
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

func ptr(s string) *string { return &s }

func TestSlotName(t *testing.T) {
	tests := []struct {
		name string
		slot *string
		want string
	}{
		{"unset", nil, settings.DefaultSaveSlot},
		// The server sends this for a game whose only slot has no name. It
		// means the same as unset, and treating it as a slot called "" gives
		// the user a blank row they cannot pick.
		{"empty", ptr(""), settings.DefaultSaveSlot},
		{"named", ptr("quicksave"), "quicksave"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SlotName(tt.slot); got != tt.want {
				t.Errorf("SlotName = %q, want %q", got, tt.want)
			}
		})
	}
}

// The slots offered to the user and the save each one resolves to are worked
// out by different code. A save the first calls "autosave" and the second
// calls "" cannot be chosen at all, so both must read it the same way.
func TestSlotName_OfferedSlotsResolveToASave(t *testing.T) {
	for _, slot := range []*string{nil, ptr(""), ptr("quicksave")} {
		// A newer save in another slot, so a name that fails to match falls
		// through to it rather than landing back on the right one by luck.
		remote := []romm.Save{
			{ID: 9, Slot: slot, UpdatedAt: time.Now().Add(-time.Hour)},
			{ID: 10, Slot: ptr("other"), UpdatedAt: time.Now()},
		}

		offered := distinctSaveSlots(remote[:1])
		if len(offered) != 1 {
			t.Fatalf("slot %v offered %v, want exactly one name", slot, offered)
		}

		chosen := SelectSaveForSlot(remote, offered[0])
		if chosen == nil || chosen.ID != 9 {
			t.Errorf("slot %v is offered as %q but resolves to %v", slot, offered[0], chosen)
		}
	}
}

// Picking a slot must reach the save held in it rather than falling through to
// the newest, which is the fallback for a slot the server has nothing in.
func TestSelectSaveForSlot_MatchesTheEmptySlot(t *testing.T) {
	// The other save is newer, so falling through to the newest-overall
	// fallback would return it instead. That is what happens when the empty
	// slot is read as a slot literally called "".
	remote := []romm.Save{
		{ID: 1, Slot: ptr(""), UpdatedAt: time.Now().Add(-time.Hour)},
		{ID: 2, Slot: ptr("quicksave"), UpdatedAt: time.Now()},
	}

	got := SelectSaveForSlot(remote, settings.DefaultSaveSlot)

	if got == nil || got.ID != 1 {
		t.Errorf("got %v, want the save in the unnamed slot", got)
	}
}

func TestHasSlot(t *testing.T) {
	summary := romm.SaveSummary{Slots: []romm.SaveSlotInfo{
		{Slot: nil},
		{Slot: ptr("quicksave")},
	}}

	if !HasSlot(summary, settings.DefaultSaveSlot) {
		t.Error("an unnamed slot is the default slot and the server does hold it")
	}
	if !HasSlot(summary, "quicksave") {
		t.Error("quicksave is held")
	}
	// A slot the server does not have has to be created by uploading into it,
	// which is a different operation from syncing one that exists.
	if HasSlot(summary, "speedrun") {
		t.Error("speedrun is not held and must read as new")
	}
}

func TestSlotNames(t *testing.T) {
	summary := romm.SaveSummary{Slots: []romm.SaveSlotInfo{
		{Slot: ptr("quicksave")},
		{Slot: nil},
		{Slot: ptr("")},
	}}

	got := SlotNames(summary)

	want := []string{"quicksave", settings.DefaultSaveSlot, settings.DefaultSaveSlot}
	if len(got) != len(want) {
		t.Fatalf("SlotNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SlotNames = %v, want %v", got, want)
			break
		}
	}
}
