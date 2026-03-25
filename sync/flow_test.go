package sync

import (
	"grout/romm"
	"testing"
	"time"
)

func ptrStr(s string) *string { return &s }
func ptrInt(i int) *int       { return &i }
func ptrTime(t time.Time) *time.Time { return &t }

// --- buildRemoteSaveStub tests ---

func TestBuildRemoteSaveStub_NilWhenNoIDOrTimestamp(t *testing.T) {
	op := romm.SyncOperationSchema{
		Action:   "upload",
		RomID:    1,
		FileName: "test.sav",
	}
	result := buildRemoteSaveStub(op)
	if result != nil {
		t.Error("expected nil when no save_id or server_updated_at")
	}
}

func TestBuildRemoteSaveStub_WithSaveID(t *testing.T) {
	op := romm.SyncOperationSchema{
		Action:   "download",
		RomID:    1,
		SaveID:   ptrInt(42),
		FileName: "test.sav",
		Slot:     ptrStr("quicksave"),
		Emulator: "mgba",
	}
	result := buildRemoteSaveStub(op)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
	if result.RomID != 1 {
		t.Errorf("expected RomID 1, got %d", result.RomID)
	}
	if result.FileName != "test.sav" {
		t.Errorf("expected FileName test.sav, got %s", result.FileName)
	}
	if result.Slot == nil || *result.Slot != "quicksave" {
		t.Errorf("expected Slot quicksave, got %v", result.Slot)
	}
	if result.Emulator != "mgba" {
		t.Errorf("expected Emulator mgba, got %s", result.Emulator)
	}
	if result.FileExtension != "sav" {
		t.Errorf("expected FileExtension sav, got %s", result.FileExtension)
	}
}

func TestBuildRemoteSaveStub_WithServerUpdatedAt(t *testing.T) {
	ts := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	op := romm.SyncOperationSchema{
		Action:          "download",
		RomID:           1,
		FileName:        "game.srm",
		ServerUpdatedAt: ptrTime(ts),
	}
	result := buildRemoteSaveStub(op)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.UpdatedAt.Equal(ts) {
		t.Errorf("expected UpdatedAt %v, got %v", ts, result.UpdatedAt)
	}
}

// --- SelectSaveForSlot tests ---

func TestSelectSaveForSlot_EmptySaves(t *testing.T) {
	result := SelectSaveForSlot(nil, "default")
	if result != nil {
		t.Errorf("expected nil for empty saves, got %+v", result)
	}
}

func TestSelectSaveForSlot_ReturnsDefaultSlot(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("default"), UpdatedAt: time.Now()},
		{ID: 99, Slot: ptrStr("slot2"), UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "default")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

func TestSelectSaveForSlot_ReturnsPreferredSlot(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("default"), UpdatedAt: time.Now()},
		{ID: 99, Slot: ptrStr("quicksave"), UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "quicksave")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 99 {
		t.Errorf("expected ID 99, got %d", result.ID)
	}
}

func TestSelectSaveForSlot_PicksLatestInSlot(t *testing.T) {
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(1 * time.Hour)

	saves := []romm.Save{
		{ID: 10, Slot: ptrStr("default"), UpdatedAt: older},
		{ID: 20, Slot: ptrStr("default"), UpdatedAt: newer},
	}

	result := SelectSaveForSlot(saves, "default")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 20 {
		t.Errorf("expected ID 20 (latest), got %d", result.ID)
	}
}

func TestSelectSaveForSlot_FallsBackToLatest(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("default"), UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "nonexistent")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42 (fallback to latest), got %d", result.ID)
	}
}

func TestSelectSaveForSlot_NilSlotTreatedAsDefault(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: nil, UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "default")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

// --- SyncAction tests ---

func TestSyncAction_String(t *testing.T) {
	tests := []struct {
		action SyncAction
		want   string
	}{
		{ActionUpload, "upload"},
		{ActionDownload, "download"},
		{ActionConflict, "conflict"},
		{ActionSkip, "skip"},
	}
	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("SyncAction(%d).String() = %q, want %q", tt.action, got, tt.want)
		}
	}
}

// --- SyncItem.Resolve tests ---

func TestSyncItem_Resolve(t *testing.T) {
	item := SyncItem{Action: ActionConflict}
	item.Resolve(ActionUpload)
	if item.Action != ActionUpload {
		t.Errorf("expected ActionUpload after Resolve, got %s", item.Action)
	}
}
