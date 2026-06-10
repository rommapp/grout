package sync

import (
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ptrStr(s string) *string { return &s }

// --- discovery fallback tests ---

func TestBuildDiscoveryItems_NeverSyncedPulled(t *testing.T) {
	uncovered := map[int]cfw.LocalRomFile{
		303: {RomID: 303, RomName: "Pokemon", FSSlug: "gba", FileName: "Pokemon.gba"},
	}
	savesByRom := map[int][]romm.Save{
		303: {
			{ID: 228, RomID: 303, FileName: "Pokemon [2026].srm", FileExtension: "srm",
				Slot: ptrStr("default"), UpdatedAt: time.Now()},
		},
	}

	items := buildDiscoveryItems(uncovered, savesByRom, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 discovery item, got %d", len(items))
	}
	if items[0].Action != ActionDownload {
		t.Errorf("expected ActionDownload, got %s", items[0].Action)
	}
	if items[0].RemoteSave == nil || items[0].RemoteSave.ID != 228 {
		t.Errorf("expected RemoteSave 228, got %+v", items[0].RemoteSave)
	}
	if items[0].LocalSave.FSSlug != "gba" || items[0].LocalSave.RomFileName != "Pokemon.gba" {
		t.Errorf("LocalSave not resolved: %+v", items[0].LocalSave)
	}
}

func TestBuildDiscoveryItems_AlreadySyncedStillPulled(t *testing.T) {
	// Discovery only runs when there is no local file, so a save this device synced
	// before (then lost locally, e.g. after a reflash) MUST still be pulled.
	uncovered := map[int]cfw.LocalRomFile{303: {RomID: 303, FSSlug: "gba", FileName: "P.gba"}}
	savesByRom := map[int][]romm.Save{
		303: {{ID: 228, RomID: 303, FileName: "P.srm", FileExtension: "srm",
			Slot: ptrStr("default"), UpdatedAt: time.Now(),
			DeviceSyncs: []romm.DeviceSaveSync{{DeviceID: "dev-1"}}}},
	}

	items := buildDiscoveryItems(uncovered, savesByRom, nil)

	if len(items) != 1 {
		t.Fatalf("expected already-synced save to still be pulled, got %d items", len(items))
	}
	if items[0].RemoteSave == nil || items[0].RemoteSave.ID != 228 {
		t.Errorf("expected RemoteSave 228, got %+v", items[0].RemoteSave)
	}
}

func TestBuildDiscoveryItems_NullSlotIncluded(t *testing.T) {
	uncovered := map[int]cfw.LocalRomFile{303: {RomID: 303, FSSlug: "gba", FileName: "P.gba"}}
	savesByRom := map[int][]romm.Save{
		303: {{ID: 223, RomID: 303, FileName: "P.srm", FileExtension: "srm",
			Slot: nil, UpdatedAt: time.Now()}},
	}

	items := buildDiscoveryItems(uncovered, savesByRom, nil)

	if len(items) != 1 {
		t.Fatalf("expected null-slot save to be included, got %d items", len(items))
	}
	if items[0].RemoteSave == nil || items[0].RemoteSave.ID != 223 {
		t.Errorf("expected RemoteSave 223, got %+v", items[0].RemoteSave)
	}
}

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

	// rom 3 has no local save, so it must be present as an installed ROM for its
	// download to be accepted (downloads are gated on local ROM presence).
	resolved := map[int]cfw.LocalRomFile{3: {RomID: 3, RomName: "Metroid", FSSlug: "snes", FileName: "Metroid.gba"}}
	items := mapOperationsToItems(ops, local, resolved, nil, nil)

	byAction := map[SyncAction]SyncItem{}
	for _, it := range items {
		byAction[it.Action] = it
	}
	// no_op dropped; upload, conflict, download mapped
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if up, ok := byAction[ActionUpload]; !ok || up.LocalSave.RomID != 1 {
		t.Errorf("upload item = %+v", up)
	}
	if cf, ok := byAction[ActionConflict]; !ok || cf.RemoteSave == nil || cf.RemoteSave.ID != 20 {
		t.Errorf("conflict item = %+v", cf)
	}
	dl, ok := byAction[ActionDownload]
	if !ok || dl.LocalSave.RomID != 3 || dl.RemoteSave == nil || dl.RemoteSave.ID != 30 {
		t.Errorf("download item = %+v", dl)
	}
}

func TestMapOperationsToItems_DropsDownloadWithoutSaveIdentity(t *testing.T) {
	// rom 5 is installed (so it passes the local-presence gate) but its download op
	// carries no save identity, so it must still be dropped.
	resolved := map[int]cfw.LocalRomFile{5: {RomID: 5, FSSlug: "gba", FileName: "x.gba"}}
	ops := []romm.SyncOperationSchema{
		{Action: "download", RomID: 5, FileName: "x.srm"}, // no SaveID, no ServerUpdatedAt
	}
	items := mapOperationsToItems(ops, nil, resolved, nil, nil)
	if len(items) != 0 {
		t.Errorf("expected malformed download op to be dropped, got %d items", len(items))
	}
}

func TestMapOperationsToItems_DownloadGatedToInstalledAndDeduped(t *testing.T) {
	// Only rom 303 is installed locally; rom 10 is not.
	resolved := map[int]cfw.LocalRomFile{
		303: {RomID: 303, RomName: "Pokemon", FSSlug: "gba", FileName: "Pokemon.gba"},
	}
	now := time.Now()
	ops := []romm.SyncOperationSchema{
		// rom 303: two slots -> exactly one download item, preferring "autosave"
		{Action: "download", RomID: 303, SaveID: ptrInt(235), FileName: "P [a].srm", Slot: ptrStr("autosave"), ServerUpdatedAt: ptrTime(now)},
		{Action: "download", RomID: 303, SaveID: ptrInt(228), FileName: "P [d].srm", Slot: ptrStr("default"), ServerUpdatedAt: ptrTime(now)},
		// rom 10 not installed -> dropped entirely
		{Action: "download", RomID: 10, SaveID: ptrInt(234), FileName: "AW [a].srm", Slot: ptrStr("autosave"), ServerUpdatedAt: ptrTime(now)},
	}

	items := mapOperationsToItems(ops, nil, resolved, nil, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item (installed + deduped), got %d", len(items))
	}
	it := items[0]
	if it.Action != ActionDownload || it.LocalSave.RomID != 303 {
		t.Fatalf("unexpected item: %+v", it)
	}
	if it.RemoteSave == nil || it.RemoteSave.ID != 235 {
		t.Errorf("expected autosave save 235, got %+v", it.RemoteSave)
	}
	if it.LocalSave.FSSlug != "gba" || it.LocalSave.RomFileName != "Pokemon.gba" {
		t.Errorf("local save not resolved from installed ROM: %+v", it.LocalSave)
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

	states := buildClientSaveStates(local, nil, nil)
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

func TestBuildClientSaveStates_SlotPrecedence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Mario.srm")
	if err := os.WriteFile(p, []byte("savedata"), 0644); err != nil {
		t.Fatal(err)
	}
	local := []LocalSave{{RomID: 7, FileName: "Mario.srm", FilePath: p, EmulatorDir: "mgba"}}

	// 1. Recorded slot wins over the autosave default when there's no explicit preference.
	recorded := map[saveKey]string{{romID: 7, fileName: "Mario.srm"}: "default"}
	states := buildClientSaveStates(local, nil, recorded)
	if len(states) != 1 || states[0].Slot != "default" {
		t.Fatalf("recorded slot should win: got %+v", states)
	}

	// 2. Explicit user preference wins over the recorded slot.
	cfg := &internal.Config{SlotPreferences: map[string]string{"7": "quicksave"}}
	states = buildClientSaveStates(local, cfg, recorded)
	if len(states) != 1 || states[0].Slot != "quicksave" {
		t.Fatalf("explicit preference should win over record: got %+v", states)
	}

	// 3. No preference and no record → autosave default.
	states = buildClientSaveStates(local, nil, nil)
	if len(states) != 1 || states[0].Slot != "autosave" {
		t.Fatalf("default should be autosave: got %+v", states)
	}
}
