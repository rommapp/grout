package saves

import (
	"os"
	"path/filepath"
	"testing"

	"grout/cfw"
)

func TestForLaunchKeepsOnlyWhatCanRunUnattended(t *testing.T) {
	items := []SyncItem{
		{LocalSave: LocalSave{RomID: 1}, Action: ActionDownload},
		{LocalSave: LocalSave{RomID: 1}, Action: ActionUpload},
		{LocalSave: LocalSave{RomID: 1}, Action: ActionConflict},
		{LocalSave: LocalSave{RomID: 1}, Action: ActionDownload, AvailableSlots: []string{"autosave", "boss"}},
		{LocalSave: LocalSave{RomID: 1}, Action: ActionSkip},
		{LocalSave: LocalSave{RomID: 2}, Action: ActionDownload},
	}

	got := ForLaunch(items, 1)

	want := []SyncAction{ActionDownload, ActionUpload, ActionSkip}
	if len(got) != len(want) {
		t.Fatalf("kept %d items, want %d: %+v", len(got), len(want), got)
	}
	for i, action := range want {
		if got[i].Action != action || got[i].LocalSave.RomID != 1 {
			t.Errorf("item %d = %v for rom %d, want %v for rom 1", i, got[i].Action, got[i].LocalSave.RomID, action)
		}
	}
}

func TestForLaunchWithNothingForTheRomIsEmpty(t *testing.T) {
	items := []SyncItem{{LocalSave: LocalSave{RomID: 2}, Action: ActionDownload}}
	if got := ForLaunch(items, 1); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestFindRomByPath(t *testing.T) {
	dir := t.TempDir()
	rom := filepath.Join(dir, "Roms", "GB", "Tetris.gb")
	if err := os.MkdirAll(filepath.Dir(rom), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rom, []byte("rom"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.gb")
	if err := os.Symlink(rom, link); err != nil {
		t.Fatal(err)
	}

	resolved := map[int]cfw.LocalRomFile{
		7: {RomID: 7, FilePath: rom},
		8: {RomID: 8, FilePath: filepath.Join(dir, "Roms", "GB", "Other.gb")},
	}

	cases := map[string]struct {
		path string
		want int
		ok   bool
	}{
		"exact":          {rom, 7, true},
		"unclean":        {filepath.Join(dir, "Roms", "GB", "..", "GB", "Tetris.gb"), 7, true},
		"through a link": {link, 7, true},
		"unknown":        {filepath.Join(dir, "Roms", "GB", "Missing.gb"), 0, false},
	}
	for name, c := range cases {
		id, ok := FindRomByPath(resolved, c.path)
		if id != c.want || ok != c.ok {
			t.Errorf("%s: got (%d, %v), want (%d, %v)", name, id, ok, c.want, c.ok)
		}
	}
}

func TestUploadsOnlyDropsEverythingElse(t *testing.T) {
	items := []SyncItem{
		{Action: ActionDownload},
		{Action: ActionUpload, TargetSlot: "a"},
		{Action: ActionSkip},
		{Action: ActionConflict},
		{Action: ActionUpload, TargetSlot: "b"},
	}

	got := UploadsOnly(items)

	if len(got) != 2 || got[0].TargetSlot != "a" || got[1].TargetSlot != "b" {
		t.Errorf("got %+v", got)
	}
}
