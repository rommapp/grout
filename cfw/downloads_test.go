package cfw

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touch(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("rom"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRomLayout_DownloadPath(t *testing.T) {
	tests := []struct {
		name   string
		layout RomLayout
		romDir string
		want   string
	}{
		{
			"single file uses the first file name",
			RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}},
			"/roms/gba", "/roms/gba/Sonic.gba",
		},
		{
			"multi disc uses the playlist",
			RomLayout{BaseName: "Final Fantasy VII", MultiDisc: true, FileNames: []string{"disc1.bin", "disc2.bin"}},
			"/roms/psx", "/roms/psx/Final Fantasy VII.m3u",
		},
		{
			"several versions resolve to the first",
			RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic (USA).gba", "Sonic (Europe).gba"}},
			"/roms/gba", "/roms/gba/Sonic (USA).gba",
		},
		{
			"no files means nothing to install",
			RomLayout{BaseName: "Sonic"},
			"/roms/gba", "",
		},
		{
			"no rom directory means no path",
			RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}},
			"", "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.layout.DownloadPath(tt.romDir); got != tt.want {
				t.Errorf("DownloadPath(%q) = %q, want %q", tt.romDir, got, tt.want)
			}
		})
	}
}

func TestRomLayout_IsDownloaded_SingleFile(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}}

	if layout.IsDownloaded(dir) {
		t.Error("expected not downloaded before the file exists")
	}

	touch(t, dir, "Sonic.gba")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded once the file exists")
	}
}

// A game offering several versions counts once any one of them is on disk;
// requiring all of them would mark every such game as missing.
func TestRomLayout_IsDownloaded_AnyVersionCounts(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:  "Sonic",
		FileNames: []string{"Sonic (USA).gba", "Sonic (Europe).gba", "Sonic (Japan).gba"},
	}

	if layout.IsDownloaded(dir) {
		t.Error("expected not downloaded with no files present")
	}

	// Deliberately not the first entry.
	touch(t, dir, "Sonic (Japan).gba")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded when any version is present")
	}
}

// The launcher reads the playlist, so the discs existing is not enough.
func TestRomLayout_IsDownloaded_MultiDiscNeedsThePlaylist(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:  "Final Fantasy VII",
		MultiDisc: true,
		FileNames: []string{"disc1.bin", "disc2.bin"},
	}

	touch(t, dir, "disc1.bin")
	touch(t, dir, "disc2.bin")
	if layout.IsDownloaded(dir) {
		t.Error("discs alone must not count as downloaded without the playlist")
	}

	touch(t, dir, "Final Fantasy VII.m3u")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded once the playlist exists")
	}
}

func TestRomLayout_IsDownloaded_EmptyInputs(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Sonic.gba")

	if (RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}}).IsDownloaded("") {
		t.Error("an empty rom directory must not report downloaded")
	}
	if (RomLayout{BaseName: "Sonic"}).IsDownloaded(dir) {
		t.Error("a rom with no files must not report downloaded")
	}
}

func TestIsFileDownloaded(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Sonic (USA).gba")

	if !IsFileDownloaded(dir, "Sonic (USA).gba", nil) {
		t.Error("expected the present file to be reported downloaded")
	}
	if IsFileDownloaded(dir, "Sonic (Europe).gba", nil) {
		t.Error("expected an absent file to be reported missing")
	}
	if IsFileDownloaded("", "Sonic (USA).gba", nil) {
		t.Error("an empty rom directory must not report downloaded")
	}
	if IsFileDownloaded(dir, "", nil) {
		t.Error("an empty file name must not report downloaded")
	}
}

func TestRomLayout_LocalPath(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Sonic (Europe).gba")
	touch(t, dir, "Final Fantasy VII.m3u")

	tests := []struct {
		name   string
		layout RomLayout
		want   string
	}{
		{
			"the version that is present, not the first listed",
			RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic (USA).gba", "Sonic (Europe).gba"}},
			filepath.Join(dir, "Sonic (Europe).gba"),
		},
		{
			"multi disc resolves to the playlist",
			RomLayout{BaseName: "Final Fantasy VII", MultiDisc: true, FileNames: []string{"disc1.bin"}},
			filepath.Join(dir, "Final Fantasy VII.m3u"),
		},
		{
			"absent rom has no path",
			RomLayout{BaseName: "Tetris", FileNames: []string{"Tetris.gb"}},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.layout.LocalPath(dir); got != tt.want {
				t.Errorf("LocalPath = %q, want %q", got, tt.want)
			}
		})
	}
}

// An archive grout unpacked is deleted, so the files RomM listed inside it are
// what is on the card, whatever they are called.
func TestRomLayout_IsDownloaded_UnpackedArchiveByItsMembers(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:       "Mario",
		FileNames:      []string{"Mario.zip"},
		ArchiveMembers: map[string][]string{"Mario.zip": {"Super Mario Bros (USA).nes"}},
	}

	if layout.IsDownloaded(dir) {
		t.Fatal("nothing unpacked yet")
	}
	touch(t, dir, "Super Mario Bros (USA).nes")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded once the archive's contents are present")
	}
}

// A disc image with its audio tracks: all of it must be there, and members may
// sit in a folder inside the archive.
func TestRomLayout_IsDownloaded_EveryMemberMustBePresent(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:  "Digimon World 3",
		FileNames: []string{"Digimon World 3.7z"},
		ArchiveMembers: map[string][]string{"Digimon World 3.7z": {
			"Digimon World 3/Digimon World 3.cue",
			"Digimon World 3/Digimon World 3 (Track 1).bin",
			"Digimon World 3/Digimon World 3 (Track 2).bin",
		}},
	}

	touch(t, dir, "Digimon World 3/Digimon World 3.cue")
	touch(t, dir, "Digimon World 3/Digimon World 3 (Track 1).bin")
	if layout.IsDownloaded(dir) {
		t.Error("a missing track must not count as downloaded")
	}
	touch(t, dir, "Digimon World 3/Digimon World 3 (Track 2).bin")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded once every member is present")
	}
}

// When RomM listed the contents, a file merely sharing the archive's name is
// not taken for them.
func TestRomLayout_IsDownloaded_MembersOutrankTheName(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Mario.nes")
	layout := RomLayout{
		BaseName:       "Mario",
		FileNames:      []string{"Mario.zip"},
		ArchiveMembers: map[string][]string{"Mario.zip": {"Super Mario Bros (USA).nes"}},
	}

	if layout.IsDownloaded(dir) {
		t.Error("the listed member is missing, so the game is not downloaded")
	}
}

// A member naming a path outside the rom directory is never looked for, and
// an archive listing only such members is not downloaded.
func TestRomLayout_IsDownloaded_IgnoresMembersOutsideTheRomDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "gba")
	touch(t, parent, "Escape.gba")
	touch(t, dir, "Inside.gba")

	escaping := RomLayout{
		FileNames:      []string{"Game.zip"},
		ArchiveMembers: map[string][]string{"Game.zip": {"../Escape.gba"}},
	}
	if escaping.IsDownloaded(dir) {
		t.Error("a member outside the rom directory must not count")
	}

	mixed := RomLayout{
		FileNames:      []string{"Game.zip"},
		ArchiveMembers: map[string][]string{"Game.zip": {"../Escape.gba", "Inside.gba", "Folder/"}},
	}
	if !mixed.IsDownloaded(dir) {
		t.Error("the safe members are present, so the game is downloaded")
	}
}

// Servers that did not look inside the archive leave grout to go by its name:
// No-Intro and Redump sets pack each game under the name of its file.
func TestRomLayout_IsDownloaded_UnpackedArchiveByName(t *testing.T) {
	layout := RomLayout{BaseName: "Pokemon FireRed", FileNames: []string{"Pokemon FireRed.zip"}}

	tests := []struct {
		name    string
		present string
		want    bool
	}{
		{"the unpacked rom", "Pokemon FireRed.gba", true},
		{"a different case", "pokemon firered.GBA", true},
		{"a playlist", "Pokemon FireRed.m3u", true},
		{"a folder of the same name", "Pokemon FireRed/rom.gba", true},
		{"a save", "Pokemon FireRed.srm", false},
		{"a numbered save state", "Pokemon FireRed.state3", false},
		{"artwork", "Pokemon FireRed.png", false},
		{"a soft patch", "Pokemon FireRed.ips", false},
		{"another archive", "Pokemon FireRed.7z", false},
		{"a hidden file", ".Pokemon FireRed.gba", false},
		{"another game", "Pokemon LeafGreen.gba", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, tt.present)
			if got := layout.IsDownloaded(dir); got != tt.want {
				t.Errorf("with %q present, IsDownloaded = %v, want %v", tt.present, got, tt.want)
			}
		})
	}
}

// Only an archive grout unpacks stands in for another name. A missing rom is
// missing even when something of the same name is present.
func TestRomLayout_IsDownloaded_OnlyArchivesGoByName(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Sonic.gbc")

	if (RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}}).IsDownloaded(dir) {
		t.Error("Sonic.gbc is not Sonic.gba")
	}
	if !(RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.7z"}}).IsDownloaded(dir) {
		t.Error("expected the unpacked 7z to be found by its name")
	}
}

// A multi-file game is unpacked into a folder named after it. Unless the
// firmware lifts the playlist out, the folder is all there is to find.
func TestRomLayout_IsDownloaded_MultiDiscUnpackedIntoItsFolder(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:  "Final Fantasy IX",
		MultiDisc: true,
		FileNames: []string{"FF9 (Disc 1).chd", "FF9 (Disc 2).chd"},
	}

	if err := os.MkdirAll(filepath.Join(dir, "Final Fantasy IX"), 0755); err != nil {
		t.Fatal(err)
	}
	if layout.IsDownloaded(dir) {
		t.Error("an empty folder must not count as downloaded")
	}

	touch(t, dir, "Final Fantasy IX/FF9 (Disc 1).chd")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded once the folder holds the game")
	}
}

// muOS hides the folder behind an underscore; the plain name is not looked at.
func TestRomLayout_IsDownloaded_MultiDiscInTheFirmwaresFolder(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:     "Digimon World 3",
		MultiDisc:    true,
		ExtractedDir: "_Digimon World 3",
	}

	touch(t, dir, "Digimon World 3/Digimon World 3.cue")
	if layout.IsDownloaded(dir) {
		t.Error("the firmware does not keep the game under its plain name")
	}

	touch(t, dir, "_Digimon World 3/Digimon World 3.cue")
	if !layout.IsDownloaded(dir) {
		t.Error("expected downloaded from the firmware's folder")
	}
}

func TestIsFileDownloaded_UnpackedArchive(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Super Mario Bros (USA).nes")

	if !IsFileDownloaded(dir, "Mario.zip", []string{"Super Mario Bros (USA).nes"}) {
		t.Error("expected the archive's contents to stand in for it")
	}
	if IsFileDownloaded(dir, "Mario.zip", nil) {
		t.Error("without its contents listed, the archive goes by its own name")
	}
}

// A directory listing is reused while the directory is unchanged, and read
// again once something lands in it.
func TestDirListings_ReuseUntilTheDirectoryChanges(t *testing.T) {
	dir := t.TempDir()
	listings := &dirListings{entries: map[string]dirListing{}}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}

	if listings.stems(dir)["game"] != "" {
		t.Fatal("the directory is empty")
	}

	// A change that leaves the modification time alone goes unseen: the
	// listing is the cached one.
	touch(t, dir, "Game.gba")
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	if listings.stems(dir)["game"] != "" {
		t.Error("expected the cached listing while the directory looks unchanged")
	}

	now := time.Now()
	if err := os.Chtimes(dir, now, now); err != nil {
		t.Fatal(err)
	}
	if listings.stems(dir)["game"] == "" {
		t.Error("expected a fresh listing once the directory changed")
	}
}

// A listing taken in the same tick as the directory's last change cannot be
// trusted: a second change in that tick would not move the time.
func TestDirListings_DoNotTrustAListingFromTheSameTick(t *testing.T) {
	dir := t.TempDir()
	listings := &dirListings{entries: map[string]dirListing{}}
	now := time.Now()
	if err := os.Chtimes(dir, now, now); err != nil {
		t.Fatal(err)
	}

	listings.stems(dir)
	touch(t, dir, "Game.gba")
	if err := os.Chtimes(dir, now, now); err != nil {
		t.Fatal(err)
	}
	if listings.stems(dir)["game"] == "" {
		t.Error("expected the directory to be read again")
	}
}

// The path handed to the frontend for an unpacked game is the file to launch,
// as unpacking itself would have chosen it.
func TestRomLayout_LocalPath_UnpackedGames(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "FF9/FF9 (Disc 1).chd")
	touch(t, dir, "FF9/FF9.m3u")
	touch(t, dir, "Pokemon FireRed.gba")
	touch(t, dir, "Pokemon FireRed.srm")
	touch(t, dir, "_Digimon/Digimon.cue")
	touch(t, dir, "Digimon/Digimon.bin")
	touch(t, dir, "Crash/Disc A/crash.bin")
	touch(t, dir, "Spyro.cue")
	touch(t, dir, "Spyro.m3u")

	tests := []struct {
		name   string
		layout RomLayout
		want   string
	}{
		{
			"listed members resolve to their playlist",
			RomLayout{FileNames: []string{"FF9.zip"}, ArchiveMembers: map[string][]string{
				"FF9.zip": {"FF9/FF9 (Disc 1).chd", "FF9/FF9.m3u"},
			}},
			filepath.Join(dir, "FF9", "FF9.m3u"),
		},
		{
			"without a playlist, the first listed member",
			RomLayout{FileNames: []string{"FF9.zip"}, ArchiveMembers: map[string][]string{
				"FF9.zip": {"FF9/FF9 (Disc 1).chd"},
			}},
			filepath.Join(dir, "FF9", "FF9 (Disc 1).chd"),
		},
		{
			"by name, the rom rather than its save",
			RomLayout{FileNames: []string{"Pokemon FireRed.zip"}},
			filepath.Join(dir, "Pokemon FireRed.gba"),
		},
		{
			"by name, the playlist over a file sharing it",
			RomLayout{FileNames: []string{"Spyro.7z"}},
			filepath.Join(dir, "Spyro.m3u"),
		},
		{
			"multi-file folder resolves to its playlist",
			RomLayout{BaseName: "FF9", MultiDisc: true},
			filepath.Join(dir, "FF9", "FF9.m3u"),
		},
		{
			"multi-file folder without a playlist, its first file",
			RomLayout{BaseName: "Digimon", MultiDisc: true, ExtractedDir: "_Digimon"},
			filepath.Join(dir, "_Digimon", "Digimon.cue"),
		},
		{
			"multi-file folder holding only folders, the folder itself",
			RomLayout{BaseName: "Crash", MultiDisc: true},
			filepath.Join(dir, "Crash"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.layout.LocalPath(dir); got != tt.want {
				t.Errorf("LocalPath = %q, want %q", got, tt.want)
			}
		})
	}
}
