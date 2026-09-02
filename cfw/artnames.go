package cfw

// ArtSlot identifies which piece of artwork a file holds on device.
//
// It is distinct from artutil.ArtKind, which names the image RomM serves
// (Box2D, Screenshot, and so on). A slot is where that image is stored locally,
// and several kinds can share one slot.
type ArtSlot int

const (
	ArtCover ArtSlot = iota
	ArtScreenshotPreview
	ArtBezel
	ArtThumbnail
	ArtMarquee
	ArtBoxback
	ArtFanart
)

func (s ArtSlot) String() string {
	switch s {
	case ArtCover:
		return "cover"
	case ArtScreenshotPreview:
		return "preview"
	case ArtBezel:
		return "bezel"
	case ArtThumbnail:
		return "thumbnail"
	case ArtMarquee:
		return "marquee"
	case ArtBoxback:
		return "boxback"
	case ArtFanart:
		return "fanart"
	default:
		return "unknown"
	}
}

// esSuffix is the filename suffix a slot takes on EmulationStation-based
// firmwares, which keep every art kind in a single directory and would
// otherwise overwrite the cover. Slots absent from this map share the cover's
// name deliberately.
var esSuffix = map[ArtSlot]string{
	ArtThumbnail: "-thumb",
	ArtMarquee:   "-marquee",
	ArtBoxback:   "-boxback",
	ArtFanart:    "-fanart",
}

// ArtFileName returns the file name a rom's artwork must have for firmware c to
// find it.
//
// romFileName is the rom's file name including its extension, or empty when the
// rom has no file list; baseName is the name without it.
//
// Two rules apply. MinUI names artwork after the rom file including its
// extension ("Sonic.gba.png") where every other firmware drops it
// ("Sonic.png"); and ES-based firmwares suffix the secondary slots.
//
// Callers must not build these names themselves. When ui/download.go and
// ui/artwork_sync.go each had their own copy they disagreed, so artwork sync
// probed for a name MinUI never wrote and re-downloaded every rom's art on
// every run.
func ArtFileName(c CFW, slot ArtSlot, romFileName, baseName string) string {
	stem := baseName

	// Only MinUI is special cased here, matching long-standing behaviour.
	// NextUI shares much of MinUI's layout but has never used this rule.
	if c == MinUI && romFileName != "" {
		stem = romFileName
	}

	if c.IsBasedOnEmulationStation() {
		return stem + esSuffix[slot] + ".png"
	}
	return stem + ".png"
}
