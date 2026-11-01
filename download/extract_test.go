package download

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/atomic"

	"grout/cfw"
	"grout/gamelist"
	"grout/library"
	"grout/romm"
	"grout/settings"
)

// zipOf writes an archive holding the named files, each with a byte in it so
// the unpacker has something to copy.
func zipOf(t *testing.T, path string, names ...string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	w := zip.NewWriter(file)
	for _, name := range names {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("rom")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractArchive(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "Game.zip")
	zipOf(t, archivePath, "Game.sfc")

	path, err := ExtractArchive(archivePath, dir, &atomic.Float64{})
	if err != nil {
		t.Fatalf("ExtractArchive: %v", err)
	}

	if want := filepath.Join(dir, "Game.sfc"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "Game.sfc")); err != nil {
		t.Errorf("the rom was not unpacked: %v", err)
	}
	// Leaving it behind would show the game twice and waste space on the card.
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("the archive must be removed once it is unpacked")
	}
}

// Several discs need the playlist that ties them together, not whichever file
// happens to come first in the archive.
func TestExtractArchive_PrefersThePlaylist(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "FF7.zip")
	zipOf(t, archivePath, "FF7 (Disc 1).chd", "FF7 (Disc 2).chd", "FF7.m3u")

	path, err := ExtractArchive(archivePath, dir, &atomic.Float64{})
	if err != nil {
		t.Fatalf("ExtractArchive: %v", err)
	}

	if filepath.Base(path) != "FF7.m3u" {
		t.Errorf("path = %q, want the playlist", path)
	}
}

// A broken archive is not worth losing the download to: the file stays put so
// a frontend that can read archives still has something to launch.
func TestExtractArchive_KeepsTheArchiveOnFailure(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "Broken.zip")
	if err := os.WriteFile(archivePath, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ExtractArchive(archivePath, dir, &atomic.Float64{}); err == nil {
		t.Fatal("expected an error unpacking a file that is not an archive")
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Error("the archive must survive a failed unpack")
	}
}

func TestExtractMultiFile(t *testing.T) {
	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	romDirectory := filepath.Join(base, "ROMS", "psx")
	if err := os.MkdirAll(romDirectory, 0755); err != nil {
		t.Fatal(err)
	}

	game := romm.Rom{ID: 42, Name: "FF7", FsName: "FF7.zip", FsNameNoExt: "FF7", HasMultipleFiles: true}

	// The download writes the archive to the temp path the plan chose.
	archivePath := MultiFileArchivePath(game)
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		t.Fatal(err)
	}
	zipOf(t, archivePath, "FF7 (Disc 1).chd", "FF7.m3u")

	path, err := ExtractMultiFile(game, romDirectory, &atomic.Float64{})
	if err != nil {
		t.Fatalf("ExtractMultiFile: %v", err)
	}

	if filepath.Ext(path) != ".m3u" {
		t.Errorf("path = %q, want the playlist", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the path handed to the frontend does not exist: %v", err)
	}
	// The archive was only ever a staging file.
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("the downloaded archive must be removed once it is unpacked")
	}
}

// zipWithACorruptEntry writes an archive whose first file unpacks and whose
// second fails its checksum, as a truncated download would.
func zipWithACorruptEntry(t *testing.T, path string) {
	t.Helper()
	zipOf(t, path, "Game (Disc 1).bin", "Game (Disc 2).bin")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Each entry holds "rom"; corrupting the last copy breaks the second
	// entry's checksum while leaving the directory readable.
	i := bytes.LastIndex(data, []byte("rom"))
	if i < 0 {
		t.Fatal("entry contents not found")
	}
	data[i] = 'X'
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// A failed unpack must not leave a folder that passes for the whole game.
func TestExtractMultiFile_RemovesWhatAFailedUnpackLeft(t *testing.T) {
	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	t.Setenv(cfw.EnvVar, string(cfw.Spruce))

	romDirectory := filepath.Join(base, "Roms", "PS")
	if err := os.MkdirAll(romDirectory, 0755); err != nil {
		t.Fatal(err)
	}

	game := romm.Rom{ID: 43, Name: "Game", FsName: "Game.zip", FsNameNoExt: "Game", HasMultipleFiles: true}
	archivePath := MultiFileArchivePath(game)
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		t.Fatal(err)
	}
	zipWithACorruptEntry(t, archivePath)

	if _, err := ExtractMultiFile(game, romDirectory, &atomic.Float64{}); err == nil {
		t.Fatal("expected an error unpacking a corrupt archive")
	}
	if _, err := os.Stat(filepath.Join(romDirectory, "Game")); !os.IsNotExist(err) {
		t.Error("the half-unpacked folder must be removed")
	}
}

// A folder that was there before the unpack is the user's, not grout's to
// remove.
func TestExtractMultiFile_KeepsAFolderThatWasThereBefore(t *testing.T) {
	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	t.Setenv(cfw.EnvVar, string(cfw.Spruce))

	romDirectory := filepath.Join(base, "Roms", "PS")
	kept := filepath.Join(romDirectory, "Game", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(kept), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kept, []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}

	game := romm.Rom{ID: 44, Name: "Game", FsName: "Game.zip", FsNameNoExt: "Game", HasMultipleFiles: true}
	archivePath := MultiFileArchivePath(game)
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		t.Fatal(err)
	}
	zipWithACorruptEntry(t, archivePath)

	if _, err := ExtractMultiFile(game, romDirectory, &atomic.Float64{}); err == nil {
		t.Fatal("expected an error unpacking a corrupt archive")
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("a file that was there before must survive: %v", err)
	}
}

// The archive is staged outside the rom directory so a failure leaves nothing
// there for the frontend to trip over.
func TestMultiFileArchivePath_IsNotInTheRomDirectory(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	game := romm.Rom{ID: 42, Name: "FF7"}
	path := MultiFileArchivePath(game)

	if filepath.Ext(path) != ".zip" {
		t.Errorf("path = %q, want a .zip", path)
	}
	if got := MultiFileArchivePath(game); got != path {
		t.Error("the path must be the same every time, since planning and unpacking both derive it")
	}
}

func TestPlan_RomLocationAndSetGamePath(t *testing.T) {
	plan := Plan{
		Roms: []Item{
			{GameName: "Mario", Location: "/roms/snes/Mario.zip"},
			{GameName: "Zelda", Location: "/roms/snes/Zelda (USA).sfc"},
		},
	}

	if got := plan.RomLocation("Zelda"); got != "/roms/snes/Zelda (USA).sfc" {
		t.Errorf("RomLocation = %q, want the version that was actually downloaded", got)
	}
	if got := plan.RomLocation("Sonic"); got != "" {
		t.Errorf("RomLocation = %q, want empty for a game not in the plan", got)
	}
}

// Unpacking moves a rom, and the metadata entry is written afterwards, so it
// has to be told where the file ended up.
func TestPlan_SetGamePath(t *testing.T) {
	plan := Plan{Entries: []gamelist.RomGameEntry{
		{Game: library.Game{FileName: "Mario.zip", Path: "/roms/snes/Mario.zip"}},
		{Game: library.Game{FileName: "Zelda.zip", Path: "/roms/snes/Zelda.zip"}},
	}}

	plan.SetGamePath("Zelda.zip", "/roms/snes/Zelda.sfc")

	if got := plan.Entries[1].Game.Path; got != "/roms/snes/Zelda.sfc" {
		t.Errorf("path = %q, want the unpacked file", got)
	}
	if got := plan.Entries[0].Game.Path; got != "/roms/snes/Mario.zip" {
		t.Errorf("Mario moved to %q, but only Zelda was unpacked", got)
	}
}

// A game with no fs_name has its metadata entry named after its first file.
// Unpacking has to find the entry by that same name, or the entry keeps
// pointing at the archive that unpacking just deleted.
func TestPlan_SetGamePathWithoutFsName(t *testing.T) {
	game := romm.Rom{
		ID: 7, Name: "Zelda", FsNameNoExt: "Zelda",
		Files: []romm.RomFile{{ID: 1, FileName: "Zelda.zip"}},
	}
	plan, _ := BuildPlan(settings.Config{}, testHost(), testPlatform(), []romm.Rom{game}, 0)
	if len(plan.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(plan.Entries))
	}

	plan.SetGamePath(game.EntryFileName(), "/roms/nds/Zelda.nds")

	if got := plan.Entries[0].Game.Path; got != "/roms/nds/Zelda.nds" {
		t.Errorf("entry path = %q, want the unpacked file", got)
	}
}
