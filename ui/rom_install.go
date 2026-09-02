package ui

import (
	"grout/cfw"
	"grout/romm"
)

// romDirResolver maps a platform to the directory its roms live in.
// internal.Config implements it, by value or by pointer.
type romDirResolver interface {
	GetPlatformRomDirectory(platform romm.Platform) string
}

// romDirectoryFor resolves where a rom's platform stores its files.
//
// A rom carries its own platform identifiers, which may differ from the screen's
// current platform when browsing a collection that spans several.
func romDirectoryFor(resolver romDirResolver, g romm.Rom) string {
	return resolver.GetPlatformRomDirectory(romm.Platform{
		ID:     g.PlatformID,
		FSSlug: g.PlatformFSSlug,
		Name:   g.PlatformDisplayName,
	})
}

// romLayout describes how g is stored on disk.
//
// This mapping lives here rather than on romm.Rom so the API client stays free
// of the filesystem: whether a rom is downloaded is a question about this
// device, not about what the server said.
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

// isRomDownloaded reports whether g is present in its platform's rom directory.
func isRomDownloaded(resolver romDirResolver, g romm.Rom) bool {
	if g.PlatformFSSlug == "" {
		return false
	}
	return romLayout(g).IsInstalled(romDirectoryFor(resolver, g))
}

// isRomFileDownloaded reports whether one named file of g is present, for games
// that offer several versions to choose between.
func isRomFileDownloaded(resolver romDirResolver, g romm.Rom, fileName string) bool {
	if g.PlatformFSSlug == "" {
		return false
	}
	return cfw.IsFileInstalled(romDirectoryFor(resolver, g), fileName)
}
