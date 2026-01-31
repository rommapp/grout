package cfw

import (
	"grout/cfw/knulli"
	"grout/cfw/rocknix"
	"grout/internal/emulationstation"
	"grout/internal/gamelist"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func scheduleESRestart() {
	err := emulationstation.ScheduleESRestart()
	if err != nil {
		gaba.GetLogger().Debug("Unable to schedule ES restart", "error", err)
	}
}

func AddGroutToGamelist(c CFW) {
	switch c {
	case Knulli:
		gamelist.AddGroutEntry(knulli.GetGroutGamelist(), "./Grout/Grout.sh")
	case ROCKNIX:
		gamelist.AddGroutEntry(rocknix.GetGroutGamelist(), "./Grout.sh")
	default:
		return
	}
	scheduleESRestart()
}
