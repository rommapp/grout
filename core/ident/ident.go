package ident

import "maps"

type Status string

const (
	StatusOK          Status = "ok"
	StatusNotFound    Status = "not_found"
	StatusNeedsKey    Status = "needs_key"
	StatusUnsupported Status = "unsupported"
	StatusIO          Status = "io"
	StatusFailed      Status = "failed"
)

func Statuses() []Status {
	return []Status{StatusOK, StatusNotFound, StatusNeedsKey, StatusUnsupported, StatusIO, StatusFailed}
}

type Source string

const (
	SourceBinary   Source = "binary"
	SourceFilename Source = "filename"
)

type Usage string

const (
	UsageFolderExact  Usage = "folder-exact"
	UsageFolderPrefix Usage = "folder-prefix"
	UsageFileExact    Usage = "file-exact"
	UsageFilePrefix   Usage = "file-prefix"
	UsageFolderSplit  Usage = "folder-split"
)

type Result struct {
	Status       Status
	TitleID      string
	SaveID       string
	RawSerial    string
	Platform     string
	Source       Source
	Usage        Usage
	Features     uint32
	Experimental bool
	N64Header    string
	N64MD5       string
	N64MD5N64    string
}

type Extractor interface {
	Extract(path, platform string) Result
	Version() string
}

var platforms = map[string]string{
	"psx": "psx", "ps2": "ps2", "psp": "psp", "psvita": "psvita", "ps3": "ps3",
	"switch": "switch", "3ds": "3ds", "new-nintendo-3ds": "3ds",
	"wii": "wii", "wiiu": "wiiu", "ngc": "gamecube", "dc": "dreamcast",
	"xbox": "xbox", "xbox360": "xbox360",
	"gb": "gb", "gbc": "gbc", "snes": "snes", "sfam": "snes", "n64": "n64",
}

func Platform(rommSlug string) (string, bool) {
	v, ok := platforms[rommSlug]
	return v, ok
}

func Platforms() map[string]string {
	return maps.Clone(platforms)
}
