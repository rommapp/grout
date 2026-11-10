package saves

import (
	"os"
	"path/filepath"
	"testing"

	"grout/cfw"
)

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
