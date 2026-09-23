package cfw

// ArtSlot is where a piece of artwork is stored on device.
//
// Distinct from library.ArtKind, which names the image RomM serves. Several
// kinds can share one slot.
type ArtSlot int

const (
	ArtCover ArtSlot = iota
	ArtScreenshotPreview
	ArtBezel
	ArtThumbnail
	ArtMarquee
	ArtBoxback
	ArtFanart
	// Not images: no suffix, and written without image processing.
	ArtVideo
	ArtManual
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
	case ArtVideo:
		return "video"
	case ArtManual:
		return "manual"
	default:
		return "unknown"
	}
}

// esSuffix distinguishes slots on EmulationStation firmwares, which keep every
// art kind in one directory. Slots absent from the map share the cover's name.
var esSuffix = map[ArtSlot]string{
	ArtThumbnail: "-thumb",
	ArtMarquee:   "-marquee",
	ArtBoxback:   "-boxback",
	ArtFanart:    "-fanart",
}

// ArtFileName returns the file name a rom's artwork must have for firmware c.
//
// romFileName includes the extension, or is empty when the rom has no file
// list; baseName excludes it. MinUI names artwork after the whole rom file
// ("Sonic.gba.png"); every other firmware drops the extension.
//
// Build names here rather than at the call site: writers and the
// missing-artwork check must agree, or art is re-downloaded forever.
func ArtFileName(c CFW, slot ArtSlot, romFileName, baseName string) string {
	stem := baseName

	// NextUI shares much of MinUI's layout but has never used this rule.
	if c == MinUI && romFileName != "" {
		stem = romFileName
	}

	if c.IsBasedOnEmulationStation() {
		return stem + esSuffix[slot] + ".png"
	}
	return stem + ".png"
}
