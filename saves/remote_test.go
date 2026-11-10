package saves

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

// A fake RomM holding one save for the game.
func fakeRomM(t *testing.T, content string, fail bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var confirmed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/saves":
			json.NewEncoder(w).Encode([]romm.Save{
				{ID: 217, RomID: 4679, UpdatedAt: time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)},
				{ID: 218, RomID: 4679, UpdatedAt: time.Date(2026, 11, 10, 7, 55, 11, 0, time.UTC)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/saves/218/content":
			if fail {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(content))
		case r.Method == http.MethodPost && r.URL.Path == "/api/saves/218/downloaded":
			confirmed.Add(1)
			w.Write([]byte("{}"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, &confirmed
}

func TestRomMsCurrentSaveIsTheNewestInTheGamesSlot(t *testing.T) {
	server, _ := fakeRomM(t, "x", false)

	got, err := RemoteSaveFor(romm.NewClient(server.URL), &settings.Config{}, "device-1", 4679)

	if err != nil || got == nil || got.ID != 218 {
		t.Errorf("got %+v, err %v", got, err)
	}
}

// The copy the player wants is the one already on RomM.
func TestChoosingRomMsSaveReplacesTheLiveCopyAndPutsTheOthersAway(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "downloaded earlier", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "old", yesterday.Add(-48*time.Hour))
	dup := DuplicateSaves{RomID: 4679, RomName: "Pokemon Monomyth", Copies: []LocalSave{gpsp, mgba}}
	server, confirmed := fakeRomM(t, "the newest from RomM", false)
	remote := &romm.Save{ID: 218, RomID: 4679, UpdatedAt: time.Date(2026, 11, 10, 7, 55, 11, 0, time.UTC)}

	err := UseRemoteSave(romm.NewClient(server.URL), &settings.Config{}, "device-1", dup, remote, filepath.Join(root, "mGBA"))
	if err != nil {
		t.Fatal(err)
	}

	live := filepath.Join(root, "mGBA", "Game.srm")
	if data, _ := os.ReadFile(live); string(data) != "the newest from RomM" {
		t.Errorf("the live save holds %q", data)
	}
	if info, _ := os.Stat(live); !info.ModTime().Equal(remote.UpdatedAt) {
		t.Errorf("the downloaded save has time %v, want RomM's %v", info.ModTime(), remote.UpdatedAt)
	}
	if confirmed.Load() != 1 {
		t.Errorf("RomM was told of the download %d times, want once", confirmed.Load())
	}
	// What was replaced, and the copy in the other folder, are both kept.
	oldBackup := filepath.Join(root, "mGBA", ".backup", "Game [2026-11-07 20-15-30].srm")
	if data, _ := os.ReadFile(oldBackup); string(data) != "old" {
		t.Errorf("the replaced save is not kept at %s", oldBackup)
	}
	if _, err := os.Stat(gpsp.FilePath); !os.IsNotExist(err) {
		t.Error("the copy in the other folder is still where Grout would find it")
	}
	if data, _ := os.ReadFile(filepath.Join(root, "gpSP", ".backup", "Game [2026-11-09 20-15-30].srm")); string(data) != "downloaded earlier" {
		t.Error("the copy in the other folder is not kept in a backup")
	}
}

func TestChoosingRomMsSaveWhenTheLiveFolderHasNoCopyWritesUnderTheExistingName(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Pokemon Monomyth v1_0.srm", "stale", yesterday)
	dup := DuplicateSaves{RomID: 4679, Copies: []LocalSave{gpsp, {RomID: 4679, FileName: "x", FilePath: filepath.Join(root, "gpSP", "x")}}}
	server, _ := fakeRomM(t, "from RomM", false)

	err := UseRemoteSave(romm.NewClient(server.URL), &settings.Config{}, "device-1", dup, &romm.Save{ID: 218, RomID: 4679}, filepath.Join(root, "mGBA"))
	if err != nil {
		t.Fatal(err)
	}

	if data, _ := os.ReadFile(filepath.Join(root, "mGBA", "Pokemon Monomyth v1_0.srm")); string(data) != "from RomM" {
		t.Errorf("the save was not written under the game's usual name: %q", data)
	}
}

func TestIfRomMsSaveCannotBeDownloadedNoLocalCopyIsTouched(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "keep me", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "keep me too", yesterday)
	dup := DuplicateSaves{RomID: 4679, Copies: []LocalSave{gpsp, mgba}}
	server, _ := fakeRomM(t, "", true)

	err := UseRemoteSave(romm.NewClient(server.URL), &settings.Config{}, "device-1", dup, &romm.Save{ID: 218, RomID: 4679}, filepath.Join(root, "mGBA"))

	if err == nil {
		t.Fatal("reported success")
	}
	for path, want := range map[string]string{gpsp.FilePath: "keep me", mgba.FilePath: "keep me too"} {
		if data, _ := os.ReadFile(path); string(data) != want {
			t.Errorf("%s now holds %q", path, data)
		}
	}
}
