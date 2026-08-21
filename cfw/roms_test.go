package cfw

import (
	"os"
	"path/filepath"
	"testing"
)

type romScanConfigStub struct {
	directoryMappings map[string]string
	rommSlugs         map[string]string
}

func (c romScanConfigStub) GetDirectoryMapping(fsSlug string) (string, bool) {
	path, ok := c.directoryMappings[fsSlug]
	return path, ok
}

func (c romScanConfigStub) ResolveRommFSSlug(cfwKey string) string {
	if slug, ok := c.rommSlugs[cfwKey]; ok {
		return slug
	}
	return cfwKey
}

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

	config := romScanConfigStub{
		directoryMappings: map[string]string{"GBA": "Game Boy Advance (MGBA)"},
		rommSlugs:         map[string]string{"gba": "GBA"},
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
