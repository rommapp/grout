package ui

import (
	"testing"

	"grout/saves"
)

// Cancelling the multi-slot slot picker must skip the unconfirmed downloads while
// preserving every other sync item (uploads, single-slot downloads).
func TestDropSyncItems_RemovesOnlyChosenIndices(t *testing.T) {
	items := []saves.SyncItem{
		{LocalSave: saves.LocalSave{RomID: 1}, Action: saves.ActionUpload},
		{LocalSave: saves.LocalSave{RomID: 2}, Action: saves.ActionDownload}, // multi-slot, cancelled
		{LocalSave: saves.LocalSave{RomID: 3}, Action: saves.ActionDownload}, // single-slot, keep
	}

	kept := dropSyncItems(items, map[int]bool{1: true})

	if len(kept) != 2 {
		t.Fatalf("expected 2 items kept, got %d", len(kept))
	}
	if kept[0].LocalSave.RomID != 1 || kept[1].LocalSave.RomID != 3 {
		t.Errorf("kept wrong items: %d, %d", kept[0].LocalSave.RomID, kept[1].LocalSave.RomID)
	}
}

func TestDropSyncItems_EmptyDropReturnsAll(t *testing.T) {
	items := []saves.SyncItem{{LocalSave: saves.LocalSave{RomID: 1}}}
	if got := dropSyncItems(items, nil); len(got) != 1 {
		t.Errorf("expected all items returned, got %d", len(got))
	}
}
