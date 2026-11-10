package saves

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/files"
	"grout/settings"
)

// LocalSavesFor lists the saves on the device that belong to one game.
func LocalSavesFor(config *settings.Config, romID int) []LocalSave {
	var mine []LocalSave
	for _, save := range ScanSaves(config) {
		if save.RomID == romID {
			mine = append(mine, save)
		}
	}
	return mine
}

// SavesToCarry picks the saves that should follow a game to another core's
// folder: the ones in other folders, when the new folder has none of the
// game's saves yet. A save already there is the one the game will use, and is
// never replaced. Saves kept as folders, which belong to other platforms, are
// left alone.
func SavesToCarry(local []LocalSave, destFolder string) []LocalSave {
	var elsewhere []LocalSave
	for _, save := range local {
		if save.IsDirectorySave {
			continue
		}
		if strings.EqualFold(filepath.Base(filepath.Dir(save.FilePath)), destFolder) {
			return nil
		}
		elsewhere = append(elsewhere, save)
	}
	return elsewhere
}

// CarrySave moves a save into another folder. Two saves for one game would be
// reported to the server as two, so the original does not stay where Grout
// finds it: it goes to the .backup folder beside it, named the way the backups
// made before a download are, and can be put back by hand.
//
// The copy is made and checked first, and if the original cannot be put away
// the copy is removed again, so a failure never leaves the game with two.
func CarrySave(save LocalSave, destDir string) error {
	dest := filepath.Join(destDir, save.FileName)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}

	info, err := os.Stat(save.FilePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	if err := files.CopyFile(save.FilePath, dest); err != nil {
		os.Remove(dest)
		return err
	}
	if copied, err := os.Stat(dest); err != nil || copied.Size() != info.Size() {
		os.Remove(dest)
		return fmt.Errorf("the copy at %s does not match the original", dest)
	}
	// The copy is the same save, so it keeps the save's own time.
	os.Chtimes(dest, info.ModTime(), info.ModTime())

	if _, err := putAway(save.FilePath, save.FileName); err != nil {
		os.Remove(dest)
		return fmt.Errorf("could not put the original at %s away: %w", save.FilePath, err)
	}
	return nil
}
