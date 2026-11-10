package saves

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

// An upload-only run (after a game, at power-off) sends and never replaces. A
// server that turns the upload down because its own save is ahead must leave the
// device's save alone.
func TestAnUploadOnlyItemTurnedDownBecauseRomMIsAheadIsLeftAloneNotDownloaded(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "the device's", yesterday)
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/saves/218/content" {
			downloads.Add(1)
		}
		w.Write([]byte("from RomM"))
	}))
	defer server.Close()
	item := SyncItem{
		LocalSave:  local,
		RemoteSave: &romm.Save{ID: 218, RomID: 4679, UpdatedAt: time.Now()},
		Action:     ActionUpload,
		UploadOnly: true,
	}
	var report SyncReport

	settleSuperseded(romm.NewClient(server.URL), &settings.Config{}, "device-1", &item, &report)

	if downloads.Load() != 0 {
		t.Error("an upload-only run downloaded RomM's save")
	}
	if data, _ := os.ReadFile(local.FilePath); string(data) != "the device's" {
		t.Errorf("the device's save was replaced by %q", data)
	}
	if item.Action != ActionConflict || report.Conflicts != 1 || report.Downloaded != 0 {
		t.Errorf("action=%v conflicts=%d downloaded=%d", item.Action, report.Conflicts, report.Downloaded)
	}
}

func TestAnItemThatMayDownloadStillTakesRomMsSaveWhenItsUploadIsSuperseded(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "unchanged since the last sync", yesterday)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/saves/218/content" {
			w.Write([]byte("from RomM"))
			return
		}
		w.Write([]byte("{}"))
	}))
	defer server.Close()
	item := SyncItem{LocalSave: local, RemoteSave: &romm.Save{ID: 218, RomID: 4679, UpdatedAt: time.Now()}, Action: ActionUpload}
	var report SyncReport

	settleSuperseded(romm.NewClient(server.URL), &settings.Config{}, "device-1", &item, &report)

	if report.Downloaded != 1 {
		t.Errorf("downloaded=%d, want the usual reconcile", report.Downloaded)
	}
	if data, _ := os.ReadFile(local.FilePath); string(data) != "from RomM" {
		t.Errorf("the save holds %q", data)
	}
}

func TestOnlyAnUploadsOnlyFilterMarksItemsUploadOnly(t *testing.T) {
	kept := UploadsOnly([]SyncItem{{Action: ActionUpload}, {Action: ActionDownload}})

	if len(kept) != 1 || !kept[0].UploadOnly {
		t.Errorf("got %+v", kept)
	}
}

// The resume state has to go the moment the save is on the card: the sync is
// stopped by a time limit, and it must not be stopped between the two.
func TestWhateverFollowsADownloadedSaveRunsBeforeRomMIsToldAboutIt(t *testing.T) {
	root := t.TempDir()
	local := copyIn(t, root, 4679, "mGBA", "Game.srm", "old", yesterday)
	var confirmedAtCallback, confirmed atomic.Int32
	var seenOnDisk string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/saves/218/content":
			w.Write([]byte("the new save"))
		case "/api/saves/218/downloaded":
			confirmed.Add(1)
			w.Write([]byte("{}"))
		}
	}))
	defer server.Close()
	ran := false
	item := SyncItem{
		LocalSave:  local,
		RemoteSave: &romm.Save{ID: 218, RomID: 4679, UpdatedAt: time.Now()},
		Action:     ActionDownload,
		AfterWrite: func() {
			ran = true
			confirmedAtCallback.Store(confirmed.Load())
			data, _ := os.ReadFile(local.FilePath)
			seenOnDisk = string(data)
		},
	}

	if !download(romm.NewClient(server.URL), &settings.Config{}, "device-1", &item) {
		t.Fatal("download failed")
	}

	if !ran {
		t.Fatal("the callback never ran")
	}
	if seenOnDisk != "the new save" {
		t.Errorf("when the callback ran the save on the card was %q, not the new one", seenOnDisk)
	}
	if confirmedAtCallback.Load() != 0 {
		t.Error("RomM had already been told of the download when the callback ran")
	}
	if confirmed.Load() != 1 {
		t.Errorf("RomM was told %d times", confirmed.Load())
	}
}
