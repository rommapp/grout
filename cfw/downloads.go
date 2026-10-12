package cfw

import (
	"grout/files"
	"path/filepath"
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
	return l.LocalPath(romDir) != ""
}

// LocalPath returns the file that makes the rom present under romDir: the
// playlist of a multi-disc game, or the first of its versions found. It is ""
// when the rom is not on the device.
func (l RomLayout) LocalPath(romDir string) string {
	if romDir == "" {
		return ""
	}

	if l.MultiDisc {
		if playlist := filepath.Join(romDir, l.BaseName+playlistExt); files.FileExists(playlist) {
			return playlist
		}
		return ""
	}

	for _, name := range l.FileNames {
		if path := filepath.Join(romDir, name); files.FileExists(path) {
			return path
		}
	}
	return ""
}

// IsFileDownloaded reports whether one named file of a rom is present.
func IsFileDownloaded(romDir, fileName string) bool {
	if romDir == "" || fileName == "" {
		return false
	}
	return files.FileExists(filepath.Join(romDir, fileName))
}
