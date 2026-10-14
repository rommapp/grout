package gamelist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Without a resolver the gamelist sits beside the roms, where every
// EmulationStation firmware but RetroDECK reads it.
func TestAddRomGamesToGamelist_DefaultsBesideTheRoms(t *testing.T) {
	romDir := t.TempDir()
	e := entry(rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba"), romDir)

	if err := AddRomGamesToGamelist([]RomGameEntry{e}, GameListFileName, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(romDir, string(GameListFileName)))
	if err != nil {
		t.Fatalf("no gamelist beside the roms: %v", err)
	}
	if !strings.Contains(string(data), "Sonic the Hedgehog") {
		t.Errorf("gamelist does not mention the game:\n%s", data)
	}
}

// ES-DE keeps gamelists in a tree of its own, so the resolver decides where the
// file goes and nothing is written beside the roms.
func TestAddRomGamesToGamelist_ResolverChoosesTheFile(t *testing.T) {
	romDir := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), string(GameListFileName))
	e := entry(rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba"), romDir)

	options := &AddRomGamesToGamelistOptions{
		PathResolver: func(RomGameEntry, FileName) string { return elsewhere },
	}
	if err := AddRomGamesToGamelist([]RomGameEntry{e}, GameListFileName, options); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(elsewhere); err != nil {
		t.Errorf("gamelist not written where the resolver said: %v", err)
	}
	if _, err := os.Stat(filepath.Join(romDir, string(GameListFileName))); !os.IsNotExist(err) {
		t.Errorf("a gamelist was also written beside the roms")
	}
}
