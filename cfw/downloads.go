package cfw

import (
	"path/filepath"

	"grout/internal/fileutil"
)

// playlistExt names the playlist that represents a multi-disc game.
const playlistExt = ".m3u"

// RomLayout describes how one rom is stored on disk. It holds no server
// identifiers: whether a rom has been downloaded is a filesystem question.
type RomLayout struct {
	// BaseName excludes the extension. A multi-disc game is "<BaseName>.m3u".
	BaseName string
	// MultiDisc means the discs are listed in a playlist.
	MultiDisc bool
	// FileNames are the rom's files, as the server names them.
	FileNames []string
}

// DownloadPath returns where the rom's file lives under romDir, or "" if it
// has none.
func (l RomLayout) DownloadPath(romDir string) string {
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

// IsDownloaded reports whether the rom is present under romDir. A multi-disc
// game needs its playlist; a game with several versions needs any one of them.
func (l RomLayout) IsDownloaded(romDir string) bool {
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

// IsFileDownloaded reports whether one named file of a rom is present.
func IsFileDownloaded(romDir, fileName string) bool {
	if romDir == "" || fileName == "" {
		return false
	}
	return fileutil.FileExists(filepath.Join(romDir, fileName))
}
