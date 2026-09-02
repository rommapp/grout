package cfw

import (
	"path/filepath"
	"strings"
	"testing"
)

// allFirmwares is every firmware grout supports. A conformance table over this
// list is what stops a new firmware from silently returning "" for a directory
// nobody remembered to add it to.
var allFirmwares = []CFW{
	NextUI, MuOS, Knulli, Spruce, ROCKNIX, Trimui,
	Allium, Onion, Koriki, ArkOS, Batocera, MinUI,
}

var esFirmwares = []CFW{Knulli, ROCKNIX, ArkOS, Batocera}

func isES(c CFW) bool {
	for _, e := range esFirmwares {
		if e == c {
			return true
		}
	}
	return false
}

const (
	testRomDir   = "/roms/gba"
	testFSSlug   = "gba"
	testPlatform = "Game Boy Advance"
)

func artDir(c CFW, slot ArtSlot) string {
	return ArtDirectory(c, slot, testRomDir, testFSSlug, testPlatform)
}

// Every firmware must know where cover art goes. This is the one kind with no
// excuse for being absent.
func TestArtDirectory_EveryFirmwareHasACoverDirectory(t *testing.T) {
	for _, c := range allFirmwares {
		if got := artDir(c, ArtCover); got == "" {
			t.Errorf("%s has no cover art directory", c)
		}
	}
}

// The EmulationStation family keeps marquee, box back and fanart beside the
// cover, told apart by a filename suffix. No other firmware has anywhere to
// put them, and saying so explicitly is what lets ArtFileName's suffixes be
// safe.
func TestArtDirectory_SuffixedSlotsShareTheCoverDirectory(t *testing.T) {
	suffixed := []ArtSlot{ArtMarquee, ArtBoxback, ArtFanart}

	for _, c := range allFirmwares {
		cover := artDir(c, ArtCover)
		for _, slot := range suffixed {
			got := artDir(c, slot)
			if isES(c) {
				if got != cover {
					t.Errorf("%s %s = %q, want the cover directory %q", c, slot, got, cover)
				}
			} else if got != "" {
				t.Errorf("%s %s = %q, want empty for a non-ES firmware", c, slot, got)
			}
		}
	}
}

// Videos, manuals and bezels have their own directories, and only the
// EmulationStation family has them at all.
func TestArtDirectory_SidecarSlotsAreESOnly(t *testing.T) {
	sidecars := map[ArtSlot]string{
		ArtVideo:  "videos",
		ArtManual: "manuals",
		ArtBezel:  "bezels",
	}

	for _, c := range allFirmwares {
		for slot, dirName := range sidecars {
			got := artDir(c, slot)
			if !isES(c) {
				if got != "" {
					t.Errorf("%s %s = %q, want empty for a non-ES firmware", c, slot, got)
				}
				continue
			}
			want := filepath.Join(testRomDir, dirName)
			if got != want {
				t.Errorf("%s %s = %q, want %q", c, slot, got, want)
			}
		}
	}
}

// Each sidecar must have its own directory. Returning the same path for two of
// them would have one overwrite the other.
func TestArtDirectory_SidecarSlotsDoNotCollide(t *testing.T) {
	for _, c := range esFirmwares {
		seen := map[string]ArtSlot{}
		for _, slot := range []ArtSlot{ArtCover, ArtVideo, ArtManual, ArtBezel} {
			dir := artDir(c, slot)
			if other, clash := seen[dir]; clash {
				t.Errorf("%s: slots %s and %s share directory %q", c, other, slot, dir)
			}
			seen[dir] = slot
		}
	}
}

// The preview and thumbnail slots exist only on muOS, which keeps a catalogue
// of per-platform art rather than putting it beside the roms.
func TestArtDirectory_PreviewAndThumbnailAreMuOSOnly(t *testing.T) {
	t.Setenv("BASE_PATH", t.TempDir())

	for _, c := range allFirmwares {
		for _, slot := range []ArtSlot{ArtScreenshotPreview, ArtThumbnail} {
			got := artDir(c, slot)
			if c == MuOS {
				if got == "" {
					t.Errorf("muOS %s should have a directory", slot)
				}
				continue
			}
			if got != "" {
				t.Errorf("%s %s = %q, want empty", c, slot, got)
			}
		}
	}
}

// muOS keeps every kind under one catalogue entry, so the kinds must land in
// different leaves of it.
func TestArtDirectory_MuOSKindsUseDistinctCatalogueLeaves(t *testing.T) {
	t.Setenv("BASE_PATH", t.TempDir())

	cover := artDir(MuOS, ArtCover)
	preview := artDir(MuOS, ArtScreenshotPreview)
	thumbnail := artDir(MuOS, ArtThumbnail)

	for name, dir := range map[string]string{"cover": cover, "preview": preview, "thumbnail": thumbnail} {
		if !strings.Contains(dir, "catalogue") {
			t.Errorf("muOS %s directory %q should sit under the catalogue", name, dir)
		}
	}
	if cover == preview || cover == thumbnail || preview == thumbnail {
		t.Errorf("muOS kinds collide: cover=%q preview=%q thumbnail=%q", cover, preview, thumbnail)
	}
}

// An unrecognised firmware or slot yields no directory rather than a wrong one.
func TestArtDirectory_UnknownInputsReturnEmpty(t *testing.T) {
	if got := ArtDirectory(CFW("NOPE"), ArtCover, testRomDir, testFSSlug, testPlatform); got != "" {
		t.Errorf("unknown firmware returned %q, want empty", got)
	}
	if got := artDir(Knulli, ArtSlot(99)); got != "" {
		t.Errorf("unknown slot returned %q, want empty", got)
	}
}

// Batocera used to be routed to Knulli's directory for one slot. Both happened
// to resolve to the same path, so it went unnoticed; assert each firmware
// answers for itself.
func TestArtDirectory_EachFirmwareAnswersForItself(t *testing.T) {
	for _, slot := range []ArtSlot{ArtCover, ArtMarquee, ArtBoxback, ArtFanart, ArtVideo, ArtManual, ArtBezel} {
		for _, c := range esFirmwares {
			if got := artDir(c, slot); !strings.HasPrefix(got, testRomDir) {
				t.Errorf("%s %s = %q, want a path under %q", c, slot, got, testRomDir)
			}
		}
	}
}
