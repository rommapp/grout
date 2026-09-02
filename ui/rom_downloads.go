package ui

import (
	"grout/cfw"
	"grout/romm"
)

// romDirResolver is implemented by internal.Config, by value or by pointer.
type romDirResolver interface {
	GetPlatformRomDirectory(platform romm.Platform) string
}

// romDirectoryFor uses the rom's own platform identifiers, which differ from
// the screen's when browsing a collection spanning several.
func romDirectoryFor(resolver romDirResolver, g romm.Rom) string {
	return resolver.GetPlatformRomDirectory(romm.Platform{
		ID:     g.PlatformID,
		FSSlug: g.PlatformFSSlug,
		Name:   g.PlatformDisplayName,
	})
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

func isRomDownloaded(resolver romDirResolver, g romm.Rom) bool {
	if g.PlatformFSSlug == "" {
		return false
	}
	return romLayout(g).IsDownloaded(romDirectoryFor(resolver, g))
}

// isRomFileDownloaded reports whether one named version of g is present.
func isRomFileDownloaded(resolver romDirResolver, g romm.Rom, fileName string) bool {
	if g.PlatformFSSlug == "" {
		return false
	}
	return cfw.IsFileDownloaded(romDirectoryFor(resolver, g), fileName)
}
