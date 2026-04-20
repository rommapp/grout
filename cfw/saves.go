package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/batocera"
	"grout/cfw/knulli"
	"grout/cfw/minui"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/onion"
	"grout/cfw/retrodeck"
	"grout/cfw/rocknix"
	"grout/cfw/spruce"
	"grout/cfw/trimui"
	"path/filepath"
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
	case ROCKNIX:
		return rocknix.Platforms // ROCKNIX stores saves alongside ROMs
	case Allium:
		return allium.SaveDirectories
	case Onion:
		return onion.SaveDirectories
	case Trimui:
		return trimui.SaveDirectories
	case Batocera:
		return batocera.Platforms
	case MinUI:
		return minui.SaveDirectories
	case RetroDECK:
		return retrodeck.Platforms
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

// GetSaveDirectory returns the full save directory path for a given filesystem slug.
// Falls back to the first emulator folder if no match is found.
func GetSaveDirectory(fsSlug string) string {
	baseSavePath := BaseSavePath()
	if baseSavePath == "" {
		return ""
	}

	emulatorDirs := EmulatorFoldersForFSSlug(fsSlug)
	if len(emulatorDirs) == 0 {
		return ""
	}

	return filepath.Join(baseSavePath, emulatorDirs[0])
}
