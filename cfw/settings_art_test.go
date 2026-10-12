package cfw

import (
	"path/filepath"
	"strings"
	"testing"

	"grout/settings"
)

// A server that binds its "megadrive" folder to the genesis platform sends
// that binding to grout, which has to use it for art the same way it does for
// roms. Otherwise muOS art lands in a catalogue entry named after the
// platform's display name ("Sega Mega Drive/Genesis"), which muOS never reads.
func TestPlatformArtDirectory_HonoursPlatformBinding(t *testing.T) {
	t.Setenv(EnvVar, string(MuOS))
	t.Setenv("BASE_PATH", t.TempDir())

	config := settings.Config{
		DirectoryMappings: map[string]settings.DirectoryMapping{
			"megadrive": {RomMSlug: "megadrive", RelativePath: "megadrive"},
		},
		PlatformsBinding: map[string]string{"megadrive": "genesis"},
	}

	got := PlatformArtDirectory(config, ArtCover, "megadrive", "Sega Mega Drive/Genesis")
	want := filepath.Join("catalogue", "Sega Mega Drive - Genesis", "box")
	if !strings.HasSuffix(got, want) {
		t.Errorf("art directory = %q, want it to end in %q", got, want)
	}
}
