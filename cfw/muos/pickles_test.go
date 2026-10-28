package muos

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// gbaManifest is the part of muOS's libretro.json that GBA needs.
const gbaManifest = `{
  "Nintendo Game Boy Advance": {
    "default": "mgba",
    "cores": {
      "beetle gba": {"core": "mednafen_gba_libretro.so"},
      "gpsp": {"core": "gpsp_libretro.so"},
      "mgba": {"core": "mgba_libretro.so"}
    }
  }
}`

func write(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// andromeda lays out the manifests a muOS with Pickles ships.
func andromeda(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, bundledManifestDir+"/libretro.json", gbaManifest)
	write(t, root, userManifestDir+"/assign.json", `{"gba": "Nintendo Game Boy Advance", "gameboyadvance": "Nintendo Game Boy Advance"}`)
	return root
}

// A folder muOS has put on Pickles saves under Pickles' own tree, named for the
// core file and the rom folder, and that folder comes first.
func TestPicklesSaveFolders_AssignedToPickles(t *testing.T) {
	root := andromeda(t)
	write(t, root, contentConfigDir+"/gba/core.cfg",
		"gpsp_libretro.so\nNintendo Game Boy Advance\nNintendo Game Boy Advance\n0\nmu-gpsp")

	folders, prefer := picklesSaveFolders(root, "gba")
	if !prefer {
		t.Error("a folder on Pickles should prefer the Pickles folder")
	}
	if len(folders) == 0 || folders[0] != "../pickles/sram/gpsp/gba" {
		t.Fatalf("folders = %v, want ../pickles/sram/gpsp/gba first", folders)
	}
	// Saves made under the other cores are still found.
	for _, want := range []string{"../pickles/sram/mgba/gba", "../pickles/sram/mednafen_gba/gba"} {
		if !slices.Contains(folders, want) {
			t.Errorf("folders = %v, missing %s", folders, want)
		}
	}
}

// A folder on RetroArch keeps RetroArch's folders first, but Pickles saves
// made before the switch are still scanned.
func TestPicklesSaveFolders_AssignedToRetroArch(t *testing.T) {
	root := andromeda(t)
	write(t, root, contentConfigDir+"/gba/core.cfg",
		"mgba_libretro.so\nNintendo Game Boy Advance\nNintendo Game Boy Advance\n0\nmgba")

	folders, prefer := picklesSaveFolders(root, "gba")
	if prefer {
		t.Error("a folder on RetroArch should not prefer Pickles")
	}
	if !slices.Contains(folders, "../pickles/sram/mgba/gba") {
		t.Errorf("folders = %v, want the Pickles folders still listed", folders)
	}
}

// muOS assigns a folder on first sight, trying Pickles first with the system's
// default core. A folder it has not seen yet is treated the same way, found by
// muOS's own normalised name.
func TestPicklesSaveFolders_UnassignedFolderDefaultsToPickles(t *testing.T) {
	root := andromeda(t)

	folders, prefer := picklesSaveFolders(root, "Game Boy-Advance")
	if !prefer || len(folders) == 0 || folders[0] != "../pickles/sram/mgba/Game Boy-Advance" {
		t.Errorf("folders = %v prefer=%v, want the default core under Pickles", folders, prefer)
	}
}

// A user's own manifest wins over the bundled one, as it does for muOS.
func TestPicklesSaveFolders_UserManifestWins(t *testing.T) {
	root := andromeda(t)
	write(t, root, userManifestDir+"/libretro.json",
		`{"Nintendo Game Boy Advance": {"default": "gpsp", "cores": {"gpsp": {"core": "gpsp_libretro.so"}}}}`)

	folders, _ := picklesSaveFolders(root, "gba")
	if len(folders) == 0 || folders[0] != "../pickles/sram/gpsp/gba" {
		t.Errorf("folders = %v, want the user's default", folders)
	}
}

// An older muOS has no manifests and no Pickles, and nothing changes there.
func TestPicklesSaveFolders_OlderMuOS(t *testing.T) {
	folders, prefer := picklesSaveFolders(t.TempDir(), "gba")
	if len(folders) != 0 || prefer {
		t.Errorf("folders = %v prefer=%v, want nothing on a muOS without Pickles", folders, prefer)
	}
}

func TestPicklesSaveFolders_BesideTheSaveRoot(t *testing.T) {
	root := andromeda(t)

	folders, _ := picklesSaveFolders(root, "gba")
	if len(folders) == 0 {
		t.Fatal("no folders")
	}
	got := filepath.Join(StoragePath, "save", folders[0])
	if want := StoragePath + "/pickles/sram/mgba/gba"; got != want {
		t.Errorf("save path = %q, want %q", got, want)
	}
}
