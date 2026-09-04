package catalog

import (
	"grout/cfw"
	"grout/romm"
	"grout/settings"
)

// DownloadState says how much of a game is already on the device.
type DownloadState int

const (
	NotDownloaded DownloadState = iota
	// PartlyDownloaded is a game stored as several files where some, but not
	// all of them, have arrived.
	PartlyDownloaded
	FullyDownloaded
)

// RomDirectory is where a game's files belong.
//
// It uses the rom's own platform slug, which differs from the screen's when
// browsing a collection that spans several platforms.
func RomDirectory(config settings.Config, game romm.Rom) string {
	return cfw.PlatformRomDirectory(config, game.PlatformFSSlug)
}

// romLayout describes how a game is stored on disk. Here rather than on
// romm.Rom so the API client stays free of the filesystem.
func romLayout(game romm.Rom) cfw.RomLayout {
	names := make([]string, 0, len(game.Files))
	for _, file := range game.Files {
		names = append(names, file.FileName)
	}
	return cfw.RomLayout{
		BaseName:  game.FsNameNoExt,
		MultiDisc: game.HasMultipleFiles,
		FileNames: names,
	}
}

// IsDownloaded reports whether a game is already on the device.
func IsDownloaded(config settings.Config, game romm.Rom) bool {
	if game.PlatformFSSlug == "" {
		return false
	}
	return romLayout(game).IsDownloaded(RomDirectory(config, game))
}

// IsFileDownloaded reports whether one named version of a game is present.
func IsFileDownloaded(config settings.Config, game romm.Rom, fileName string) bool {
	if game.PlatformFSSlug == "" {
		return false
	}
	return cfw.IsFileDownloaded(RomDirectory(config, game), fileName)
}

// DownloadStateOf reports how much of a game is on the device.
//
// Every answer costs a look at the card, which is slow enough on these devices
// to be worth asking for only when something will use it.
func DownloadStateOf(config settings.Config, game romm.Rom) DownloadState {
	if !game.HasNestedSingleFile {
		if IsDownloaded(config, game) {
			return FullyDownloaded
		}
		return NotDownloaded
	}

	if len(game.Files) == 0 {
		return NotDownloaded
	}

	present := 0
	for _, file := range game.Files {
		if IsFileDownloaded(config, game, file.FileName) {
			present++
		}
	}

	switch {
	case present == len(game.Files):
		return FullyDownloaded
	case present > 0:
		return PartlyDownloaded
	default:
		return NotDownloaded
	}
}
