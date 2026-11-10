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
