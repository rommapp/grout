package main

import (
	"grout/cfw"
	"grout/update"
	"os"

	_ "github.com/BrandonKowalski/certifiable"
	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func main() {
	defer cleanup()

	result := setup()
	config := result.Config
	platforms := result.Platforms

	logger := gaba.GetLogger()
	logger.Debug("Starting Grout")

	currentCFW := cfw.GetCFW()
	quitOnBack := len(config.Hosts) == 1
	showCollections := config.ShowCollections(config.Hosts[0])

	if err := runWithRouter(config, currentCFW, platforms, quitOnBack, showCollections); err != nil {
		logger.Error("Router error", "error", err)
	}
}

func cleanup() {
	if err := os.RemoveAll(".tmp"); err != nil {
		gaba.GetLogger().Error("Failed to clean .tmp directory", "error", err)
	}
	update.CleanupUpdateArtifacts()
	gaba.Close()
}
