package cfw

import (
	"log/slog"

	"grout/cfw/muos"
	"grout/internal/emulationstation"
	"grout/internal/gamelist"
)

func scheduleESRestart() {
	if err := emulationstation.ScheduleESRestart(); err != nil {
		slog.Default().Debug("Unable to schedule ES restart", "error", err)
	}
}

// AddGroutToGamelist adds grout's launcher entry to the frontend's game list.
// Only the EmulationStation family has somewhere to put it.
func AddGroutToGamelist(c CFW) {
	f := Lookup(c)
	path, launcher := f.GroutGamelist(), f.GroutLauncherPath()
	if path == "" || launcher == "" {
		return
	}

	gamelist.AddGroutEntry(path, launcher)
	scheduleESRestart()
}

// FillGamesMetadata writes game metadata in the form the firmware reads.
func FillGamesMetadata(entries []gamelist.RomGameEntry) {
	logger := slog.Default()

	switch ActiveFirmware().Gamelist() {
	case GamelistEmulationStation:
		if err := gamelist.AddRomGamesToGamelist(entries, gamelist.GameListFileName); err != nil {
			logger.Warn("Failed to add games to ES gamelist.xml", "error", err)
		}
		scheduleESRestart()

	case GamelistMiyoo:
		if err := gamelist.AddRomGamesToGamelist(entries, gamelist.MiyooGameListFileName); err != nil {
			logger.Warn("Failed to add games to miyoogamelist.xml", "error", err)
		}

	case GamelistMuOSText:
		for _, entry := range entries {
			muos.AddGameDescription(entry)
		}

	case GamelistNone:
	}
}
