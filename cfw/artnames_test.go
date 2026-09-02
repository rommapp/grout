package cfw

import "testing"

const (
	gbaFileName = "Sonic the Hedgehog.gba"
	gbaBaseName = "Sonic the Hedgehog"
)

func TestArtFileName(t *testing.T) {
	tests := []struct {
		name string
		cfw  CFW
		slot ArtSlot
		file string
		base string
		want string
	}{
		// MinUI names artwork after the rom file including its extension. This
		// is the rule ui/download.go applied and ui/artwork_sync.go did not, so
		// every MinUI rom's art was reported missing and re-downloaded forever.
		{"minui cover keeps the rom extension", MinUI, ArtCover, gbaFileName, gbaBaseName, "Sonic the Hedgehog.gba.png"},
		{"minui preview keeps the rom extension", MinUI, ArtScreenshotPreview, gbaFileName, gbaBaseName, "Sonic the Hedgehog.gba.png"},
		{"minui is not ES based so takes no suffix", MinUI, ArtThumbnail, gbaFileName, gbaBaseName, "Sonic the Hedgehog.gba.png"},

		// With no file list there is no extension to use.
		{"minui without files falls back to the bare name", MinUI, ArtCover,
			"", gbaBaseName,
			"Sonic the Hedgehog.png"},

		// ES-based firmwares keep every art kind in one directory, so the
		// secondary kinds carry a suffix to avoid overwriting the cover.
		{"knulli cover", Knulli, ArtCover, gbaFileName, gbaBaseName, "Sonic the Hedgehog.png"},
		{"knulli thumbnail", Knulli, ArtThumbnail, gbaFileName, gbaBaseName, "Sonic the Hedgehog-thumb.png"},
		{"knulli marquee", Knulli, ArtMarquee, gbaFileName, gbaBaseName, "Sonic the Hedgehog-marquee.png"},
		{"knulli boxback", Knulli, ArtBoxback, gbaFileName, gbaBaseName, "Sonic the Hedgehog-boxback.png"},
		{"knulli fanart", Knulli, ArtFanart, gbaFileName, gbaBaseName, "Sonic the Hedgehog-fanart.png"},
		{"knulli bezel shares the cover name", Knulli, ArtBezel, gbaFileName, gbaBaseName, "Sonic the Hedgehog.png"},
		{"rocknix behaves like knulli", ROCKNIX, ArtMarquee, gbaFileName, gbaBaseName, "Sonic the Hedgehog-marquee.png"},
		{"arkos behaves like knulli", ArkOS, ArtThumbnail, gbaFileName, gbaBaseName, "Sonic the Hedgehog-thumb.png"},
		{"batocera behaves like knulli", Batocera, ArtBoxback, gbaFileName, gbaBaseName, "Sonic the Hedgehog-boxback.png"},

		// Everything else drops the extension and takes no suffix.
		{"spruce cover", Spruce, ArtCover, gbaFileName, gbaBaseName, "Sonic the Hedgehog.png"},
		{"spruce thumbnail takes no suffix", Spruce, ArtThumbnail, gbaFileName, gbaBaseName, "Sonic the Hedgehog.png"},
		{"muos marquee takes no suffix", MuOS, ArtMarquee, gbaFileName, gbaBaseName, "Sonic the Hedgehog.png"},
		{"nextui is not special cased like minui", NextUI, ArtCover, gbaFileName, gbaBaseName, "Sonic the Hedgehog.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArtFileName(tt.cfw, tt.slot, tt.file, tt.base); got != tt.want {
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
		name := ArtFileName(Knulli, slot, gbaFileName, gbaBaseName)
		if other, clash := seen[name]; clash {
			t.Errorf("slots %v and %v both produce %q", other, slot, name)
		}
		seen[name] = slot
	}
}
