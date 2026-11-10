package saves

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func saveIn(t *testing.T, root, folder, name, content string) LocalSave {
	t.Helper()
	path := filepath.Join(root, folder, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return LocalSave{RomID: 1, FileName: name, FilePath: path, EmulatorDir: folder}
}

func TestOnlySavesInOtherFoldersFollowTheGame(t *testing.T) {
	root := t.TempDir()
	inGpsp := saveIn(t, root, "gpSP", "Game.srm", "old")

	got := SavesToCarry([]LocalSave{inGpsp}, "mGBA")
	if len(got) != 1 || got[0].FilePath != inGpsp.FilePath {
		t.Errorf("got %+v", got)
	}
}

func TestASaveAlreadyInTheNewFolderIsLeftToTheGame(t *testing.T) {
	root := t.TempDir()
	inGpsp := saveIn(t, root, "gpSP", "Game.srm", "old")
	inMgba := saveIn(t, root, "mGBA", "Game.srm", "current")

	if got := SavesToCarry([]LocalSave{inGpsp, inMgba}, "mgba"); got != nil {
		t.Errorf("would move %+v over the save the game already has", got)
	}
}

func TestSavesKeptAsFoldersAreNotCarried(t *testing.T) {
	dir := LocalSave{IsDirectorySave: true, FilePath: "/saves/PSP/SAVEDATA/X"}
	if got := SavesToCarry([]LocalSave{dir}, "mGBA"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestCarryingASaveMovesItAndKeepsABackup(t *testing.T) {
	root := t.TempDir()
	save := saveIn(t, root, "gpSP", "Game.srm", "progress")
	saved := time.Date(2026, 11, 9, 20, 15, 30, 0, time.Local)
	os.Chtimes(save.FilePath, saved, saved)

	if err := CarrySave(save, filepath.Join(root, "mGBA")); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(root, "mGBA", "Game.srm")
	if data, _ := os.ReadFile(dest); string(data) != "progress" {
		t.Errorf("the new folder holds %q", data)
	}
	if info, _ := os.Stat(dest); !info.ModTime().Equal(saved) {
		t.Errorf("the moved save lost its time: %v", info.ModTime())
	}
	if _, err := os.Stat(save.FilePath); !os.IsNotExist(err) {
		t.Error("the original is still where Grout would find it")
	}
	backup := filepath.Join(root, "gpSP", ".backup", "Game [2026-11-09 20-15-30].srm")
	if data, _ := os.ReadFile(backup); string(data) != "progress" {
		t.Errorf("no backup of the original at %s", backup)
	}
}

func TestCarryingNeverOverwritesASaveInTheNewFolder(t *testing.T) {
	root := t.TempDir()
	save := saveIn(t, root, "gpSP", "Game.srm", "old")
	saveIn(t, root, "mGBA", "Game.srm", "current")

	if err := CarrySave(save, filepath.Join(root, "mGBA")); err == nil {
		t.Error("overwrote a save")
	}
	if data, _ := os.ReadFile(filepath.Join(root, "mGBA", "Game.srm")); string(data) != "current" {
		t.Errorf("the save in the new folder was changed to %q", data)
	}
	if _, err := os.Stat(save.FilePath); err != nil {
		t.Error("the original was moved although nothing was carried")
	}
}

func TestIfTheOriginalCannotBePutAwayTheCopyIsRemoved(t *testing.T) {
	root := t.TempDir()
	save := saveIn(t, root, "gpSP", "Game.srm", "progress")
	// A file where the backup folder has to go.
	os.WriteFile(filepath.Join(root, "gpSP", ".backup"), []byte("in the way"), 0644)

	if err := CarrySave(save, filepath.Join(root, "mGBA")); err == nil {
		t.Fatal("reported success")
	}
	if _, err := os.Stat(filepath.Join(root, "mGBA", "Game.srm")); !os.IsNotExist(err) {
		t.Error("a second copy of the save was left behind")
	}
	if _, err := os.Stat(save.FilePath); err != nil {
		t.Error("the original is gone")
	}
}
