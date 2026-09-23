package saves

import (
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

func TestWatchUploads(t *testing.T) {
	watch := WatchUploads([]SyncItem{
		{Action: ActionUpload},
		{Action: ActionDownload},
		{Action: ActionConflict},
	})

	if len(watch) != 3 {
		t.Fatalf("watch = %v, want one entry per item", watch)
	}
	if !watch[0] || watch[1] || watch[2] {
		t.Errorf("watch = %v, want only the upload marked", watch)
	}
}

// An upload the server rejects with a 409 becomes a conflict with the server's
// save attached, and that is worth going back to ask about. Two other kinds
// look identical afterwards and must be left alone, or the sync screen and the
// conflict screen hand each other the same work forever.
func TestConflictWatch_Surfaced(t *testing.T) {
	remote := &romm.Save{ID: 75}

	items := []SyncItem{
		{Action: ActionDownload},                     // a download, never an upload
		{Action: ActionConflict, RemoteSave: remote}, // the 409 worth asking about
		{Action: ActionUpload},                       // uploaded fine, still an upload
		{Action: ActionConflict, RemoteSave: remote}, // already answered before this run
		{Action: ActionConflict, RemoteSave: nil},    // nothing to compare against
	}
	watch := ConflictWatch{false, true, true, false, true}

	got := watch.Surfaced(items)

	if len(got) != 1 || got[0] != 1 {
		t.Errorf("Surfaced = %v, want only the resolvable 409 at index 1", got)
	}
}

func TestConflictWatch_SurfacedNoneWhenAllResolved(t *testing.T) {
	items := []SyncItem{
		{Action: ActionUpload, Success: true},
		{Action: ActionDownload, Success: true},
	}

	if got := (ConflictWatch{true, false}).Surfaced(items); len(got) != 0 {
		t.Errorf("Surfaced = %v, want nothing to go back for", got)
	}
}

// The loop between the two screens has to end. Feeding a run's output back
// through the watch must eventually surface nothing, whatever the user chose.
func TestConflictWatch_SurfacedTerminates(t *testing.T) {
	remote := &romm.Save{ID: 75}
	items := []SyncItem{{Action: ActionUpload}}

	for round := 0; round < 5; round++ {
		watch := WatchUploads(items)

		// Worst case: every upload comes back as a resolvable conflict.
		for i := range items {
			if items[i].Action == ActionUpload {
				items[i].Action = ActionConflict
				items[i].RemoteSave = remote
			}
		}

		surfaced := watch.Surfaced(items)
		if round == 0 {
			if len(surfaced) != 1 {
				t.Fatalf("round 0 surfaced %v, want the conflict", surfaced)
			}
			continue
		}
		if len(surfaced) != 0 {
			t.Fatalf("round %d surfaced %v again, so the screens would loop", round, surfaced)
		}
	}
}

func TestPendingConflicts(t *testing.T) {
	items := []SyncItem{
		{Action: ActionUpload},
		{Action: ActionConflict},
		{Action: ActionDownload},
		{Action: ActionConflict},
	}

	got := PendingConflicts(items)

	// The conflict screen addresses them by position among the conflicts, so
	// the mapping has to be dense and in order.
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("PendingConflicts = %v, want {0:1, 1:3}", got)
	}
}

func TestNeedsSlotChoice(t *testing.T) {
	items := []SyncItem{
		{AvailableSlots: nil},
		{AvailableSlots: []string{"default"}},
		{AvailableSlots: []string{"default", "autosave"}},
	}

	got := NeedsSlotChoice(items)

	// One slot is not a choice, so only the last needs asking about.
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("NeedsSlotChoice = %v, want just index 2", got)
	}
}

func slot(name string) *string { return &name }

func TestChooseSlot(t *testing.T) {
	config := &settings.Config{}
	item := SyncItem{
		LocalSave: LocalSave{RomID: 7},
		AllRemoteSaves: []romm.Save{
			{ID: 1, Slot: slot("default")},
			{ID: 2, Slot: slot("autosave")},
		},
		RemoteSave: &romm.Save{ID: 1},
	}

	ChooseSlot(config, &item, "autosave")

	if got := config.GetSlotPreference(7); got != "autosave" {
		t.Errorf("preference = %q, want it remembered for next time", got)
	}
	if item.RemoteSave == nil || item.RemoteSave.ID != 2 {
		t.Errorf("RemoteSave = %v, want the save held in the chosen slot", item.RemoteSave)
	}
}

// A slot the server has nothing in falls back to the newest save rather than
// leaving the download with no source. The preference is still recorded, so a
// later sync that does find that slot uses it.
func TestChooseSlot_UnknownSlotFallsBackToNewest(t *testing.T) {
	config := &settings.Config{}
	older := time.Now().Add(-time.Hour)
	item := SyncItem{
		LocalSave: LocalSave{RomID: 7},
		AllRemoteSaves: []romm.Save{
			{ID: 1, Slot: slot("default"), UpdatedAt: older},
			{ID: 2, Slot: slot("default"), UpdatedAt: time.Now()},
		},
		RemoteSave: &romm.Save{ID: 1},
	}

	ChooseSlot(config, &item, "nonexistent")

	if got := config.GetSlotPreference(7); got != "nonexistent" {
		t.Errorf("preference = %q, want it recorded even with nothing in that slot yet", got)
	}
	if item.RemoteSave == nil || item.RemoteSave.ID != 2 {
		t.Errorf("RemoteSave = %v, want the newest save as the fallback", item.RemoteSave)
	}
}

// Backing out of the slot picker skips those downloads, and must not take
// anything else with them.
func TestWithout(t *testing.T) {
	items := []SyncItem{
		{LocalSave: LocalSave{RomID: 1}, Action: ActionUpload},
		{LocalSave: LocalSave{RomID: 2}, Action: ActionDownload},
		{LocalSave: LocalSave{RomID: 3}, Action: ActionDownload},
	}

	kept := Without(items, map[int]bool{1: true})

	if len(kept) != 2 || kept[0].LocalSave.RomID != 1 || kept[1].LocalSave.RomID != 3 {
		t.Errorf("kept %v, want the untouched items only", kept)
	}
	if got := Without(items, nil); len(got) != 3 {
		t.Errorf("dropping nothing returned %d items, want all 3", len(got))
	}
}

func TestActionable(t *testing.T) {
	tests := []struct {
		name  string
		items []SyncItem
		want  bool
	}{
		{"nothing", nil, false},
		{"only skips", []SyncItem{{Action: ActionSkip}}, false},
		{"only conflicts", []SyncItem{{Action: ActionConflict}}, false},
		{"an upload", []SyncItem{{Action: ActionSkip}, {Action: ActionUpload}}, true},
		{"a download", []SyncItem{{Action: ActionDownload}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Actionable(tt.items); got != tt.want {
				t.Errorf("Actionable = %v, want %v", got, tt.want)
			}
		})
	}
}
