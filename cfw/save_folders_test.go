package cfw

import (
	"slices"
	"testing"

	"grout/settings"
)

// picklesFirmware stands in for muOS with a GBA folder on Pickles, or on
// RetroArch when onPickles is false.
func picklesFirmware(onPickles bool) *Firmware {
	return &Firmware{
		saveDirectories: map[string][]string{"gba": {"file/gpSP", "file/mGBA"}},
		extraSaveFolders: func(romFolder string) ([]string, bool) {
			return []string{"../pickles/sram/mgba/" + romFolder, "../pickles/sram/gpsp/" + romFolder}, onPickles
		},
	}
}

func mappedGBA() settings.Config {
	return settings.Config{DirectoryMappings: map[string]settings.DirectoryMapping{
		"gba": {RomMSlug: "gba", RelativePath: "Game Boy Advance"},
	}}
}

// On Pickles its folders lead, named for the mapped rom folder, and
// RetroArch's are still listed so older saves are found.
func TestSaveFolders_PicklesFirst(t *testing.T) {
	got := picklesFirmware(true).SaveFolders(mappedGBA(), "gba")
	want := []string{"../pickles/sram/mgba/Game Boy Advance", "../pickles/sram/gpsp/Game Boy Advance", "file/gpSP", "file/mGBA"}
	if !slices.Equal(got, want) {
		t.Errorf("folders = %v, want %v", got, want)
	}
}

func TestSaveFolders_RetroArchFirst(t *testing.T) {
	got := picklesFirmware(false).SaveFolders(mappedGBA(), "gba")
	if len(got) != 4 || got[0] != "file/gpSP" {
		t.Errorf("folders = %v, want RetroArch's first", got)
	}
}

// When muOS runs a folder under Pickles, a save mapped to a RetroArch folder
// would never load, so Pickles wins. A mapping to another Pickles core stays.
func TestPreferredSaveFolder(t *testing.T) {
	config := mappedGBA()
	config.SaveDirectoryMappings = map[string]string{"gba": "file/mGBA"}
	if got := picklesFirmware(true).PreferredSaveFolder(config, "gba"); got != "../pickles/sram/mgba/Game Boy Advance" {
		t.Errorf("RetroArch mapping on Pickles: %q", got)
	}

	config.SaveDirectoryMappings["gba"] = "../pickles/sram/gpsp/Game Boy Advance"
	if got := picklesFirmware(true).PreferredSaveFolder(config, "gba"); got != "../pickles/sram/gpsp/Game Boy Advance" {
		t.Errorf("Pickles mapping on Pickles: %q", got)
	}

	if got := picklesFirmware(false).PreferredSaveFolder(config, "gba"); got != "" {
		t.Errorf("on RetroArch the mapping decides, got %q", got)
	}
	if got := Lookup(Knulli).PreferredSaveFolder(config, "gba"); got != "" {
		t.Errorf("a firmware without Pickles has no opinion, got %q", got)
	}
}
