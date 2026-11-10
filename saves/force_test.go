package saves

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

func TestRomMsLatestIsTheNewestInAnySlot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]romm.Save{
			{ID: 1, RomID: 7, Slot: str("autosave"), UpdatedAt: time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)},
			{ID: 2, RomID: 7, Slot: str("boss"), UpdatedAt: time.Date(2026, 11, 10, 7, 55, 0, 0, time.UTC)},
			{ID: 3, RomID: 7, Slot: str("autosave"), UpdatedAt: time.Date(2026, 11, 8, 1, 0, 0, 0, time.UTC)},
		})
	}))
	defer server.Close()

	got, err := NewestRemoteSave(romm.NewClient(server.URL), "device-1", 7)

	if err != nil || got == nil || got.ID != 2 || SlotOf(got) != "boss" {
		t.Errorf("got %+v, err %v", got, err)
	}
}

func TestNoSavesOnRomMIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("[]")) }))
	defer server.Close()

	if got, err := NewestRemoteSave(romm.NewClient(server.URL), "device-1", 7); got != nil || err != nil {
		t.Errorf("got %+v, err %v", got, err)
	}
}

// The player's choice: RomM takes the device's save even though RomM's is newer.
func TestForceUploadTellsRomMToOverwriteAndSendsTheDevicesSave(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "the miyoo's progress", yesterday)
	var sawOverwrite, sawSlot, sawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/saves" {
			http.NotFound(w, r)
			return
		}
		sawOverwrite = r.URL.Query().Get("overwrite")
		sawSlot = r.URL.Query().Get("slot")
		file, _, err := r.FormFile("saveFile")
		if err == nil {
			data, _ := io.ReadAll(file)
			sawBody = string(data)
		}
		json.NewEncoder(w).Encode(romm.Save{ID: 300, RomID: 4679, UpdatedAt: time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)})
	}))
	defer server.Close()

	err := ForceUpload(romm.NewClient(server.URL), &settings.Config{}, "device-1", local)

	if err != nil {
		t.Fatal(err)
	}
	if sawOverwrite != "true" {
		t.Errorf("overwrite = %q, want true: RomM would have refused a save it thinks is older", sawOverwrite)
	}
	if sawSlot != "autosave" {
		t.Errorf("slot = %q", sawSlot)
	}
	if sawBody != "the miyoo's progress" {
		t.Errorf("RomM was sent %q", sawBody)
	}
}

func TestForceUploadReportsRomMRefusingIt(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "x", yesterday)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := ForceUpload(romm.NewClient(server.URL), &settings.Config{}, "device-1", local); err == nil {
		t.Error("reported success although RomM answered with an error")
	}
}

// A game with no save on the device at all can be given RomM's.
func TestRomMsSaveIsPutInPlaceForAGameWithNoSaveOnTheDevice(t *testing.T) {
	root := t.TempDir()
	server, confirmed := fakeRomM(t, "from RomM", false)
	dup := DuplicateSaves{RomID: 4679, RomName: "Pokemon Monomyth", FSSlug: "gba", RomFileName: "Pokemon Monomyth v1_0.zip"}
	remote := &romm.Save{ID: 218, RomID: 4679, FileName: "Pokemon Monomyth v1_0 [2026-11-10_07-55-11].srm", FileExtension: "srm",
		UpdatedAt: time.Date(2026, 11, 10, 7, 55, 11, 0, time.UTC)}

	err := UseRemoteSave(romm.NewClient(server.URL), &settings.Config{}, "device-1", dup, remote, filepath.Join(root, "mGBA"))

	if err != nil {
		t.Fatal(err)
	}
	// Named after the ROM, the way a sync names a new save, not after RomM's tagged file.
	if data, _ := os.ReadFile(filepath.Join(root, "mGBA", "Pokemon Monomyth v1_0.srm")); string(data) != "from RomM" {
		entries, _ := os.ReadDir(filepath.Join(root, "mGBA"))
		t.Errorf("the save is not under the game's name; folder holds %v", entries)
	}
	if confirmed.Load() != 1 {
		t.Errorf("RomM was told of the download %d times", confirmed.Load())
	}
}

func TestFollowingASaveFromAnotherSlotMovesAnExplicitSlotChoice(t *testing.T) {
	config := &settings.Config{}
	config.SetSlotPreference(4679, "autosave")

	if !FollowSlot(config, 4679, &romm.Save{Slot: str("boss")}) {
		t.Error("an explicit choice of another slot was left in place")
	}
	if got := config.GetSlotPreference(4679); got != "boss" {
		t.Errorf("slot preference = %q", got)
	}

	// With no explicit choice the slot RomM's save is in is recorded when it is
	// downloaded, so there is nothing to change.
	fresh := &settings.Config{}
	if FollowSlot(fresh, 4679, &romm.Save{Slot: str("boss")}) {
		t.Error("created a slot choice nobody made")
	}
}
