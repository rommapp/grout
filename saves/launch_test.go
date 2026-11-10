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

	got, held := ForLaunch(items, 1)

	if held != 2 {
		t.Errorf("held %d items for an answer, want 2 (the conflict and the slot choice)", held)
	}

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
	if got, held := ForLaunch(items, 1); len(got) != 0 || held != 0 {
		t.Errorf("got %+v, held %d", got, held)
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

func TestOnlyRomKeepsJustTheGameBeingLaunched(t *testing.T) {
	dir := t.TempDir()
	tetris := filepath.Join(dir, "GB", "Tetris.gb")
	scan := cfw.LocalRomScan{
		"gb":  {{FilePath: tetris}, {FilePath: filepath.Join(dir, "GB", "Zelda.gb")}},
		"gba": {{FilePath: filepath.Join(dir, "GBA", "Tetris.gba")}},
	}

	got := onlyRom(scan, filepath.Join(dir, "GB", "..", "GB", "Tetris.gb"))

	if len(got) != 1 || len(got["gb"]) != 1 || got["gb"][0].FilePath != tetris {
		t.Errorf("got %+v", got)
	}
	if len(onlyRom(scan, filepath.Join(dir, "GB", "Missing.gb"))) != 0 {
		t.Error("matched a game that is not there")
	}
}

// A game moved to its own core (Y menu > Game core) reads that core's save
// folder, not the one the rest of its system uses, so a save downloaded for it
// has to go there.
func TestASavePutsItselfWhereTheGamesOwnCoreReadsIt(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CFW", "ONION")
	t.Setenv("BASE_PATH", base)

	rom := filepath.Join(base, "Roms", "GBA", "Pokemon Monomyth.gba")
	other := filepath.Join(base, "Roms", "GBA", "Other.gba")
	for _, path := range []string{
		rom, other,
		filepath.Join(base, "Roms", "GBA", ".game_config", "Pokemon Monomyth.cfg"),
		filepath.Join(base, "RetroArch", ".retroarch", "cores", "mgba_libretro.info"),
	} {
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte("x"), 0644)
	}
	os.WriteFile(filepath.Join(base, "Roms", "GBA", ".game_config", "Pokemon Monomyth.cfg"), []byte("core = \"mgba_libretro\"\n"), 0644)
	os.WriteFile(filepath.Join(base, "RetroArch", ".retroarch", "cores", "mgba_libretro.info"), []byte("corename = \"mGBA\"\n"), 0644)

	saves := filepath.Join(base, "Saves", "CurrentProfile", "saves")

	if got := saveDirectoryForCore("gba", rom, nil); got != filepath.Join(saves, "mGBA") {
		t.Errorf("game on its own core: %q", got)
	}
	if got := saveDirectoryForCore("gba", other, nil); got != "" {
		t.Errorf("game on the default core was given %q", got)
	}

	discovered := resolveDiscoveredSaveDirectory(cfw.LocalRomFile{FSSlug: "gba", FilePath: rom}, nil)
	if discovered != filepath.Join(saves, "mGBA") {
		t.Errorf("save discovered for a game on its own core goes to %q", discovered)
	}
}

func TestACoreThePlatformHasNoSaveFolderForIsIgnored(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CFW", "ONION")
	t.Setenv("BASE_PATH", base)

	rom := filepath.Join(base, "Roms", "GBA", "Game.gba")
	os.MkdirAll(filepath.Join(base, "Roms", "GBA", ".game_config"), 0755)
	os.MkdirAll(filepath.Join(base, "RetroArch", ".retroarch", "cores"), 0755)
	os.WriteFile(filepath.Join(base, "Roms", "GBA", ".game_config", "Game.cfg"), []byte("core = \"vba_libretro\"\n"), 0644)
	os.WriteFile(filepath.Join(base, "RetroArch", ".retroarch", "cores", "vba_libretro.info"), []byte("corename = \"VBA Next\"\n"), 0644)

	if got := saveDirectoryForCore("gba", rom, nil); got != "" {
		t.Errorf("a save was sent to %q, a folder nothing Grout knows reads", got)
	}
}
