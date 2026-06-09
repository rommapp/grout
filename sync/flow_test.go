package sync

import (
	"grout/romm"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ptrStr(s string) *string { return &s }

func ptrInt(i int) *int              { return &i }
func ptrTime(t time.Time) *time.Time { return &t }

// --- SelectSaveForSlot tests ---

func TestSelectSaveForSlot_EmptySaves(t *testing.T) {
	result := SelectSaveForSlot(nil, "autosave")
	if result != nil {
		t.Errorf("expected nil for empty saves, got %+v", result)
	}
}

func TestSelectSaveForSlot_ReturnsPreferredSlot(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("autosave"), UpdatedAt: time.Now()},
		{ID: 99, Slot: ptrStr("slot2"), UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "autosave")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

func TestSelectSaveForSlot_ReturnsNamedSlot(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("autosave"), UpdatedAt: time.Now()},
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
		{ID: 10, Slot: ptrStr("autosave"), UpdatedAt: older},
		{ID: 20, Slot: ptrStr("autosave"), UpdatedAt: newer},
	}

	result := SelectSaveForSlot(saves, "autosave")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 20 {
		t.Errorf("expected ID 20 (latest), got %d", result.ID)
	}
}

func TestSelectSaveForSlot_FallsBackToLatest(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("autosave"), UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "nonexistent")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42 (fallback to latest), got %d", result.ID)
	}
}

func TestSelectSaveForSlot_NilSlotTreatedAsAutosave(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: nil, UpdatedAt: time.Now()},
	}

	result := SelectSaveForSlot(saves, "autosave")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

// --- mapOperationsToItems tests ---

func TestMapOperationsToItems_DropsNoOpAndMapsActions(t *testing.T) {
	local := []LocalSave{
		{RomID: 1, RomName: "Mario", FileName: "Mario.srm", FilePath: "/x/Mario.srm", FSSlug: "snes"},
		{RomID: 2, RomName: "Zelda", FileName: "Zelda.srm", FilePath: "/x/Zelda.srm", FSSlug: "snes"},
	}
	ops := []romm.SyncOperationSchema{
		{Action: "upload", RomID: 1, FileName: "Mario.srm"},
		{Action: "conflict", RomID: 2, FileName: "Zelda.srm", SaveID: ptrInt(20), ServerUpdatedAt: ptrTime(time.Now())},
		{Action: "no_op", RomID: 99, FileName: "skip.srm"},
		{Action: "download", RomID: 3, FileName: "Metroid.srm", SaveID: ptrInt(30), ServerUpdatedAt: ptrTime(time.Now())},
	}

	items := mapOperationsToItems(ops, local, nil, nil, nil)

	var got []string
	for _, it := range items {
		got = append(got, it.Action.String())
	}
	// no_op dropped; order preserved
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d (%v)", len(items), got)
	}
	if items[0].Action != ActionUpload || items[0].LocalSave.RomID != 1 {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[1].Action != ActionConflict || items[1].RemoteSave == nil || items[1].RemoteSave.ID != 20 {
		t.Errorf("item1 = %+v", items[1])
	}
	if items[2].Action != ActionDownload || items[2].LocalSave.RomID != 3 {
		t.Errorf("item2 = %+v", items[2])
	}
	if items[2].RemoteSave == nil || items[2].RemoteSave.ID != 30 {
		t.Errorf("item2 RemoteSave = %+v", items[2].RemoteSave)
	}
}

func TestMapOperationsToItems_DropsDownloadWithoutSaveIdentity(t *testing.T) {
	ops := []romm.SyncOperationSchema{
		{Action: "download", RomID: 5, FileName: "x.srm"}, // no SaveID, no ServerUpdatedAt
	}
	items := mapOperationsToItems(ops, nil, nil, nil, nil)
	if len(items) != 0 {
		t.Errorf("expected malformed download op to be dropped, got %d items", len(items))
	}
}

// --- buildClientSaveStates tests ---

func TestBuildClientSaveStates_FileSlotEmulatorHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Mario.srm")
	if err := os.WriteFile(p, []byte("savedata"), 0644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	local := []LocalSave{{
		RomID:       7,
		FileName:    "Mario.srm",
		FilePath:    p,
		EmulatorDir: "mgba",
	}}

	states := buildClientSaveStates(local, nil)
	if len(states) != 1 {
		t.Fatalf("got %d states", len(states))
	}
	s := states[0]
	if s.RomID != 7 || s.FileName != "Mario.srm" {
		t.Errorf("rom/file = %d/%s", s.RomID, s.FileName)
	}
	if s.Slot != "autosave" {
		t.Errorf("slot = %q, want autosave", s.Slot)
	}
	if s.Emulator != "mgba" {
		t.Errorf("emulator = %q", s.Emulator)
	}
	if s.FileSizeBytes != int64(len("savedata")) {
		t.Errorf("size = %d", s.FileSizeBytes)
	}
	if !s.UpdatedAt.Equal(mtime) {
		t.Errorf("updated_at = %v, want %v", s.UpdatedAt, mtime)
	}
	if s.ContentHash == "" {
		t.Error("expected a content hash for a file save")
	}
}
