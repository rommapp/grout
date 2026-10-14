package cfw

import (
	"os"
	"path/filepath"
	"testing"

	"grout/gamelist"
	"grout/library"
)

func sonicIn(romDir string) []gamelist.RomGameEntry {
	return []gamelist.RomGameEntry{{
		Game:         library.Game{FileName: "Sonic.gba", DisplayName: "Sonic"},
		Platform:     library.Platform{FSSlug: "gba"},
		RomDirectory: romDir,
	}}
}

// ES-DE reads gamelists from its own tree, one folder per system, and has no
// restart flag to watch.
func TestFillGamesMetadata_RetroDECKWritesIntoESDE(t *testing.T) {
	t.Setenv(EnvVar, string(RetroDECK))
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Chdir(t.TempDir())

	romDir := filepath.Join(t.TempDir(), "gba")
	want := filepath.Join(configHome, "ES-DE", "gamelists", "gba", "gamelist.xml")
	if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
		t.Fatal(err)
	}

	FillGamesMetadata(sonicIn(romDir))

	if _, err := os.Stat(want); err != nil {
		t.Errorf("no gamelist in ES-DE's tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(romDir, "gamelist.xml")); !os.IsNotExist(err) {
		t.Error("a gamelist was also written beside the roms")
	}
	if _, err := os.Stat(esRestartFlag); !os.IsNotExist(err) {
		t.Error("RetroDECK should not be asked to restart")
	}
}

// The other EmulationStation firmwares keep the gamelist beside the roms and
// reload when the flag appears.
func TestFillGamesMetadata_BatoceraWritesBesideTheRoms(t *testing.T) {
	t.Setenv(EnvVar, string(Batocera))
	t.Chdir(t.TempDir())

	romDir := t.TempDir()
	FillGamesMetadata(sonicIn(romDir))

	if _, err := os.Stat(filepath.Join(romDir, "gamelist.xml")); err != nil {
		t.Errorf("no gamelist beside the roms: %v", err)
	}
	if _, err := os.Stat(esRestartFlag); err != nil {
		t.Errorf("Batocera was not asked to restart: %v", err)
	}
}
