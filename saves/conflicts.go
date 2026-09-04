package saves

import "grout/settings"

// ConflictWatch remembers which items were uploads before a sync ran.
//
// Executing rewrites an item's action in place, so telling a conflict that
// appeared during the run from one the user was already asked about needs a
// snapshot taken beforehand.
type ConflictWatch []bool

// WatchUploads snapshots the items about to be executed.
func WatchUploads(items []SyncItem) ConflictWatch {
	watch := make(ConflictWatch, len(items))
	for i := range items {
		watch[i] = items[i].Action == ActionUpload
	}
	return watch
}

// Surfaced returns the conflicts that appeared during execution, mapping each
// one's position among the conflicts to its position in items.
//
// The server rejects an upload with a 409 when its own save has moved on,
// which turns that item into a conflict with the server's save attached. Those
// are worth asking the user about.
//
// Two kinds are deliberately left out, because sending either back to the
// conflict screen would put the two screens in a loop that never ends: one the
// user has already answered, and one with no server save to compare against,
// which there is no way to resolve.
func (w ConflictWatch) Surfaced(items []SyncItem) map[int]int {
	surfaced := map[int]int{}

	position := 0
	for i := range items {
		if i >= len(w) || !w[i] {
			continue
		}
		if items[i].Action != ActionConflict || items[i].RemoteSave == nil {
			continue
		}
		surfaced[position] = i
		position++
	}

	return surfaced
}

// PendingConflicts maps each conflict's position among the conflicts to its
// position in items, which is how the conflict screen addresses them.
func PendingConflicts(items []SyncItem) map[int]int {
	conflicts := map[int]int{}

	position := 0
	for i := range items {
		if items[i].Action == ActionConflict {
			conflicts[position] = i
			position++
		}
	}

	return conflicts
}

// NeedsSlotChoice returns the indices of items the server holds several saves
// for, where the user has not yet said which slot to take.
func NeedsSlotChoice(items []SyncItem) []int {
	var indices []int
	for i := range items {
		if len(items[i].AvailableSlots) > 1 {
			indices = append(indices, i)
		}
	}
	return indices
}

// ChooseSlot records the user's preference and points the item at the server
// save held in that slot.
func ChooseSlot(config *settings.Config, item *SyncItem, slot string) {
	config.SetSlotPreference(item.LocalSave.RomID, slot)

	if chosen := SelectSaveForSlot(item.AllRemoteSaves, slot); chosen != nil {
		item.RemoteSave = chosen
	}
}

// Without returns items with the indexed entries removed.
func Without(items []SyncItem, drop map[int]bool) []SyncItem {
	if len(drop) == 0 {
		return items
	}

	kept := make([]SyncItem, 0, len(items))
	for i := range items {
		if !drop[i] {
			kept = append(kept, items[i])
		}
	}
	return kept
}

// UploadsForRom builds the items to push every local save of one game into a
// named slot.
func UploadsForRom(config *settings.Config, romID int, slot string) []SyncItem {
	var items []SyncItem
	for _, local := range ScanSaves(config) {
		if local.RomID == romID {
			items = append(items, SyncItem{LocalSave: local, TargetSlot: slot, Action: ActionUpload})
		}
	}
	return items
}

// Actionable reports whether anything here will actually touch the network.
// A run of nothing but skips and conflicts needs no progress bar.
func Actionable(items []SyncItem) bool {
	for i := range items {
		if items[i].Action != ActionSkip && items[i].Action != ActionConflict {
			return true
		}
	}
	return false
}
