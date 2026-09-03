package ui

import (
	"grout/cfw"
	"grout/romm"
	"grout/settings"
)

// romDirectoryFor uses the rom's own platform slug, which differs from the
// screen's when browsing a collection spanning several platforms.
func romDirectoryFor(config settings.Config, g romm.Rom) string {
	return cfw.PlatformRomDirectory(config, g.PlatformFSSlug)
}

// romLayout describes how g is stored on disk. Here rather than on romm.Rom so
// the API client stays free of the filesystem.
func romLayout(g romm.Rom) cfw.RomLayout {
	names := make([]string, 0, len(g.Files))
	for _, f := range g.Files {
		names = append(names, f.FileName)
	}
	return cfw.RomLayout{
		BaseName:  g.FsNameNoExt,
		MultiDisc: g.HasMultipleFiles,
		FileNames: names,
	}
}

func isRomDownloaded(config settings.Config, g romm.Rom) bool {
	if g.PlatformFSSlug == "" {
		return false
	}
	return romLayout(g).IsDownloaded(romDirectoryFor(config, g))
}

// isRomFileDownloaded reports whether one named version of g is present.
func isRomFileDownloaded(config settings.Config, g romm.Rom, fileName string) bool {
	if g.PlatformFSSlug == "" {
		return false
	}
	return cfw.IsFileDownloaded(romDirectoryFor(config, g), fileName)
}
