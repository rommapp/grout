package library

// ArtKind names which image RomM should serve. Distinct from cfw.ArtSlot,
// which is where the answer is stored on device.
//
// These values are persisted as art_kind in config.json. Renaming one resets
// the setting for every existing install; add kinds rather than renaming.
type ArtKind string

const (
	ArtKindNone ArtKind = ""
	// ArtKindDefault uses whatever cover RomM considers primary.
	ArtKindDefault    ArtKind = "Default"
	ArtKindBox2D      ArtKind = "Box2D"
	ArtKindBox3D      ArtKind = "Box3D"
	ArtKindMixImage   ArtKind = "Miximage"
	ArtKindMarquee    ArtKind = "Marquee"
	ArtKindLogo       ArtKind = "Logo"
	ArtKindTitle      ArtKind = "Title"
	ArtKindScreenshot ArtKind = "Screenshot"
	ArtKindVideo      ArtKind = "Video"
)

func (k ArtKind) Enabled() bool { return k != ArtKindNone }
