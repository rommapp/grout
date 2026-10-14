package leaf

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestGetBasePath_PrefersBasePathThenSDCardPath(t *testing.T) {
	t.Setenv("BASE_PATH", "/base")
	t.Setenv("SDCARD_PATH", "/sd")
	if got := GetBasePath(); got != "/base" {
		t.Errorf("GetBasePath() = %q, want BASE_PATH", got)
	}

	t.Setenv("BASE_PATH", "")
	if got := GetBasePath(); got != "/sd" {
		t.Errorf("GetBasePath() = %q, want SDCARD_PATH", got)
	}
}

// Art goes under Images, named like the rom folder; Leaf falls back to that
// folder when the system's canonical one has no match.
func TestGetArtDirectory_MirrorsTheRomFolder(t *testing.T) {
	t.Setenv("BASE_PATH", "/sd")

	cases := map[string]string{
		"/sd/Roms/SNES":   "/sd/Images/SNES",
		"/sd/Roms/FC":     "/sd/Images/FC",
		"/sd/Roms/Custom": "/sd/Images/Custom",
	}
	for romDir, want := range cases {
		if got := GetArtDirectory(romDir); got != want {
			t.Errorf("GetArtDirectory(%q) = %q, want %q", romDir, got, want)
		}
	}
}

func TestGetBIOSFilePaths(t *testing.T) {
	t.Setenv("BASE_PATH", "/sd")

	cases := []struct {
		relativePath, slug string
		want               []string
	}{
		{"scph5501.bin", "psx", []string{"/sd/BIOS/scph5501.bin"}},
		{"dc/dc_boot.bin", "dc", []string{"/sd/BIOS/dc/dc_boot.bin"}},
		{"kick34005.A500", "amiga", []string{"/sd/BIOS/puae/kick34005.A500"}},
		{"saturn_bios.bin", "saturn", []string{"/sd/BIOS/SATURN/saturn_bios.bin", "/sd/BIOS/saturn_bios.bin"}},
		{"fbneo/neogeo.zip", "neogeoaes", []string{"/sd/BIOS/neogeo.zip", "/sd/BIOS/fbneo/neogeo.zip"}},
	}
	for _, tc := range cases {
		if got := GetBIOSFilePaths(tc.relativePath, tc.slug); !slices.Equal(got, tc.want) {
			t.Errorf("GetBIOSFilePaths(%q, %q) = %v, want %v", tc.relativePath, tc.slug, got, tc.want)
		}
	}
}

// leafRomRoots are the public rom folders of Leaf's system catalog
// (rom_root in config/retroarch/mlp1/systems.json).
var leafRomRoots = []string{
	"32X", "A7800", "AMIGA", "ARCADE", "ATARI2600", "ATOMISWAVE", "COLECO",
	"DC", "DOS", "EASYRPG", "FDS", "GB", "GBA", "GBC", "GENESIS", "GG", "GW",
	"LYNX", "MAME", "MAME2003", "MAME2010", "MD32X", "MS", "N64", "NAOMI",
	"NDS", "NEOGEO", "NES", "NGP", "NGPC", "PC98", "PCE", "PCECD", "PICO8",
	"PORTS", "PSP", "PSX", "SATURN", "SEGACD", "SNES", "VB", "VECTREX", "WS",
	"WSC",
}

// Downloads go to a platform's first folder, so it must be the one Leaf
// recommends rather than an alias: that is also where its art then lands.
func TestPlatforms_DownloadToTheCanonicalFolder(t *testing.T) {
	for slug, folders := range Platforms {
		if len(folders) > 0 && !slices.Contains(leafRomRoots, folders[0]) {
			t.Errorf("%s downloads to %q, which is not one of Leaf's rom folders", slug, folders[0])
		}
	}
}

func TestSaveDirectories_CoverEveryPlatform(t *testing.T) {
	for slug := range Platforms {
		if _, ok := SaveDirectories[slug]; !ok {
			t.Errorf("%s has a platform entry but no save entry", slug)
		}
	}
	for slug, dirs := range SaveDirectories {
		for _, dir := range dirs {
			if dir == "" || filepath.Base(dir) != dir {
				t.Errorf("%s save folder %q is not a single core folder", slug, dir)
			}
		}
	}
}

func TestGetInputMappingBytes(t *testing.T) {
	data, err := GetInputMappingBytes()
	if err != nil {
		t.Fatalf("GetInputMappingBytes() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("GetInputMappingBytes() returned no mapping")
	}
}
