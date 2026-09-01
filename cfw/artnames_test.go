package cfw

import (
	"testing"

	"grout/romm"
)

func gbaRom() romm.Rom {
	return romm.Rom{
		FsName:      "Sonic the Hedgehog.gba",
		FsNameNoExt: "Sonic the Hedgehog",
		Files:       []romm.RomFile{{FileName: "Sonic the Hedgehog.gba"}},
	}
}

func TestArtFileName(t *testing.T) {
	tests := []struct {
		name string
		cfw  CFW
		slot ArtSlot
		rom  romm.Rom
		want string
	}{
		// MinUI names artwork after the rom file including its extension. This
		// is the rule ui/download.go applied and ui/artwork_sync.go did not, so
		// every MinUI rom's art was reported missing and re-downloaded forever.
		{"minui cover keeps the rom extension", MinUI, ArtCover, gbaRom(), "Sonic the Hedgehog.gba.png"},
		{"minui preview keeps the rom extension", MinUI, ArtScreenshotPreview, gbaRom(), "Sonic the Hedgehog.gba.png"},
		{"minui is not ES based so takes no suffix", MinUI, ArtThumbnail, gbaRom(), "Sonic the Hedgehog.gba.png"},

		// With no file list there is no extension to use.
		{"minui without files falls back to the bare name", MinUI, ArtCover,
			romm.Rom{FsName: "Sonic the Hedgehog.gba", FsNameNoExt: "Sonic the Hedgehog"},
			"Sonic the Hedgehog.png"},

		// ES-based firmwares keep every art kind in one directory, so the
		// secondary kinds carry a suffix to avoid overwriting the cover.
		{"knulli cover", Knulli, ArtCover, gbaRom(), "Sonic the Hedgehog.png"},
		{"knulli thumbnail", Knulli, ArtThumbnail, gbaRom(), "Sonic the Hedgehog-thumb.png"},
		{"knulli marquee", Knulli, ArtMarquee, gbaRom(), "Sonic the Hedgehog-marquee.png"},
		{"knulli boxback", Knulli, ArtBoxback, gbaRom(), "Sonic the Hedgehog-boxback.png"},
		{"knulli fanart", Knulli, ArtFanart, gbaRom(), "Sonic the Hedgehog-fanart.png"},
		{"knulli bezel shares the cover name", Knulli, ArtBezel, gbaRom(), "Sonic the Hedgehog.png"},
		{"rocknix behaves like knulli", ROCKNIX, ArtMarquee, gbaRom(), "Sonic the Hedgehog-marquee.png"},
		{"arkos behaves like knulli", ArkOS, ArtThumbnail, gbaRom(), "Sonic the Hedgehog-thumb.png"},
		{"batocera behaves like knulli", Batocera, ArtBoxback, gbaRom(), "Sonic the Hedgehog-boxback.png"},

		// Everything else drops the extension and takes no suffix.
		{"spruce cover", Spruce, ArtCover, gbaRom(), "Sonic the Hedgehog.png"},
		{"spruce thumbnail takes no suffix", Spruce, ArtThumbnail, gbaRom(), "Sonic the Hedgehog.png"},
		{"muos marquee takes no suffix", MuOS, ArtMarquee, gbaRom(), "Sonic the Hedgehog.png"},
		{"nextui is not special cased like minui", NextUI, ArtCover, gbaRom(), "Sonic the Hedgehog.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArtFileName(tt.cfw, tt.slot, tt.rom); got != tt.want {
				t.Errorf("ArtFileName(%v, %v) = %q, want %q", tt.cfw, tt.slot, got, tt.want)
			}
		})
	}
}

// On an ES-based firmware every art kind shares one directory, so no two slots
// may produce the same file name -- otherwise one overwrites another.
func TestArtFileName_ESSlotsDoNotCollide(t *testing.T) {
	distinct := []ArtSlot{ArtCover, ArtThumbnail, ArtMarquee, ArtBoxback, ArtFanart}

	seen := make(map[string]ArtSlot, len(distinct))
	for _, slot := range distinct {
		name := ArtFileName(Knulli, slot, gbaRom())
		if other, clash := seen[name]; clash {
			t.Errorf("slots %v and %v both produce %q", other, slot, name)
		}
		seen[name] = slot
	}
}

// The name must be stable regardless of how the rom's display name is rendered,
// since art files are matched by name on disk.
func TestArtFileName_IgnoresDisplayName(t *testing.T) {
	rom := gbaRom()
	rom.Name = "Sonic The Hedgehog"
	rom.Regions = []string{"USA"}
	withRegions := ArtFileName(Knulli, ArtCover, rom)

	rom.Regions = nil
	rom.Name = "something else entirely"
	withoutRegions := ArtFileName(Knulli, ArtCover, rom)

	if withRegions != withoutRegions {
		t.Errorf("art name changed with the display name: %q vs %q", withRegions, withoutRegions)
	}
}
