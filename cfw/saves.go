package cfw

import (
	"grout/cfw/knulli"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/spruce"
)

// EmulatorFolderMap returns the emulator/save directory mapping for the given CFW.
func EmulatorFolderMap(c CFW) map[string][]string {
	switch c {
	case MuOS:
		return muos.SaveDirectories
	case NextUI:
		return nextui.SaveDirectories
	case Knulli:
		return knulli.Platforms // Knulli uses platforms map for save directories
	case Spruce:
		return spruce.SaveDirectories
	default:
		return nil
	}
}

// EmulatorFoldersForFSSlug returns the emulator folders for a given filesystem slug.
func EmulatorFoldersForFSSlug(fsSlug string) []string {
	saveDirectoriesMap := EmulatorFolderMap(GetCFW())
	if saveDirectoriesMap == nil {
		return nil
	}
	return saveDirectoriesMap[fsSlug]
}
