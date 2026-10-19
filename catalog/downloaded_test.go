package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"grout/cfw"
	"grout/romm"
	"grout/settings"
)

// muosRomDir points the firmware at a fresh muOS card and returns its rom
// directory for platform.
func muosRomDir(t *testing.T, platform string) string {
	t.Helper()

	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	dir := filepath.Join(base, "ROMS", platform)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func place(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
}

// Issue #233: a zipped rom unpacked on download no longer has its archive, so
// it is found through what RomM listed inside it.
func TestIsDownloaded_UnpackedArchive(t *testing.T) {
	dir := muosRomDir(t, "nes")
	game := romm.Rom{
		ID: 1, Name: "Mario", PlatformFSSlug: "nes", FsNameNoExt: "Mario",
		Files: []romm.RomFile{{
			FileName:       "Mario.zip",
			ArchiveMembers: []romm.RomArchiveMember{{Name: "Super Mario Bros (USA).nes"}},
		}},
	}

	if IsDownloaded(settings.Config{}, game) {
		t.Fatal("nothing is on the card yet")
	}
	place(t, dir, "Super Mario Bros (USA).nes")
	if !IsDownloaded(settings.Config{}, game) {
		t.Error("expected the unpacked rom to count as the game")
	}
	if DownloadStateOf(settings.Config{}, game) != FullyDownloaded {
		t.Error("expected the game listed as downloaded")
	}
}

// muOS moves a multi-file game without a playlist into an underscored folder.
func TestIsDownloaded_MultiFileGameInMuOSFolder(t *testing.T) {
	dir := muosRomDir(t, "psx")
	game := romm.Rom{
		ID: 2, Name: "Digimon World 3", PlatformFSSlug: "psx", FsNameNoExt: "Digimon World 3",
		HasMultipleFiles: true,
		Files: []romm.RomFile{
			{FileName: "Digimon World 3.cue"},
			{FileName: "Digimon World 3.bin"},
		},
	}

	if IsDownloaded(settings.Config{}, game) {
		t.Fatal("nothing is on the card yet")
	}
	place(t, dir, "_Digimon World 3/Digimon World 3.cue")
	if !IsDownloaded(settings.Config{}, game) {
		t.Error("expected the game found in muOS's folder")
	}
}

// One version of a game, picked from several and unpacked, is found through
// that version's archive contents.
func TestIsFileDownloaded_UnpackedVersion(t *testing.T) {
	dir := muosRomDir(t, "gba")
	game := romm.Rom{
		ID: 3, Name: "Sonic", PlatformFSSlug: "gba", FsNameNoExt: "Sonic",
		HasNestedSingleFile: true,
		Files: []romm.RomFile{
			{FileName: "Sonic (USA).zip", ArchiveMembers: []romm.RomArchiveMember{{Name: "Sonic Advance (USA).gba"}}},
			{FileName: "Sonic (Japan).zip", ArchiveMembers: []romm.RomArchiveMember{{Name: "Sonic Advance (Japan).gba"}}},
		},
	}

	place(t, dir, "Sonic Advance (USA).gba")
	if !IsFileDownloaded(settings.Config{}, game, "Sonic (USA).zip") {
		t.Error("expected the unpacked USA version found")
	}
	if IsFileDownloaded(settings.Config{}, game, "Sonic (Japan).zip") {
		t.Error("the Japanese version is not on the card")
	}
	if DownloadStateOf(settings.Config{}, game) != PartlyDownloaded {
		t.Error("expected one version of two to read as partly downloaded")
	}
}

// A single zip holding a folder of discs and their playlist, as RomM 5.3.1
// reports it (hashes trimmed). Unpacking keeps the folder.
func TestIsDownloaded_UnpackedArchiveWithAFolderOfDiscs(t *testing.T) {
	dir := muosRomDir(t, "psx")
	var game romm.Rom
	body := `{
		"id": 2, "name": "Final Fantasy IX", "platform_fs_slug": "psx",
		"fs_name": "Final Fantasy IX (France).zip", "fs_name_no_ext": "Final Fantasy IX (France)",
		"has_simple_single_file": true, "has_multiple_files": false,
		"files": [{
			"id": 5, "file_name": "Final Fantasy IX (France).zip", "category": "game",
			"archive_members": [
				{"name": "Final Fantasy IX (France)/Final Fantasy IX (France) (Disc 1).chd", "size": 432010579, "crc_hash": "074cbe4f"},
				{"name": "Final Fantasy IX (France)/Final Fantasy IX (France) (Disc 2).chd", "size": 350437002, "crc_hash": "f0d09a35"},
				{"name": "Final Fantasy IX (France)/Final Fantasy IX (France) (Disc 3).chd", "size": 375922076, "crc_hash": "ada836cb"},
				{"name": "Final Fantasy IX (France)/Final Fantasy IX (France) (Disc 4).chd", "size": 396856005, "crc_hash": "e4e8d49d"},
				{"name": "Final Fantasy IX (France)/Final Fantasy IX (France).m3u", "size": 156, "crc_hash": "8bd1eefa"}
			]
		}]
	}`
	if err := json.Unmarshal([]byte(body), &game); err != nil {
		t.Fatal(err)
	}

	for _, disc := range []string{"Disc 1", "Disc 2", "Disc 3"} {
		place(t, dir, "Final Fantasy IX (France)/Final Fantasy IX (France) ("+disc+").chd")
	}
	place(t, dir, "Final Fantasy IX (France)/Final Fantasy IX (France).m3u")
	if IsDownloaded(settings.Config{}, game) {
		t.Error("disc 4 is missing, so the game is not downloaded")
	}

	place(t, dir, "Final Fantasy IX (France)/Final Fantasy IX (France) (Disc 4).chd")
	if !IsDownloaded(settings.Config{}, game) {
		t.Error("expected downloaded once every disc and the playlist are unpacked")
	}
}
