package cfw

import (
	"path/filepath"

	"grout/internal/fileutil"
)

// playlistExt is the extension of the playlist that stands in for a multi-disc
// game on disk. One file represents the whole game to the launcher.
const playlistExt = ".m3u"

// RomLayout describes how one rom is stored on disk.
//
// It deliberately holds no server identifiers. Whether a rom is installed is a
// question about the filesystem, so answering it needs only the names involved
// -- which is what keeps the RomM client free of filesystem knowledge and this
// package free of the RomM client.
type RomLayout struct {
	// BaseName is the rom's file name without its extension. A multi-disc
	// game is stored as "<BaseName>.m3u".
	BaseName string
	// MultiDisc marks a game whose discs are listed in a playlist rather than
	// stored as one file.
	MultiDisc bool
	// FileNames are the rom's individual files, as the server names them.
	FileNames []string
}

// InstalledPath returns where the rom lives under romDir, or "" when there is
// nothing to install.
func (l RomLayout) InstalledPath(romDir string) string {
	if romDir == "" {
		return ""
	}
	if l.MultiDisc {
		return filepath.Join(romDir, l.BaseName+playlistExt)
	}
	if len(l.FileNames) > 0 {
		return filepath.Join(romDir, l.FileNames[0])
	}
	return ""
}

// IsInstalled reports whether the rom is present under romDir.
//
// A multi-disc game counts as installed once its playlist exists; a single-file
// game once any of its files does, since a game with several versions needs
// only one of them on disk.
func (l RomLayout) IsInstalled(romDir string) bool {
	if romDir == "" {
		return false
	}

	if l.MultiDisc {
		return fileutil.FileExists(filepath.Join(romDir, l.BaseName+playlistExt))
	}

	for _, name := range l.FileNames {
		if fileutil.FileExists(filepath.Join(romDir, name)) {
			return true
		}
	}
	return false
}

// IsFileInstalled reports whether one named file of a rom is present under
// romDir, for games that offer several versions to choose between.
func IsFileInstalled(romDir, fileName string) bool {
	if romDir == "" || fileName == "" {
		return false
	}
	return fileutil.FileExists(filepath.Join(romDir, fileName))
}
