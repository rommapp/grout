package library

// ArtKind names which image RomM should serve for a game.
//
// It is distinct from cfw.ArtSlot, which names where an image is stored on
// device. A kind is what to ask the server for; a slot is where the answer
// goes. Several kinds can end up in the same slot -- a user who prefers 3D
// boxes and one who prefers 2D both get a file called the cover.
//
// These values are persisted: Config.ArtKind is written to config.json as
// art_kind. Changing a string here silently resets that setting for every
// existing install, so add new kinds rather than renaming existing ones.
type ArtKind string

const (
	// ArtKindNone means this kind of art is not downloaded at all.
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

// Enabled reports whether this kind should be downloaded.
func (k ArtKind) Enabled() bool { return k != ArtKindNone }
