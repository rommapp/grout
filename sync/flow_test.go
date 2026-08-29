package sync

import (
	"encoding/json"
	"fmt"
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
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

func TestBuildDiscoveryItems_NextUIPreservesRomEmulatorDirectory(t *testing.T) {
	t.Setenv("CFW", "NEXTUI")
	basePath := t.TempDir()
	t.Setenv("BASE_PATH", basePath)

	romPath := filepath.Join(basePath, "Roms", "Game Boy Advance (MGBA)", "Final Fantasy Tactics Advance (USA).zip")
	uncovered := map[int]cfw.LocalRomFile{
		3112: {
			RomID:    3112,
			RomName:  "Final Fantasy Tactics Advance",
			FSSlug:   "GBA",
			FileName: "Final Fantasy Tactics Advance (USA).zip",
			FilePath: romPath,
		},
	}
	savesByRom := map[int][]romm.Save{
		3112: {{ID: 11, RomID: 3112, FileName: "Final Fantasy Tactics Advance (USA) [2026].srm", FileExtension: "srm", UpdatedAt: time.Now()}},
	}
	config := &internal.Config{PlatformsBinding: map[string]string{"GBA": "gba"}}

	items := buildDiscoveryItems(uncovered, savesByRom, config)
	if len(items) != 1 {
		t.Fatalf("expected 1 discovery item, got %d", len(items))
	}
	wantDir := filepath.Join(basePath, "Saves", "MGBA")
	if got := items[0].LocalSave.EmulatorDir; got != wantDir {
		t.Fatalf("EmulatorDir = %q, want %q", got, wantDir)
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

func TestFetchSavesForRomsBulkPreservesDuplicatesAndOrder(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		query := r.URL.Query()
		if query.Get("device_id") != "device-1" || !slices.Equal(query["rom_ids"], []string{"2", "3"}) || query.Has("rom_id") || query.Has("platform_id") {
			t.Errorf("bulk query = %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode([]romm.Save{
			{ID: 10, RomID: 2}, {ID: 12, RomID: 2}, {ID: 14, RomID: 3}, {ID: 10, RomID: 2},
		})
	}))
	defer server.Close()

	uncovered := map[int]cfw.LocalRomFile{2: {RomID: 2}, 3: {RomID: 3}}
	got := fetchSavesForRoms(romm.NewClient(server.URL), "device-1", uncovered)
	if requests.Load() != 1 {
		t.Fatalf("requests=%d, want 1", requests.Load())
	}
	wantIDs := []int{10, 12, 10}
	if len(got[2]) != len(wantIDs) {
		t.Fatalf("ROM 2 saves = %+v", got[2])
	}
	for i, want := range wantIDs {
		if got[2][i].ID != want {
			t.Fatalf("ROM 2 order[%d] = %d, want %d", i, got[2][i].ID, want)
		}
	}
	if len(got[3]) != 1 || got[3][0].ID != 14 {
		t.Fatalf("ROM 3 saves = %+v", got[3])
	}
}

func TestFetchSavesForRomsBulk1001RomsUsesThreeRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		ids := r.URL.Query()["rom_ids"]
		if len(ids) == 0 || len(ids) > 500 {
			t.Errorf("rom_ids count = %d", len(ids))
		}
		if slices.Contains(ids, "1001") {
			_ = json.NewEncoder(w).Encode([]romm.Save{{ID: 60, RomID: 1001}})
			return
		}
		_ = json.NewEncoder(w).Encode([]romm.Save{})
	}))
	defer server.Close()
	uncovered := make(map[int]cfw.LocalRomFile, 1001)
	for romID := 1; romID <= 1001; romID++ {
		uncovered[romID] = cfw.LocalRomFile{RomID: romID}
	}
	got := fetchSavesForRoms(romm.NewClient(server.URL), "device-large", uncovered)
	if requests.Load() != 3 || len(got) != 1 {
		t.Fatalf("requests=%d groups=%d", requests.Load(), len(got))
	}
}

func TestFetchSavesForRomsSuccessfulEmptyBulkNeverFallsBack(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode([]romm.Save{})
	}))
	defer server.Close()
	got := fetchSavesForRoms(romm.NewClient(server.URL), "device-empty", map[int]cfw.LocalRomFile{1: {RomID: 1}, 2: {RomID: 2}})
	if requests.Load() != 1 || len(got) != 0 {
		t.Fatalf("requests=%d saves=%+v", requests.Load(), got)
	}
}

func TestFetchSavesForRomsEmptyDeviceUsesPerRomFallbackOnly(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		query := r.URL.Query()
		if query.Get("rom_id") == "" || query.Has("device_id") {
			t.Errorf("empty-device fallback query = %q", r.URL.RawQuery)
		}
		var romID int
		_, _ = fmt.Sscan(query.Get("rom_id"), &romID)
		_ = json.NewEncoder(w).Encode([]romm.Save{{ID: romID, RomID: romID}})
	}))
	defer server.Close()
	uncovered := map[int]cfw.LocalRomFile{1: {RomID: 1}, 2: {RomID: 2}, 3: {RomID: 3}}
	got := fetchSavesForRoms(romm.NewClient(server.URL), "", uncovered)
	if requests.Load() != 3 || len(got) != 3 {
		t.Fatalf("requests=%d saves=%d", requests.Load(), len(got))
	}
}

func TestFetchSavesForRomsBulkErrorsFallBackAndStayBounded(t *testing.T) {
	for _, failure := range []string{"http", "decode", "network"} {
		t.Run(failure, func(t *testing.T) {
			var requests, active, peak atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestNumber := requests.Add(1)
				query := r.URL.Query()
				if query.Get("device_id") != "device-fallback" {
					t.Errorf("request %d device_id = %q", requestNumber, query.Get("device_id"))
				}
				if query.Get("rom_id") == "" {
					switch failure {
					case "http":
						w.WriteHeader(http.StatusBadGateway)
						_, _ = w.Write([]byte(`[{"id":999,"rom_id":1}]`))
					case "decode":
						_, _ = w.Write([]byte(`[{"id":999,"rom_id":1}`))
					case "network":
						hijacker, ok := w.(http.Hijacker)
						if !ok {
							t.Fatal("test server cannot hijack connection")
						}
						conn, _, err := hijacker.Hijack()
						if err != nil {
							t.Fatal(err)
						}
						_ = conn.Close()
					}
					return
				}
				current := active.Add(1)
				defer active.Add(-1)
				for {
					old := peak.Load()
					if current <= old || peak.CompareAndSwap(old, current) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				var romID int
				_, _ = fmt.Sscan(query.Get("rom_id"), &romID)
				_ = json.NewEncoder(w).Encode([]romm.Save{{ID: romID, RomID: romID}})
			}))
			defer server.Close()

			uncovered := make(map[int]cfw.LocalRomFile, 12)
			for romID := 1; romID <= 12; romID++ {
				uncovered[romID] = cfw.LocalRomFile{RomID: romID}
			}
			got := fetchSavesForRoms(romm.NewClient(server.URL), "device-fallback", uncovered)
			if requests.Load() != 13 {
				t.Fatalf("requests=%d, want 13", requests.Load())
			}
			if peak.Load() > maxConcurrentRequests {
				t.Fatalf("peak fallback concurrency=%d, max=%d", peak.Load(), maxConcurrentRequests)
			}
			if len(got) != 12 || got[1][0].ID == 999 {
				t.Fatalf("partial bulk response leaked or fallback incomplete: %+v", got)
			}
		})
	}
}

func TestFetchSavesForRomsFallbackWorkerPoolBoundsHighCardinality(t *testing.T) {
	for _, uncoveredCount := range []int{0, 3, 1888, 6000} {
		t.Run(fmt.Sprintf("U=%d", uncoveredCount), func(t *testing.T) {
			var requests, active, peak atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				current := active.Add(1)
				defer active.Add(-1)
				for {
					old := peak.Load()
					if current <= old || peak.CompareAndSwap(old, current) {
						break
					}
				}
				time.Sleep(50 * time.Microsecond)
				var romID int
				_, _ = fmt.Sscan(r.URL.Query().Get("rom_id"), &romID)
				_ = json.NewEncoder(w).Encode([]romm.Save{{ID: romID, RomID: romID}})
			}))
			defer server.Close()

			uncovered := make(map[int]cfw.LocalRomFile, uncoveredCount)
			for romID := 1; romID <= uncoveredCount; romID++ {
				uncovered[romID] = cfw.LocalRomFile{RomID: romID}
			}
			got := fetchSavesForRoms(romm.NewClient(server.URL), "", uncovered)
			if int(requests.Load()) != uncoveredCount || len(got) != uncoveredCount {
				t.Fatalf("requests=%d groups=%d", requests.Load(), len(got))
			}
			if peak.Load() > maxConcurrentRequests {
				t.Fatalf("peak requests=%d max=%d", peak.Load(), maxConcurrentRequests)
			}
			if uncoveredCount == 0 && peak.Load() != 0 {
				t.Fatalf("U=0 peak=%d", peak.Load())
			}
		})
	}
}

func TestFetchSavesForRomsFallbackOmitsOnlyFailedRoms(t *testing.T) {
	for name, failed := range map[string]map[int]bool{
		"one":      {2: true},
		"multiple": {2: true, 4: true, 6: true},
	} {
		t.Run(name, func(t *testing.T) {
			var requests, active, peak atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				romIDText := r.URL.Query().Get("rom_id")
				if romIDText == "" {
					http.Error(w, "bulk unavailable", http.StatusBadGateway)
					return
				}
				current := active.Add(1)
				defer active.Add(-1)
				for {
					old := peak.Load()
					if current <= old || peak.CompareAndSwap(old, current) {
						break
					}
				}
				var romID int
				_, _ = fmt.Sscan(romIDText, &romID)
				if failed[romID] {
					http.Error(w, "per-rom unavailable", http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode([]romm.Save{{ID: romID, RomID: romID}})
			}))
			defer server.Close()

			uncovered := make(map[int]cfw.LocalRomFile, 8)
			for romID := 1; romID <= 8; romID++ {
				uncovered[romID] = cfw.LocalRomFile{RomID: romID}
			}
			got := fetchSavesForRoms(romm.NewClient(server.URL), "device-failures", uncovered)
			if requests.Load() != 9 || len(got) != 8-len(failed) {
				t.Fatalf("requests=%d groups=%d", requests.Load(), len(got))
			}
			for romID := range failed {
				if _, ok := got[romID]; ok {
					t.Fatalf("failed ROM %d leaked into results", romID)
				}
			}
			if peak.Load() > maxConcurrentRequests {
				t.Fatalf("peak=%d max=%d", peak.Load(), maxConcurrentRequests)
			}
		})
	}
}

func TestFetchSavesForRomsFallbackRejectsMismatchedROM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("rom_id") == "" {
			http.Error(w, "bulk unavailable", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode([]romm.Save{{ID: 99, RomID: 999}})
	}))
	defer server.Close()

	uncovered := map[int]cfw.LocalRomFile{7: {RomID: 7}}
	got := fetchSavesForRoms(romm.NewClient(server.URL), "device-1", uncovered)
	if len(got) != 0 {
		t.Fatalf("mismatched save leaked into ROM 7 results: %+v", got)
	}
}

func TestFetchSavesForRomsFallbackRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("rom_id") == "" {
			http.Error(w, "bulk unavailable", http.StatusBadGateway)
			return
		}
		saves := make([]romm.Save, 10001)
		for i := range saves {
			saves[i] = romm.Save{ID: i + 1, RomID: 7}
		}
		_ = json.NewEncoder(w).Encode(saves)
	}))
	defer server.Close()

	uncovered := map[int]cfw.LocalRomFile{7: {RomID: 7}}
	got := fetchSavesForRoms(romm.NewClient(server.URL), "device-1", uncovered)
	if len(got) != 0 {
		t.Fatalf("oversized fallback response produced download candidates: %d", len(got[7]))
	}
}

func TestFetchSavesForRomsServerIgnoresDeviceStillFiltersUncovered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]romm.Save{
			{ID: 1, RomID: 7}, {ID: 2, RomID: 999}, {ID: 3, RomID: 7}, {ID: 1, RomID: 7},
		})
	}))
	defer server.Close()
	got := fetchSavesForRoms(romm.NewClient(server.URL), "device-ignored", map[int]cfw.LocalRomFile{7: {RomID: 7}})
	if len(got) != 1 {
		t.Fatalf("got=%+v", got)
	}
	for i, want := range []int{1, 3, 1} {
		if got[7][i].ID != want {
			t.Fatalf("order[%d]=%d want=%d", i, got[7][i].ID, want)
		}
	}
}

func TestDiscoverRemoteOnlySavesCreatesIntentWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "two.srm")
	if err := os.WriteFile(savePath, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode([]romm.Save{{ID: 20, RomID: 2, FileName: "two.srm"}})
	}))
	defer server.Close()
	resolved := map[int]cfw.LocalRomFile{
		1: {RomID: 1, FSSlug: "gba", FileName: "one.gba"},
		2: {RomID: 2, FSSlug: "gba", FileName: "two.gba"},
	}
	items := discoverRemoteOnlySaves(romm.NewClient(server.URL), nil, "device-covered", []LocalSave{{RomID: 1}}, nil, resolved)
	if requests.Load() != 1 || len(items) != 1 || items[0].LocalSave.RomID != 2 || items[0].Action != ActionDownload {
		t.Fatalf("requests=%d items=%+v", requests.Load(), items)
	}
	got, err := os.ReadFile(savePath)
	if err != nil || string(got) != "untouched" {
		t.Fatalf("discovery modified save: data=%q err=%v", got, err)
	}
}

func TestDiscoverRemoteOnlySavesZeroUncoveredMakesNoRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("zero-uncovered discovery made unexpected request: %s", r.URL.String())
	}))
	defer server.Close()
	resolved := map[int]cfw.LocalRomFile{1: {RomID: 1}}
	items := discoverRemoteOnlySaves(romm.NewClient(server.URL), nil, "device-zero", []LocalSave{{RomID: 1}}, nil, resolved)
	if items != nil {
		t.Fatalf("items=%+v, want nil", items)
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
	items := mapOperationsToItems(ops, local, resolved, nil, nil, nil, nil)

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
	items := mapOperationsToItems(ops, nil, resolved, nil, nil, nil, nil)
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

	items := mapOperationsToItems(ops, nil, resolved, nil, nil, nil, nil)

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

func TestMapOperationsToItems_SkipsOtherSlotDownloadWhenLocalSaveExists(t *testing.T) {
	// The ROM already has a local save synced under "autosave". The server offers a
	// "default"-slot save for the same ROM — grout manages one slot per ROM, so it must
	// NOT pull the other slot (which would clobber the local save and flip-flop).
	local := []LocalSave{{RomID: 303, FileName: "Pokemon.srm", FilePath: "/x/Pokemon.srm", FSSlug: "gba"}}
	recorded := map[saveKey]string{{romID: 303, fileName: "Pokemon.srm"}: "autosave"}
	ops := []romm.SyncOperationSchema{
		{Action: "download", RomID: 303, SaveID: ptrInt(228), FileName: "P [d].srm", Slot: ptrStr("default"), ServerUpdatedAt: ptrTime(time.Now())},
	}

	items := mapOperationsToItems(ops, local, nil, nil, nil, recorded, nil)

	if len(items) != 0 {
		t.Fatalf("expected other-slot download to be skipped, got %d", len(items))
	}
}

func TestMapOperationsToItems_AcceptsSameSlotDownloadWhenLocalSaveExists(t *testing.T) {
	// A download for the ROM's own (managed) slot — e.g. the server copy is newer — is
	// legitimate and must be applied.
	local := []LocalSave{{RomID: 303, FileName: "Pokemon.srm", FilePath: "/x/Pokemon.srm", FSSlug: "gba"}}
	recorded := map[saveKey]string{{romID: 303, fileName: "Pokemon.srm"}: "autosave"}
	ops := []romm.SyncOperationSchema{
		{Action: "download", RomID: 303, SaveID: ptrInt(235), FileName: "P [a].srm", Slot: ptrStr("autosave"), ServerUpdatedAt: ptrTime(time.Now())},
	}

	items := mapOperationsToItems(ops, local, nil, nil, nil, recorded, nil)

	if len(items) != 1 || items[0].RemoteSave == nil || items[0].RemoteSave.ID != 235 {
		t.Fatalf("expected same-slot download to be applied, got %+v", items)
	}
}

func TestBuildUploadQuery_OverwriteOnlyWhenForced(t *testing.T) {
	// A normal upload op (orchestrator said client is newer) carries a RemoteSave stub but
	// must NOT force overwrite — overwrite=false lets the server's 409 guard catch races.
	normal := &SyncItem{
		LocalSave:  LocalSave{RomID: 303, EmulatorDir: "/saves/mGBA"},
		RemoteSave: &romm.Save{ID: 235},
		TargetSlot: "autosave",
		Action:     ActionUpload,
	}
	if q := buildUploadQuery("dev-1", normal); q.Overwrite {
		t.Errorf("normal upload must send overwrite=false, got true")
	}

	// A conflict resolved as keep-local sets ForceOverwrite → overwrite=true.
	forced := &SyncItem{
		LocalSave:      LocalSave{RomID: 303, EmulatorDir: "/saves/mGBA"},
		RemoteSave:     &romm.Save{ID: 235},
		TargetSlot:     "autosave",
		ForceOverwrite: true,
		Action:         ActionUpload,
	}
	if q := buildUploadQuery("dev-1", forced); !q.Overwrite {
		t.Errorf("keep-local upload must send overwrite=true, got false")
	}
}

func TestBuildUploadQuery_AutocleanupOnlyForAutosave(t *testing.T) {
	autosave := &SyncItem{LocalSave: LocalSave{RomID: 1, EmulatorDir: "/s/mGBA"}, TargetSlot: "autosave"}
	if q := buildUploadQuery("d", autosave); !q.Autocleanup || q.AutocleanupLimit != 10 {
		t.Errorf("autosave slot should enable autocleanup limit 10, got %+v", q)
	}
	named := &SyncItem{LocalSave: LocalSave{RomID: 1, EmulatorDir: "/s/mGBA"}, TargetSlot: "quicksave"}
	if q := buildUploadQuery("d", named); q.Autocleanup {
		t.Errorf("named slot should not enable autocleanup, got %+v", q)
	}
}

func TestMapOperationsToItems_UploadMatchesBySlotNotFilename(t *testing.T) {
	// The server datetime-tags slot saves, so the upload op's file_name does not equal
	// grout's plain local filename. The op must still pair to the local save by
	// (rom_id, slot) — matching the orchestrator's and Argosy's pairing key.
	local := []LocalSave{{RomID: 303, FileName: "Pokemon.srm", FilePath: "/x/Pokemon.srm", FSSlug: "gba"}}
	ops := []romm.SyncOperationSchema{
		{
			Action: "upload", RomID: 303, SaveID: ptrInt(235),
			FileName:        "Pokemon [2026-06-09_14-49-22].srm", // server-tagged, != local name
			Slot:            ptrStr("autosave"),
			ServerUpdatedAt: ptrTime(time.Now()),
		},
	}

	items := mapOperationsToItems(ops, local, nil, nil, nil, nil, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 upload item matched by slot, got %d", len(items))
	}
	if items[0].Action != ActionUpload {
		t.Errorf("action = %v, want upload", items[0].Action)
	}
	if items[0].LocalSave.FileName != "Pokemon.srm" {
		t.Errorf("matched wrong local save: %q", items[0].LocalSave.FileName)
	}
	if items[0].TargetSlot != "autosave" {
		t.Errorf("target slot = %q, want autosave", items[0].TargetSlot)
	}
}

func TestMapOperationsToItems_DownloadKeepsExistingLocalPath(t *testing.T) {
	localPath := "/mnt/SDCARD/Saves/MGBA/Final Fantasy Tactics Advance (USA).zip.sav"
	local := []LocalSave{{
		RomID:       3112,
		RomName:     "Final Fantasy Tactics Advance",
		FSSlug:      "GBA",
		FileName:    "Final Fantasy Tactics Advance (USA).zip.sav",
		FilePath:    localPath,
		EmulatorDir: "/mnt/SDCARD/Saves/MGBA",
	}}
	ops := []romm.SyncOperationSchema{{
		Action:          "download",
		RomID:           3112,
		SaveID:          ptrInt(13),
		FileName:        "Final Fantasy Tactics Advance (USA).zip [2026-08-21_06-52-14].sav",
		Slot:            ptrStr("autosave"),
		ServerUpdatedAt: ptrTime(time.Now()),
	}}

	items := mapOperationsToItems(ops, local, nil, nil, nil, nil, nil)
	if len(items) != 1 {
		t.Fatalf("expected 1 download item, got %d", len(items))
	}
	if got := items[0].LocalSave.FilePath; got != localPath {
		t.Fatalf("download path = %q, want existing local path %q", got, localPath)
	}
	if got := items[0].LocalSave.EmulatorDir; got != "/mnt/SDCARD/Saves/MGBA" {
		t.Fatalf("emulator dir = %q, want existing MGBA directory", got)
	}
}

func TestMapOperationsToItems_ConflictMatchesBySlot(t *testing.T) {
	local := []LocalSave{{RomID: 7, FileName: "Mario.srm", FilePath: "/x/Mario.srm", FSSlug: "snes"}}
	recorded := map[saveKey]string{{romID: 7, fileName: "Mario.srm"}: "quicksave"}
	ops := []romm.SyncOperationSchema{
		{
			Action: "conflict", RomID: 7, SaveID: ptrInt(99),
			FileName:        "Mario [2026-06-01_10-00-00].srm",
			Slot:            ptrStr("quicksave"),
			ServerUpdatedAt: ptrTime(time.Now()),
		},
	}

	items := mapOperationsToItems(ops, local, nil, nil, nil, recorded, nil)

	if len(items) != 1 || items[0].Action != ActionConflict {
		t.Fatalf("expected 1 conflict item, got %+v", items)
	}
	if items[0].TargetSlot != "quicksave" {
		t.Errorf("target slot = %q, want quicksave", items[0].TargetSlot)
	}
}

func TestMapOperationsToItems_FirstTimeMultiSlotOffersChoice(t *testing.T) {
	// rom 303 is installed but has no local save; the server offers two slots. The item
	// should carry AvailableSlots + AllRemoteSaves so the UI can prompt for a choice.
	resolved := map[int]cfw.LocalRomFile{
		303: {RomID: 303, RomName: "Pokemon", FSSlug: "gba", FileName: "Pokemon.gba"},
	}
	now := time.Now()
	ops := []romm.SyncOperationSchema{
		{Action: "download", RomID: 303, SaveID: ptrInt(235), FileName: "P [a].srm", Slot: ptrStr("autosave"), ServerUpdatedAt: ptrTime(now)},
		{Action: "download", RomID: 303, SaveID: ptrInt(228), FileName: "P [q].srm", Slot: ptrStr("quicksave"), ServerUpdatedAt: ptrTime(now)},
	}

	items := mapOperationsToItems(ops, nil, resolved, nil, nil, nil, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if len(it.AvailableSlots) != 2 || it.AvailableSlots[0] != "autosave" || it.AvailableSlots[1] != "quicksave" {
		t.Errorf("AvailableSlots = %v, want [autosave quicksave]", it.AvailableSlots)
	}
	if len(it.AllRemoteSaves) != 2 {
		t.Errorf("AllRemoteSaves = %d, want 2", len(it.AllRemoteSaves))
	}
}

func TestMapOperationsToItems_LocalSaveDoesNotOfferMultiSlot(t *testing.T) {
	// ROM already has a local save in "autosave"; the "quicksave" download is skipped by
	// the managed-slot gate, so no picker is offered.
	local := []LocalSave{{RomID: 303, FileName: "Pokemon.srm", FilePath: "/x/Pokemon.srm", FSSlug: "gba"}}
	recorded := map[saveKey]string{{romID: 303, fileName: "Pokemon.srm"}: "autosave"}
	now := time.Now()
	ops := []romm.SyncOperationSchema{
		{Action: "download", RomID: 303, SaveID: ptrInt(228), FileName: "P [q].srm", Slot: ptrStr("quicksave"), ServerUpdatedAt: ptrTime(now)},
	}

	items := mapOperationsToItems(ops, local, nil, nil, nil, recorded, nil)
	if len(items) != 0 {
		t.Fatalf("expected other-slot download skipped (no picker), got %d items", len(items))
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

// When a save is downloaded for a ROM with no existing local save, it must be written
// under the filename the emulator will look for: keepRomExt=true (minarch) retains the ROM
// extension, false (RetroArch) strips it (issue #245). Falls back to the server filename
// when the ROM filename is unknown.
func TestDownloadSaveFileName(t *testing.T) {
	if got := downloadSaveFileName("Donkey Kong Country (USA) (Rev 2).sfc", "Server [2026].srm", "sav", true); got != "Donkey Kong Country (USA) (Rev 2).sfc.sav" {
		t.Errorf("keep: got %q, want %q", got, "Donkey Kong Country (USA) (Rev 2).sfc.sav")
	}
	if got := downloadSaveFileName("Pokemon - Emerald Version (USA, Europe).gba", "Server [2026].srm", "srm", false); got != "Pokemon - Emerald Version (USA, Europe).srm" {
		t.Errorf("strip: got %q, want %q", got, "Pokemon - Emerald Version (USA, Europe).srm")
	}
	if got := downloadSaveFileName("", "Server [2026-01-01_00-00-00].srm", "srm", true); got != "Server [2026-01-01_00-00-00].srm" {
		t.Errorf("fallback to server name: got %q", got)
	}
}

// saveDirKeepsRomExt infers the device's save-naming style from existing save files in a
// directory, so a fresh download is written under the convention the emulator already uses
// (issue #245). NextUI supports both styles, so this is detected, not assumed.
func TestSaveDirKeepsRomExt(t *testing.T) {
	keep, known := saveDirKeepsRomExt([]string{"Donkey Kong Country (USA) (Rev 2).sfc.sav"})
	if !known || !keep {
		t.Errorf("retained-ext dir: keep=%v known=%v, want true/true", keep, known)
	}

	keep, known = saveDirKeepsRomExt([]string{"Pokemon - Emerald Version (USA, Europe).srm"})
	if !known || keep {
		t.Errorf("retroarch dir: keep=%v known=%v, want false/true", keep, known)
	}

	// A dotted version token must not be mistaken for a retained ROM extension.
	keep, known = saveDirKeepsRomExt([]string{"Final Fantasy IV (J) (V1.1).srm"})
	if !known || keep {
		t.Errorf("dotted-title dir: keep=%v known=%v, want false/true", keep, known)
	}

	// Non-save files are ignored; a dir with nothing to infer from is unknown.
	if _, known := saveDirKeepsRomExt([]string{"notes.txt", ".nomedia"}); known {
		t.Errorf("no save files: known=%v, want false", known)
	}
}

// saveLookupKeys turns a scanned save's no-extension name into the ROM expected-basename
// candidates it could match. RetroArch CFWs name the save <rombase>.<ext>, so the name
// IS the rom basename; minarch CFWs (NextUI/MinUI) name it <rombase>.<romext>.<ext>, so
// stripping one more (ROM-looking) extension recovers the rom basename (issue #245).
func TestSaveLookupKeys(t *testing.T) {
	cases := []struct {
		name      string
		nameNoExt string
		want      []string
	}{
		{
			"RetroArch name is already the rom basename",
			"Pokemon - Emerald Version (USA, Europe)",
			[]string{"Pokemon - Emerald Version (USA, Europe)"},
		},
		{
			"NextUI retained ROM extension yields a stripped candidate",
			"Donkey Kong Country (USA) (Rev 2).sfc",
			[]string{"Donkey Kong Country (USA) (Rev 2).sfc", "Donkey Kong Country (USA) (Rev 2)"},
		},
		{
			"retained extension after an earlier dotted token",
			"Machine, The (World) (Rev v1.2) (Unl).gbc",
			[]string{"Machine, The (World) (Rev v1.2) (Unl).gbc", "Machine, The (World) (Rev v1.2) (Unl)"},
		},
		{
			"a dotted version token is not mistaken for a ROM extension",
			"Final Fantasy IV (J) (V1.1)",
			[]string{"Final Fantasy IV (J) (V1.1)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := saveLookupKeys(tc.nameNoExt)
			if len(got) != len(tc.want) {
				t.Fatalf("saveLookupKeys(%q) = %v, want %v", tc.nameNoExt, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("saveLookupKeys(%q) = %v, want %v", tc.nameNoExt, got, tc.want)
				}
			}
		})
	}
}

// An explicit "autosave" choice must override a recorded non-autosave slot, so a user
// can switch a ROM back to autosave to sync against another client (issue #250).
func TestBuildClientSaveStates_ExplicitAutosaveOverridesRecorded(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Pokemon.srm")
	if err := os.WriteFile(p, []byte("savedata"), 0644); err != nil {
		t.Fatal(err)
	}
	local := []LocalSave{{RomID: 6, FileName: "Pokemon.srm", FilePath: p, EmulatorDir: "mgba"}}
	recorded := map[saveKey]string{{romID: 6, fileName: "Pokemon.srm"}: "default"}

	cfg := &internal.Config{}
	cfg.SetSlotPreference(6, "autosave") // user explicitly picks autosave

	states := buildClientSaveStates(local, cfg, recorded)
	if len(states) != 1 || states[0].Slot != "autosave" {
		t.Fatalf("explicit autosave must override recorded 'default': got %+v", states)
	}
}
