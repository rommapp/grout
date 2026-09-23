package cfw

import (
	"os"
	"path/filepath"
	"testing"
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

	if !IsFileDownloaded(dir, "Sonic (USA).gba") {
		t.Error("expected the present file to be reported downloaded")
	}
	if IsFileDownloaded(dir, "Sonic (Europe).gba") {
		t.Error("expected an absent file to be reported missing")
	}
	if IsFileDownloaded("", "Sonic (USA).gba") {
		t.Error("an empty rom directory must not report downloaded")
	}
	if IsFileDownloaded(dir, "") {
		t.Error("an empty file name must not report downloaded")
	}
}
