package main

import (
	"grout/utils"
	"os"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	_ "github.com/UncleJunVIP/certifiable"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

func main() {
	defer cleanup()

	result := setup()
	config := result.Config
	platforms := result.Platforms

	logger := gaba.GetLogger()
	logger.Debug("Starting Grout")

	cfw := utils.GetCFW()
	quitOnBack := len(config.Hosts) == 1
	showCollections := utils.ShowCollections(config, config.Hosts[0])

	fsm := buildFSM(config, cfw, platforms, quitOnBack, showCollections)

	if err := fsm.Run(); err != nil {
		logger.Error("FSM error", "error", err)
	}
}

func cleanup() {
	if autoSync != nil && autoSync.IsRunning() {
		gaba.GetLogger().Info("Waiting for auto-sync to complete before exiting...")
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "auto_sync_waiting", Other: "Waiting for save sync to complete..."}, nil),
			gaba.ProcessMessageOptions{},
			func() (interface{}, error) {
				autoSync.Wait()
				return nil, nil
			},
		)
	}

	if err := os.RemoveAll(".tmp"); err != nil {
		gaba.GetLogger().Error("Failed to clean .tmp directory", "error", err)
	}
	gaba.Close()
}
