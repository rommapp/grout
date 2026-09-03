package cfw

import (
	"os"
	"path/filepath"
	"testing"

	"grout/settings"
)

func TestScanRomsByPlatform_NextUIUsesRommFSSlug(t *testing.T) {
	romRoot := t.TempDir()
	romDir := filepath.Join(romRoot, "Game Boy Advance (MGBA)")
	if err := os.Mkdir(romDir, 0755); err != nil {
		t.Fatal(err)
	}
	romName := "Final Fantasy Tactics Advance (USA).zip"
	if err := os.WriteFile(filepath.Join(romDir, romName), []byte("rom"), 0644); err != nil {
		t.Fatal(err)
	}

	// PlatformsBinding maps a RomM slug to a CFW key, so this binding makes
	// ResolveRommFSSlug("gba") answer "GBA".
	config := settings.Config{
		DirectoryMappings: map[string]settings.DirectoryMapping{
			"GBA": {RelativePath: "Game Boy Advance (MGBA)"},
		},
		PlatformsBinding: map[string]string{"GBA": "gba"},
	}
	platforms := map[string][]string{
		"gba": {"Game Boy Advance (GBA)", "Game Boy Advance (MGBA)"},
	}

	scan := scanRomsByPlatform(romRoot, platforms, config, NextUI)
	roms := scan["GBA"]
	if len(roms) != 1 {
		t.Fatalf("GBA scan count = %d, want 1 (scan=%+v)", len(roms), scan)
	}
	if roms[0].FSSlug != "GBA" || roms[0].FileName != romName {
		t.Fatalf("resolved ROM = %+v, want RomM slug GBA and file %q", roms[0], romName)
	}
	if _, exists := scan["gba"]; exists {
		t.Fatalf("scan retained CFW slug gba instead of RomM slug: %+v", scan)
	}
}
