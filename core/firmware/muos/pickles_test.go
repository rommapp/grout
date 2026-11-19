package muos

import (
	"path"
	"slices"
	"testing"
)

const gbaConfig = "/opt/muos/share/info/content/gba/core.cfg"

func TestPicklesFoldersAssignedToPickles(t *testing.T) {
	h := mockHost(t)
	put(h, gbaConfig, "gpsp_libretro.so\nNintendo Game Boy Advance\nNintendo Game Boy Advance\n0\nmu-gpsp")

	folders, prefer := PicklesFolders(h, "gba")
	if !prefer {
		t.Error("a folder on Pickles should prefer the Pickles folder")
	}
	if len(folders) == 0 || folders[0] != "../pickles/sram/gpsp/gba" {
		t.Fatalf("folders = %v, want ../pickles/sram/gpsp/gba first", folders)
	}
	for _, want := range []string{"../pickles/sram/mgba/gba", "../pickles/sram/mednafen_gba/gba"} {
		if !slices.Contains(folders, want) {
			t.Errorf("folders = %v, missing %s", folders, want)
		}
	}
}

func TestPicklesFoldersAssignedToRetroArch(t *testing.T) {
	h := mockHost(t)
	put(h, gbaConfig, "mgba_libretro.so\nNintendo Game Boy Advance\nNintendo Game Boy Advance\n0\nmgba")

	folders, prefer := PicklesFolders(h, "gba")
	if prefer {
		t.Error("a folder on RetroArch should not prefer Pickles")
	}
	if !slices.Contains(folders, "../pickles/sram/mgba/gba") {
		t.Errorf("folders = %v, want the Pickles folders still listed", folders)
	}
}

func TestPicklesFoldersTagIsCaseInsensitive(t *testing.T) {
	h := mockHost(t)
	put(h, gbaConfig, "gpsp_libretro.so\nNintendo Game Boy Advance\nNintendo Game Boy Advance\n0\nMU-gpsp")

	if _, prefer := PicklesFolders(h, "gba"); !prefer {
		t.Error("an upper case mu- tag is still Pickles")
	}
}

func TestPicklesFoldersUnassignedFolderDefaultsToPickles(t *testing.T) {
	h := mockHost(t)

	folders, prefer := PicklesFolders(h, "Game Boy-Advance")
	if !prefer || len(folders) == 0 || folders[0] != "../pickles/sram/mgba/Game Boy-Advance" {
		t.Errorf("folders = %v prefer=%v, want the default core under Pickles", folders, prefer)
	}
}

func TestPicklesFoldersUnassignedExternalSystem(t *testing.T) {
	h := mockHost(t)

	folders, prefer := PicklesFolders(h, "nds")
	if prefer {
		t.Error("a system whose default runtime is external should not prefer Pickles")
	}
	want := []string{"../pickles/sram/desmume2015/nds", "../pickles/sram/melonds/nds", "../pickles/sram/melondsds/nds"}
	if !slices.Equal(folders, want) {
		t.Errorf("folders = %v, want %v", folders, want)
	}
}

func TestPicklesFoldersShortConfigIsUnassigned(t *testing.T) {
	h := mockHost(t)
	put(h, gbaConfig, "gpsp_libretro.so\nNintendo Game Boy Advance\n")

	folders, prefer := PicklesFolders(h, "gba")
	if !prefer || len(folders) == 0 || folders[0] != "../pickles/sram/mgba/gba" {
		t.Errorf("folders = %v prefer=%v, want the default core", folders, prefer)
	}
}

func TestPicklesFoldersUserManifestWins(t *testing.T) {
	h := mockHost(t)
	remove(h, gbaConfig)
	put(h, "/run/muos/storage/info/manifest/core.json",
		`{"Nintendo Game Boy Advance": {"default": "gpsp", "libretro": {"gpsp": {"core": "gpsp_libretro.so"}}}}`)

	folders, _ := PicklesFolders(h, "gba")
	if !slices.Equal(folders, []string{"../pickles/sram/gpsp/gba"}) {
		t.Errorf("folders = %v, want only the user's default", folders)
	}
}

func TestPicklesFoldersBrokenUserManifestFallsBack(t *testing.T) {
	h := mockHost(t)
	remove(h, gbaConfig)
	put(h, "/run/muos/storage/info/manifest/core.json", "{not json")

	folders, prefer := PicklesFolders(h, "gba")
	if !prefer || len(folders) == 0 || folders[0] != "../pickles/sram/mgba/gba" {
		t.Errorf("folders = %v prefer=%v, want the bundled manifest", folders, prefer)
	}
}

func TestPicklesFoldersOlderMuOS(t *testing.T) {
	folders, prefer := PicklesFolders(emptyHost(), "gba")
	if len(folders) != 0 || prefer {
		t.Errorf("folders = %v prefer=%v, want nothing on a muOS without Pickles", folders, prefer)
	}
}

func TestPicklesFoldersUnknownFolder(t *testing.T) {
	folders, prefer := PicklesFolders(mockHost(t), "Unsorted")
	if len(folders) != 0 || prefer {
		t.Errorf("folders = %v prefer=%v, want nothing for a folder muOS cannot assign", folders, prefer)
	}
}

func TestPicklesFoldersMockFixture(t *testing.T) {
	folders, prefer := PicklesFolders(mockHost(t), "gba")
	if !prefer || len(folders) == 0 {
		t.Fatalf("folders = %v prefer=%v, want the mock's Pickles assignment", folders, prefer)
	}
	if got, want := path.Join("/run/muos/storage/save", folders[0]), "/run/muos/storage/pickles/sram/mgba/gba"; got != want {
		t.Errorf("save path = %q, want %q", got, want)
	}
}
