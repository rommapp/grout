package cfw

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"grout/settings"
)

func TestPlatformDirectories(t *testing.T) {
	tests := []struct {
		name    string
		cfw     CFW
		fsSlug  string
		binding map[string]string
		want    []string
	}{
		{"known platform", MuOS, "gba", nil, []string{"gba", "Nintendo Game Boy Advance"}},
		{"tagged firmware", MinUI, "gba", nil, []string{"Game Boy Advance (GBA)", "Game Boy Advance (MGBA)"}},
		{"unknown platform falls back to its slug", MuOS, "not-a-console", nil, []string{"not-a-console"}},
		{"binding redirects the lookup", MuOS, "gameboyadvance", map[string]string{"gameboyadvance": "gba"}, []string{"gba", "Nintendo Game Boy Advance"}},
		{"binding for an unknown target still falls back", MuOS, "x", map[string]string{"x": "y"}, []string{"y"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlatformDirectories(tt.cfw, tt.fsSlug, tt.binding); !slices.Equal(got, tt.want) {
				t.Errorf("PlatformDirectories = %v, want %v", got, tt.want)
			}
		})
	}
}

// These functions take the firmware as an argument, so they must answer for
// that firmware rather than for whichever one the device happens to be running.
func TestDirectoriesMatch_HonoursTheFirmwareGiven(t *testing.T) {
	t.Setenv(EnvVar, string(MuOS))

	if !DirectoriesMatch(MinUI, "Game Boy Advance (GBA)", "Handheld (GBA)") {
		t.Error("MinUI compares folders by tag, so both are the GBA folder")
	}
	if DirectoriesMatch(MuOS, "Game Boy Advance (GBA)", "Handheld (GBA)") {
		t.Error("muOS compares folder names in full")
	}
}

func TestDirectoryMatchesPlatform(t *testing.T) {
	t.Setenv(EnvVar, string(MinUI))

	tests := []struct {
		name   string
		cfw    CFW
		fsSlug string
		dir    string
		want   bool
	}{
		{"exact folder", MuOS, "gba", "gba", true},
		{"wrong platform", MuOS, "gba", "snes", false},
		// muOS lists a long-form alias too, but auto-detection only recognises
		// the primary folder name.
		{"alias is not auto-detected", MuOS, "gba", "Nintendo Game Boy Advance", false},
		{"tagged folder", MinUI, "gba", "Game Boy Advance (GBA)", true},
		{"a different tag is a different folder", MinUI, "gba", "Game Boy Advance (MGBA)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DirectoryMatchesPlatform(tt.cfw, tt.fsSlug, tt.dir); got != tt.want {
				t.Errorf("DirectoryMatchesPlatform(%s, %q, %q) = %v, want %v", tt.cfw, tt.fsSlug, tt.dir, got, tt.want)
			}
		})
	}
}

func TestIsPlatformDirectory(t *testing.T) {
	dirs := PlatformDirectories(MuOS, "gba", nil)

	if !IsPlatformDirectory(MuOS, "Nintendo Game Boy Advance", dirs) {
		t.Error("every folder the firmware accepts must be recognised, not just the first")
	}
	if IsPlatformDirectory(MuOS, "snes", dirs) {
		t.Error("another platform's folder must not match")
	}
}

func TestCreateRomDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "gba"), 0755); err != nil {
		t.Fatal(err)
	}

	mappings := map[string]settings.DirectoryMapping{
		"gba":  {RomMSlug: "gba", RelativePath: "gba"},
		"snes": {RomMSlug: "snes", RelativePath: "snes"},
		// A skipped platform stores an empty path and must not create the root.
		"n64": {RomMSlug: "n64", RelativePath: ""},
	}

	if err := CreateRomDirectories(mappings, root, []string{"gba"}); err != nil {
		t.Fatalf("CreateRomDirectories: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	if want := []string{"gba", "snes"}; !slices.Equal(names, want) {
		t.Errorf("directories = %v, want %v", names, want)
	}
}

// The rom root is on a card the user can pull out. A missing one has to surface
// as an error rather than a panic or a silent success.
func TestCreateRomDirectories_UnwritableRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	mappings := map[string]settings.DirectoryMapping{
		"gba": {RomMSlug: "gba", RelativePath: "gba"},
	}
	if err := CreateRomDirectories(mappings, root, nil); err == nil {
		t.Error("expected an error when the rom root is not a directory")
	}
}
