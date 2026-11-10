package onion

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGameCoreReadsTheChoiceKeptBesideTheRom(t *testing.T) {
	roms := t.TempDir()
	rom := filepath.Join(roms, "GBA", "Pokemon Monomyth.gba")
	writeFile(t, rom, "rom")
	writeFile(t, filepath.Join(roms, "GBA", ".game_config", "Pokemon Monomyth.cfg"), "core = \"mgba_libretro\"\n")

	if got := GameCore(rom); got != "mgba_libretro" {
		t.Errorf("GameCore = %q", got)
	}
}

func TestGameCoreIsEmptyForAGameOnItsSystemsDefault(t *testing.T) {
	roms := t.TempDir()
	rom := filepath.Join(roms, "GBA", "Other.gba")
	writeFile(t, rom, "rom")
	writeFile(t, filepath.Join(roms, "GBA", ".game_config", "Pokemon Monomyth.cfg"), "core = \"mgba_libretro\"\n")

	if got := GameCore(rom); got != "" {
		t.Errorf("GameCore = %q for a game with no choice of its own", got)
	}
}

func TestGameCoreIgnoresOtherSettingsInTheFile(t *testing.T) {
	roms := t.TempDir()
	rom := filepath.Join(roms, "GBA", "Game.zip")
	writeFile(t, filepath.Join(roms, "GBA", ".game_config", "Game.cfg"), "scale = \"2\"\ncore = \"gpsp_libretro\"\nscore = \"9\"\n")

	if got := GameCore(rom); got != "gpsp_libretro" {
		t.Errorf("GameCore = %q", got)
	}
}

func TestSaveFolderFollowsTheCoreName(t *testing.T) {
	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	rom := filepath.Join(base, "Roms", "GBA", "Pokemon Monomyth.gba")
	writeFile(t, rom, "rom")
	writeFile(t, filepath.Join(base, "Roms", "GBA", ".game_config", "Pokemon Monomyth.cfg"), "core = \"mgba_libretro\"\n")
	writeFile(t, filepath.Join(base, "RetroArch", ".retroarch", "cores", "mgba_libretro.info"),
		"display_name = \"Nintendo - Game Boy Advance (mGBA)\"\ncorename = \"mGBA\"\n")

	if got := SaveFolderForRom(rom); got != "mGBA" {
		t.Errorf("SaveFolderForRom = %q", got)
	}
}

func TestSaveFolderIsEmptyWhenTheCoreIsNotKnown(t *testing.T) {
	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	rom := filepath.Join(base, "Roms", "GBA", "Game.gba")
	writeFile(t, filepath.Join(base, "Roms", "GBA", ".game_config", "Game.cfg"), "core = \"nothing_libretro\"\n")

	if got := SaveFolderForRom(rom); got != "" {
		t.Errorf("SaveFolderForRom = %q for a core with no info file", got)
	}
}

// A card laid out the way Onion lays it out: a GBA system whose launcher
// defaults to gpSP, and the cores that can run it.
func gbaCard(t *testing.T) (base, rom string) {
	t.Helper()
	base = t.TempDir()
	t.Setenv("BASE_PATH", base)

	writeFile(t, filepath.Join(base, "Emu", "GBA", "config.json"), `{"label":"GBA","rompath":"../../Roms/GBA"}`)
	writeFile(t, filepath.Join(base, "Emu", "GBA", "launch.sh"), "#!/bin/sh\ndefault_core=gpsp\nHOME=/x ./retroarch -L .retroarch/cores/${default_core}_libretro.so \"$1\"\n")
	rom = filepath.Join(base, "Roms", "GBA", "Pokemon Monomyth.gba")
	writeFile(t, rom, "rom")

	cores := filepath.Join(base, "RetroArch", ".retroarch", "cores")
	for id, name := range map[string]string{"gpsp_libretro": "gpSP", "mgba_libretro": "mGBA", "vba_next_libretro": "VBA Next"} {
		writeFile(t, filepath.Join(cores, id+".so"), "so")
		writeFile(t, filepath.Join(cores, id+".info"), "corename = \""+name+"\"\n")
	}
	// An info file for a core that is not installed.
	writeFile(t, filepath.Join(cores, "uninstalled_libretro.info"), "corename = \"gpSP\"\n")
	return base, rom
}

func TestGameCoreChoicesOfferOnlyCoresWhoseSavesGroutKnows(t *testing.T) {
	_, rom := gbaCard(t)

	got, err := GameCoreChoices(rom, []string{"gpSP", "mGBA"})
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, c := range got.Choices {
		names = append(names, c.Name)
	}
	if len(names) != 2 || names[0] != "gpSP" || names[1] != "mGBA" {
		t.Errorf("choices = %v, want gpSP and mGBA only", names)
	}
	if got.Default != "gpsp_libretro" || got.Current != "gpsp_libretro" {
		t.Errorf("default=%q current=%q", got.Default, got.Current)
	}
}

func TestGameCoreChoicesReportTheGamesOwnCoreAsCurrent(t *testing.T) {
	_, rom := gbaCard(t)
	if err := SetGameCore(rom, "mgba_libretro"); err != nil {
		t.Fatal(err)
	}

	got, _ := GameCoreChoices(rom, []string{"gpSP", "mGBA"})
	if got.Current != "mgba_libretro" || got.Default != "gpsp_libretro" {
		t.Errorf("current=%q default=%q", got.Current, got.Default)
	}
}

func TestTheDefaultCoreIsReadTheWayOnionReadsIt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "variable", "launch.sh"), "#!/bin/sh\ndefault_core=\"mgba\"\n")
	writeFile(t, filepath.Join(dir, "command", "launch.sh"),
		"#!/bin/sh\ncd /mnt/SDCARD/RetroArch/\nHOME=/x $progdir/../../RetroArch/retroarch -v -L $progdir/../../RetroArch/.retroarch/cores/gambatte_libretro.so \"$1\"\n")

	if got := defaultCore(filepath.Join(dir, "variable")); got != "mgba_libretro" {
		t.Errorf("from a default_core variable: %q", got)
	}
	if got := defaultCore(filepath.Join(dir, "command")); got != "gambatte_libretro" {
		t.Errorf("from the command: %q", got)
	}
	if got := defaultCore(filepath.Join(dir, "none")); got != "" {
		t.Errorf("for a system with no launcher: %q", got)
	}
}

func TestChoosingACoreWritesTheGamesOwnFile(t *testing.T) {
	base, rom := gbaCard(t)

	if err := SetGameCore(rom, "mgba_libretro"); err != nil {
		t.Fatal(err)
	}

	got := read(t, filepath.Join(base, "Roms", "GBA", ".game_config", "Pokemon Monomyth.cfg"))
	if got != "core = \"mgba_libretro\"\n" {
		t.Errorf("file = %q", got)
	}
	if SaveFolderForRom(rom) != "mGBA" {
		t.Errorf("the save folder did not follow the new core: %q", SaveFolderForRom(rom))
	}
}

func TestChoosingTheSystemsDefaultTakesTheChoiceAway(t *testing.T) {
	base, rom := gbaCard(t)
	SetGameCore(rom, "mgba_libretro")

	if err := SetGameCore(rom, "gpsp_libretro"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(base, "Roms", "GBA", ".game_config")); !os.IsNotExist(err) {
		t.Error("the empty settings folder was left behind, as an override would be")
	}
	if GameCore(rom) != "" {
		t.Errorf("game still has a core of its own: %q", GameCore(rom))
	}
}

func TestChangingTheCoreKeepsTheGamesOtherSettings(t *testing.T) {
	base, rom := gbaCard(t)
	cfg := filepath.Join(base, "Roms", "GBA", ".game_config", "Pokemon Monomyth.cfg")
	writeFile(t, cfg, "scale = \"2\"\ncore = \"gpsp_libretro\"\nscore = \"9\"\n")

	if err := SetGameCore(rom, "mgba_libretro"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, cfg); got != "scale = \"2\"\nscore = \"9\"\ncore = \"mgba_libretro\"\n" {
		t.Errorf("file = %q", got)
	}

	if err := SetGameCore(rom, ""); err != nil {
		t.Fatal(err)
	}
	if got := read(t, cfg); got != "scale = \"2\"\nscore = \"9\"\n" {
		t.Errorf("after clearing = %q", got)
	}
}
