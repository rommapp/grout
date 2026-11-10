package saves

import "grout/settings"

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
