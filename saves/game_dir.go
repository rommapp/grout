package saves

import (
	"path/filepath"
	"strings"

	"grout/settings"
)

// SaveDirectoryForGame is the folder the core a game runs on keeps its saves in,
// whether or not there is a save there yet, or "" when that cannot be told.
func SaveDirectoryForGame(fsSlug, romPath string, config *settings.Config) string {
	return saveDirectoryForCore(fsSlug, romPath, config)
}

// ChangesSaveInUse reports whether making a copy the game's save in destDir
// replaces the save the game reads. A copy chosen from RomM (nil) always does; a
// copy from the folder it is already in does not.
func ChangesSaveInUse(copy *LocalSave, destDir string) bool {
	if copy == nil {
		return true
	}
	return !strings.EqualFold(FolderName(*copy), filepath.Base(destDir))
}
