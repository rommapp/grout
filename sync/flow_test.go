package sync

import (
	"grout/internal"
	"grout/romm"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeTempSave(t *testing.T, mtime time.Time) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sav")
	if err := os.WriteFile(path, []byte("save"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

func ptrStr(s string) *string { return &s }

// --- determineAction tests ---

func TestDetermineAction_NoRemoteSave(t *testing.T) {
	now := time.Now()
	path := makeTempSave(t, now)
	ls := &LocalSave{RomID: 1, FilePath: path}

	action := determineAction(nil, ls, "device-1")

	if action != ActionUpload {
		t.Errorf("expected ActionUpload, got %s", action)
	}
}

func TestDetermineAction_LocalFileUnreadable(t *testing.T) {
	ls := &LocalSave{RomID: 1, FilePath: "/nonexistent/path/save.sav"}
	remote := &romm.Save{ID: 10, UpdatedAt: time.Now()}

	action := determineAction(remote, ls, "device-1")

	if action != ActionDownload {
		t.Errorf("expected ActionDownload, got %s", action)
	}
}

func TestDetermineAction_DeviceCurrent_BothChanged(t *testing.T) {
	lastSync := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	localMtime := lastSync.Add(1 * time.Hour)
	remoteUpdated := lastSync.Add(2 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:        10,
		UpdatedAt: remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{
			{DeviceID: "device-1", IsCurrent: true, LastSyncedAt: lastSync},
		},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionConflict {
		t.Errorf("expected ActionConflict, got %s", action)
	}
}

func TestDetermineAction_DeviceCurrent_LocalNewer(t *testing.T) {
	remoteUpdated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lastSync := remoteUpdated // remote hasn't changed since last sync
	localMtime := remoteUpdated.Add(1 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:        10,
		UpdatedAt: remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{
			{DeviceID: "device-1", IsCurrent: true, LastSyncedAt: lastSync},
		},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionUpload {
		t.Errorf("expected ActionUpload, got %s", action)
	}
}

func TestDetermineAction_DeviceCurrent_RemoteNewerOrEqual(t *testing.T) {
	lastSync := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	remoteUpdated := lastSync.Add(1 * time.Hour)
	localMtime := lastSync.Add(-1 * time.Hour) // local older than both

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:        10,
		UpdatedAt: remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{
			{DeviceID: "device-1", IsCurrent: true, LastSyncedAt: lastSync},
		},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionSkip {
		t.Errorf("expected ActionSkip, got %s", action)
	}
}

func TestDetermineAction_DeviceTrackedNotCurrent_BothChanged(t *testing.T) {
	lastSync := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	localMtime := lastSync.Add(1 * time.Hour)
	remoteUpdated := lastSync.Add(2 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:        10,
		UpdatedAt: remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{
			{DeviceID: "device-1", IsCurrent: false, LastSyncedAt: lastSync},
		},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionConflict {
		t.Errorf("expected ActionConflict, got %s", action)
	}
}

func TestDetermineAction_DeviceTrackedNotCurrent_OnlyRemoteChanged(t *testing.T) {
	lastSync := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	localMtime := lastSync.Add(-1 * time.Hour)
	remoteUpdated := lastSync.Add(2 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:        10,
		UpdatedAt: remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{
			{DeviceID: "device-1", IsCurrent: false, LastSyncedAt: lastSync},
		},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionDownload {
		t.Errorf("expected ActionDownload, got %s", action)
	}
}

func TestDetermineAction_DeviceNotTracked_LocalNewer(t *testing.T) {
	remoteUpdated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	localMtime := remoteUpdated.Add(1 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:          10,
		UpdatedAt:   remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{}, // empty - device not tracked
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionUpload {
		t.Errorf("expected ActionUpload, got %s", action)
	}
}

func TestDetermineAction_DeviceNotTracked_SameMtime(t *testing.T) {
	mtime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	path := makeTempSave(t, mtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:          10,
		UpdatedAt:   mtime,
		DeviceSyncs: []romm.DeviceSaveSync{},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionSkip {
		t.Errorf("expected ActionSkip, got %s", action)
	}
}

func TestDetermineAction_DeviceNotTracked_LocalOlder(t *testing.T) {
	remoteUpdated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	localMtime := remoteUpdated.Add(-1 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:          10,
		UpdatedAt:   remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionDownload {
		t.Errorf("expected ActionDownload, got %s", action)
	}
}

func TestDetermineAction_OtherDeviceCurrent(t *testing.T) {
	remoteUpdated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	localMtime := remoteUpdated.Add(-1 * time.Hour)

	path := makeTempSave(t, localMtime)
	ls := &LocalSave{RomID: 1, FilePath: path}
	remote := &romm.Save{
		ID:        10,
		UpdatedAt: remoteUpdated,
		DeviceSyncs: []romm.DeviceSaveSync{
			{DeviceID: "other-device", IsCurrent: true, LastSyncedAt: remoteUpdated},
		},
	}

	action := determineAction(remote, ls, "device-1")

	if action != ActionDownload {
		t.Errorf("expected ActionDownload, got %s", action)
	}
}

// --- DetermineActions tests ---

func TestDetermineActions_SkipsSavesWithoutRemote(t *testing.T) {
	now := time.Now()
	path := makeTempSave(t, now)
	localSaves := []LocalSave{
		{RomID: 1, FilePath: path},
		{RomID: 2, FilePath: path},
	}
	remoteSaves := map[int][]romm.Save{} // no remote saves

	items := DetermineActions(localSaves, remoteSaves, "device-1", nil)

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestDetermineActions_EmptyInputs(t *testing.T) {
	items := DetermineActions(nil, nil, "device-1", nil)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestDetermineActions_ReturnsCorrectActions(t *testing.T) {
	remoteUpdated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// ROM 1: local newer, no device tracking → upload
	uploadPath := makeTempSave(t, remoteUpdated.Add(1*time.Hour))
	// ROM 2: local older, no device tracking → download
	downloadPath := makeTempSave(t, remoteUpdated.Add(-1*time.Hour))

	localSaves := []LocalSave{
		{RomID: 1, RomName: "Mario", FilePath: uploadPath},
		{RomID: 2, RomName: "Zelda", FilePath: downloadPath},
	}

	remoteSaves := map[int][]romm.Save{
		1: {{ID: 10, RomID: 1, UpdatedAt: remoteUpdated, Slot: ptrStr("default")}},
		2: {{ID: 20, RomID: 2, UpdatedAt: remoteUpdated, Slot: ptrStr("default")}},
	}

	items := DetermineActions(localSaves, remoteSaves, "device-1", nil)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	actionsByRom := map[int]SyncAction{}
	for _, item := range items {
		actionsByRom[item.LocalSave.RomID] = item.Action
	}

	if actionsByRom[1] != ActionUpload {
		t.Errorf("ROM 1: expected ActionUpload, got %s", actionsByRom[1])
	}
	if actionsByRom[2] != ActionDownload {
		t.Errorf("ROM 2: expected ActionDownload, got %s", actionsByRom[2])
	}
}

func TestDetermineActions_PopulatesRemoteSave(t *testing.T) {
	remoteUpdated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	path := makeTempSave(t, remoteUpdated.Add(1*time.Hour))

	localSaves := []LocalSave{
		{RomID: 1, RomName: "Mario", FilePath: path},
	}
	remoteSaves := map[int][]romm.Save{
		1: {{ID: 10, RomID: 1, UpdatedAt: remoteUpdated, Slot: ptrStr("default")}},
	}

	items := DetermineActions(localSaves, remoteSaves, "device-1", nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].RemoteSave == nil {
		t.Fatal("expected RemoteSave to be set")
	}
	if items[0].RemoteSave.ID != 10 {
		t.Errorf("expected RemoteSave.ID 10, got %d", items[0].RemoteSave.ID)
	}
}

func TestDetermineActions_NoRemoteSavesForRom(t *testing.T) {
	path := makeTempSave(t, time.Now())
	localSaves := []LocalSave{
		{RomID: 1, FilePath: path},
	}
	remoteSaves := map[int][]romm.Save{
		1: {}, // empty saves list
	}

	items := DetermineActions(localSaves, remoteSaves, "device-1", nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	// No remote save (empty list) → upload
	if items[0].Action != ActionUpload {
		t.Errorf("expected ActionUpload for empty saves, got %s", items[0].Action)
	}
}

// --- selectSaveForSync tests ---

func TestSelectSaveForSync_EmptySaves(t *testing.T) {
	result := selectSaveForSync(nil, "default")
	if result != nil {
		t.Errorf("expected nil for empty saves, got %+v", result)
	}
}

func TestSelectSaveForSync_ReturnsDefaultSlot(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("default"), UpdatedAt: time.Now()},
		{ID: 99, Slot: ptrStr("slot2"), UpdatedAt: time.Now()},
	}

	result := selectSaveForSync(saves, "default")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

func TestSelectSaveForSync_ReturnsPreferredSlot(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("default"), UpdatedAt: time.Now()},
		{ID: 99, Slot: ptrStr("quicksave"), UpdatedAt: time.Now()},
	}

	result := selectSaveForSync(saves, "quicksave")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 99 {
		t.Errorf("expected ID 99, got %d", result.ID)
	}
}

func TestSelectSaveForSync_PicksLatestInSlot(t *testing.T) {
	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(1 * time.Hour)

	saves := []romm.Save{
		{ID: 10, Slot: ptrStr("default"), UpdatedAt: older},
		{ID: 20, Slot: ptrStr("default"), UpdatedAt: newer},
	}

	result := selectSaveForSync(saves, "default")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 20 {
		t.Errorf("expected ID 20 (latest), got %d", result.ID)
	}
}

func TestSelectSaveForSync_FallsBackToLatest(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: ptrStr("default"), UpdatedAt: time.Now()},
	}

	result := selectSaveForSync(saves, "nonexistent")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42 (fallback to latest), got %d", result.ID)
	}
}

func TestSelectSaveForSync_NilSlotTreatedAsDefault(t *testing.T) {
	saves := []romm.Save{
		{ID: 42, Slot: nil, UpdatedAt: time.Now()},
	}

	result := selectSaveForSync(saves, "default")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
}

func TestDetermineActions_WithSlotPreference(t *testing.T) {
	remoteUpdated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	path := makeTempSave(t, remoteUpdated.Add(1*time.Hour))

	localSaves := []LocalSave{
		{RomID: 1, RomName: "Mario", FilePath: path},
	}

	remoteSaves := map[int][]romm.Save{
		1: {
			{ID: 10, RomID: 1, UpdatedAt: remoteUpdated, Slot: ptrStr("default")},
			{ID: 20, RomID: 1, UpdatedAt: remoteUpdated, Slot: ptrStr("quicksave")},
		},
	}

	config := &internal.Config{
		SlotPreferences: map[string]string{"1": "quicksave"},
	}

	items := DetermineActions(localSaves, remoteSaves, "device-1", config)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].RemoteSave == nil {
		t.Fatal("expected RemoteSave to be set")
	}
	if items[0].RemoteSave.ID != 20 {
		t.Errorf("expected RemoteSave.ID 20 (quicksave slot), got %d", items[0].RemoteSave.ID)
	}
}

func TestDetermineActions_SlotFallbackForcesUpload(t *testing.T) {
	// When the preferred slot doesn't exist on the server, DetermineActions
	// should force an upload with nil RemoteSave so the slot gets created,
	// rather than comparing against a fallback save from a different slot.
	remoteUpdated := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	// Local file is OLDER than the remote — without fallback detection this
	// would normally be ActionDownload, which would be wrong.
	localMtime := remoteUpdated.Add(-1 * time.Hour)
	path := makeTempSave(t, localMtime)

	localSaves := []LocalSave{
		{RomID: 1, RomName: "Mario", FilePath: path},
	}

	// Remote only has "default" slot, but user prefers "quicksave"
	remoteSaves := map[int][]romm.Save{
		1: {{ID: 10, RomID: 1, UpdatedAt: remoteUpdated, Slot: ptrStr("default")}},
	}

	config := &internal.Config{
		SlotPreferences: map[string]string{"1": "quicksave"},
	}

	items := DetermineActions(localSaves, remoteSaves, "device-1", config)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != ActionUpload {
		t.Errorf("expected ActionUpload (slot fallback), got %s", items[0].Action)
	}
	if items[0].RemoteSave != nil {
		t.Error("expected nil RemoteSave when falling back to different slot")
	}
	if items[0].TargetSlot != "quicksave" {
		t.Errorf("expected TargetSlot 'quicksave', got %q", items[0].TargetSlot)
	}
}

// --- Helper function tests ---

func TestLocalSavesWithoutRemote(t *testing.T) {
	saves := []LocalSave{
		{RomID: 1, RomName: "Mario"},
		{RomID: 2, RomName: "Zelda"},
		{RomID: 3, RomName: "Metroid"},
	}
	remoteSaves := map[int][]romm.Save{
		1: {{ID: 10}},
		3: {{ID: 30}},
	}

	result := LocalSavesWithoutRemote(saves, remoteSaves)

	if len(result) != 1 {
		t.Fatalf("expected 1 save without remote, got %d", len(result))
	}
	if result[0].RomID != 2 {
		t.Errorf("expected RomID 2, got %d", result[0].RomID)
	}
}

func TestNewSaveUploadActions(t *testing.T) {
	saves := []LocalSave{
		{RomID: 1, RomName: "Mario"},
		{RomID: 2, RomName: "Zelda"},
	}

	items := NewSaveUploadActions(saves, nil)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.Action != ActionUpload {
			t.Errorf("expected ActionUpload, got %s", item.Action)
		}
	}
}
