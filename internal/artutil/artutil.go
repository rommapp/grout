package artutil

type ArtKind string

const (
	ArtKindNone       ArtKind = ""
	ArtKindDefault    ArtKind = "Default"
	ArtKindBox2D      ArtKind = "Box2D"
	ArtKindBox3D      ArtKind = "Box3D"
	ArtKindMixImage   ArtKind = "Miximage"
	ArtKindMarquee    ArtKind = "Marquee"
	ArtKindTitle      ArtKind = "Title"
	ArtKindScreenshot ArtKind = "Screenshot"
	ArtKindVideo      ArtKind = "Video"
)
