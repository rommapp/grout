package cfw

import (
	"grout/cfw/allium"
	"grout/cfw/arkos"
	"grout/cfw/batocera"
	"grout/cfw/knulli"
	"grout/cfw/koriki"
	"grout/cfw/minui"
	"grout/cfw/muos"
	"grout/cfw/nextui"
	"grout/cfw/onion"
	"grout/cfw/rocknix"
	"grout/cfw/spruce"
	"grout/cfw/trimui"
	"path/filepath"
	"strings"
)

// SaveBasename returns the on-disk basename (the part before the save-file extension) an
// emulator uses for a ROM's save files, given whether the device keeps the ROM extension.
//
// keepRomExt=false is the RetroArch convention: the save is named after the ROM basename
// WITHOUT its extension (e.g. ROM "Game (USA).gba" -> save "Game (USA).srm"). keepRomExt=true
// is the minarch convention (NextUI/MinUI default): the save keeps the FULL ROM filename,
// extension included (e.g. ROM "Game (USA).sfc" -> save "Game (USA).sfc.sav"). Reading or
// writing a save under the wrong convention silently breaks sync (issue #245). NextUI
// exposes both as a setting, so the style is detected per-device rather than assumed by CFW.
func SaveBasename(keepRomExt bool, romFileName string) string {
	if keepRomExt {
		return romFileName
	}
	return strings.TrimSuffix(romFileName, filepath.Ext(romFileName))
}

// DefaultKeepsRomExt reports the CFW's default save-naming style, used only as a fallback
// when the actual on-device convention can't be detected from existing saves. The minarch
// CFWs (NextUI, MinUI) default to keeping the ROM extension; all others default to the
// RetroArch convention of stripping it (issue #245).
func DefaultKeepsRomExt(c CFW) bool {
	switch c {
	case NextUI, MinUI:
		return true
	default:
		return false
	}
}

// EmulatorFolderMap returns the emulator/save directory mapping for the given CFW.
func EmulatorFolderMap(c CFW) map[string][]string {
	switch c {
	case MuOS:
		return muos.SaveDirectories
	case NextUI:
		return nextui.SaveDirectories
	case Knulli:
		return knulli.SaveDirectories
	case Spruce:
		return spruce.SaveDirectories
	case ROCKNIX:
		return rocknix.Platforms // ROCKNIX stores saves alongside ROMs
	case ArkOS:
		return arkos.Platforms // ArkOS stores saves alongside ROMs
	case Allium:
		return allium.SaveDirectories
	case Onion:
		return onion.SaveDirectories
	case Koriki:
		return koriki.SaveDirectories
	case Trimui:
		return trimui.SaveDirectories
	case Batocera:
		return batocera.Platforms
	case MinUI:
		return minui.SaveDirectories
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
