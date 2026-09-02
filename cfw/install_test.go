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

func TestRomLayout_InstalledPath(t *testing.T) {
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
			if got := tt.layout.InstalledPath(tt.romDir); got != tt.want {
				t.Errorf("InstalledPath(%q) = %q, want %q", tt.romDir, got, tt.want)
			}
		})
	}
}

func TestRomLayout_IsInstalled_SingleFile(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}}

	if layout.IsInstalled(dir) {
		t.Error("expected not installed before the file exists")
	}

	touch(t, dir, "Sonic.gba")
	if !layout.IsInstalled(dir) {
		t.Error("expected installed once the file exists")
	}
}

// A game offering several versions is installed once any one of them is on
// disk; requiring all of them would mark every such game as missing.
func TestRomLayout_IsInstalled_AnyVersionCounts(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:  "Sonic",
		FileNames: []string{"Sonic (USA).gba", "Sonic (Europe).gba", "Sonic (Japan).gba"},
	}

	if layout.IsInstalled(dir) {
		t.Error("expected not installed with no files present")
	}

	// Deliberately not the first entry.
	touch(t, dir, "Sonic (Japan).gba")
	if !layout.IsInstalled(dir) {
		t.Error("expected installed when any version is present")
	}
}

// A multi-disc game is represented by its playlist. The individual discs
// existing is not enough, because the launcher reads the playlist.
func TestRomLayout_IsInstalled_MultiDiscNeedsThePlaylist(t *testing.T) {
	dir := t.TempDir()
	layout := RomLayout{
		BaseName:  "Final Fantasy VII",
		MultiDisc: true,
		FileNames: []string{"disc1.bin", "disc2.bin"},
	}

	touch(t, dir, "disc1.bin")
	touch(t, dir, "disc2.bin")
	if layout.IsInstalled(dir) {
		t.Error("discs alone must not count as installed without the playlist")
	}

	touch(t, dir, "Final Fantasy VII.m3u")
	if !layout.IsInstalled(dir) {
		t.Error("expected installed once the playlist exists")
	}
}

func TestRomLayout_IsInstalled_EmptyInputs(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Sonic.gba")

	if (RomLayout{BaseName: "Sonic", FileNames: []string{"Sonic.gba"}}).IsInstalled("") {
		t.Error("an empty rom directory must not report installed")
	}
	if (RomLayout{BaseName: "Sonic"}).IsInstalled(dir) {
		t.Error("a rom with no files must not report installed")
	}
}

func TestIsFileInstalled(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Sonic (USA).gba")

	if !IsFileInstalled(dir, "Sonic (USA).gba") {
		t.Error("expected the present file to be reported installed")
	}
	if IsFileInstalled(dir, "Sonic (Europe).gba") {
		t.Error("expected an absent file to be reported missing")
	}
	if IsFileInstalled("", "Sonic (USA).gba") {
		t.Error("an empty rom directory must not report installed")
	}
	if IsFileInstalled(dir, "") {
		t.Error("an empty file name must not report installed")
	}
}
