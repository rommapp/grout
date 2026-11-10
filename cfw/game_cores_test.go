package cfw

import (
	"os"
	"path/filepath"
	"testing"

	"grout/settings"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// An Onion card with a GBA system that defaults to gpSP, and a GB system with
// only one core that keeps saves where Grout looks.
func onionCard(t *testing.T) (gba, gb string) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("BASE_PATH", base)

	put(t, filepath.Join(base, "Emu", "GBA", "config.json"), `{"rompath":"../../Roms/GBA"}`)
	put(t, filepath.Join(base, "Emu", "GBA", "launch.sh"), "default_core=gpsp\n")
	put(t, filepath.Join(base, "Emu", "GB", "config.json"), `{"rompath":"../../Roms/GB"}`)
	put(t, filepath.Join(base, "Emu", "GB", "launch.sh"), "./retroarch -L .retroarch/cores/gambatte_libretro.so \"$1\"\n")

	cores := filepath.Join(base, "RetroArch", ".retroarch", "cores")
	for id, name := range map[string]string{"gpsp_libretro": "gpSP", "mgba_libretro": "mGBA", "gambatte_libretro": "Gambatte"} {
		put(t, filepath.Join(cores, id+".so"), "so")
		put(t, filepath.Join(cores, id+".info"), "corename = \""+name+"\"\n")
	}

	gba = filepath.Join(base, "Roms", "GBA", "Game.gba")
	gb = filepath.Join(base, "Roms", "GB", "Game.gb")
	put(t, gba, "rom")
	put(t, gb, "rom")
	return gba, gb
}

func TestOnionOffersAGameACoreWhereSeveralKeepSavesGroutSyncs(t *testing.T) {
	gba, _ := onionCard(t)

	cores, ok := Lookup(Onion).GameCores(settings.Config{}, "gba", gba)
	if !ok {
		t.Fatal("no choice offered for a GBA game")
	}
	if len(cores.Choices) != 2 || cores.Default != "gpsp_libretro" || cores.Current != "gpsp_libretro" {
		t.Errorf("got %+v", cores)
	}
}

func TestNothingIsOfferedWhereThereIsNothingToChooseBetween(t *testing.T) {
	_, gb := onionCard(t)

	if _, ok := Lookup(Onion).GameCores(settings.Config{}, "gb", gb); ok {
		t.Error("offered a choice of one")
	}
}

func TestFirmwaresThatCannotChangeAGamesCoreOfferNothing(t *testing.T) {
	gba, _ := onionCard(t)

	for _, id := range []CFW{MinUI, NextUI, MuOS, Knulli} {
		if _, ok := Lookup(id).GameCores(settings.Config{}, "gba", gba); ok {
			t.Errorf("%s offered a game core", id)
		}
		if err := Lookup(id).SetGameCore(gba, "mgba_libretro"); err != nil {
			t.Errorf("%s: setting a core should do nothing, got %v", id, err)
		}
	}
	if _, ok := Lookup(Onion).GameCores(settings.Config{}, "gba", ""); ok {
		t.Error("offered a core for a game that is not on the device")
	}
}

// Only a firmware that lets one game change core has saves in folders per core,
// which is what makes a save in two folders something to settle.
func TestOnlyFirmwaresThatKeepSavesPerCoreTreatTwoFoldersAsADoubt(t *testing.T) {
	if !Lookup(Onion).KeepsSavesPerCore() {
		t.Error("Onion keeps saves per core")
	}
	for _, id := range []CFW{MinUI, NextUI, MuOS, Knulli, Spruce, Allium} {
		if Lookup(id).KeepsSavesPerCore() {
			t.Errorf("%s was treated as keeping saves per core", id)
		}
	}
}
