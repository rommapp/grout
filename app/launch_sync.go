package main

import (
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/romm"
	"grout/saves"
	"grout/settings"
	"os"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// syncRomFlag asks for the one-shot sync a frontend's launch hook runs just
// before it starts a game: grout --sync-rom <path to the rom>.
const syncRomFlag = "--sync-rom"

// launchResolveBudget bounds the read-only part of a launch sync. The game is
// waiting behind it, and an unreachable server should cost seconds, not the
// API timeout of every request.
const launchResolveBudget = 20 * time.Second

// syncRomArg returns the ROM named on the command line, if the launch sync
// was asked for.
func syncRomArg(args []string) (string, bool) {
	if len(args) == 2 && args[0] == syncRomFlag && args[1] != "" {
		return args[1], true
	}
	return "", false
}

// runLaunchSync syncs one game's save without bringing up the interface and
// returns the process exit code. It never blocks a launch: every way it can
// fail or have nothing to do is logged and left to the launch script to ignore.
func runLaunchSync(romPath string) int {
	logger := gaba.GetLogger()

	if _, err := cfw.Active(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot sync: %v\n", err)
		return 1
	}

	config, err := settings.LoadConfig()
	if err != nil {
		logger.Error("Launch sync: no usable config", "error", err)
		return 1
	}
	gaba.SetRawLogLevel(string(config.LogLevel))
	os.Setenv(settings.RomStorageEnvVar, string(config.MuOSRomStorage))

	if !config.SyncOnLaunch {
		logger.Debug("Launch sync: turned off")
		return 0
	}
	if len(config.Hosts) == 0 || config.Hosts[0].DeviceID == "" {
		logger.Info("Launch sync: no registered device")
		return 0
	}
	host := config.Hosts[0]

	if err := cache.InitCacheManager(host, *config); err != nil {
		logger.Error("Launch sync: cache unavailable", "error", err)
		return 1
	}
	defer func() {
		if cm := cache.GetCacheManager(); cm != nil {
			cm.Close()
		}
	}()

	probe := romm.NewClientFromHost(host, settings.ValidationTimeout)
	if _, err := probe.GetHeartbeat(); err != nil {
		logger.Info("Launch sync: server unreachable", "error", err)
		return 0
	}

	client := romm.NewClientFromHost(host, config.ApiTimeout.Duration())

	type resolution struct {
		result saves.SyncResult
		err    error
	}
	done := make(chan resolution, 1)
	go func() {
		result, err := saves.ResolveRomOnLaunch(client, config, host.DeviceID, romPath)
		done <- resolution{result, err}
	}()

	var resolved resolution
	select {
	case resolved = <-done:
	case <-time.After(launchResolveBudget):
		// Nothing has been written yet, so walking away is safe.
		logger.Warn("Launch sync: gave up waiting for the server", "rom", romPath)
		return 0
	}
	if resolved.err != nil {
		logger.Info("Launch sync: nothing to do", "rom", romPath, "reason", resolved.err)
		return 0
	}

	// From here on it runs to the end: a download interrupted partway is worse
	// than one that makes the game wait.
	report := saves.ExecuteSaveSync(client, config, host.DeviceID, resolved.result.Items, resolved.result.SessionID, nil)
	logger.Info("Launch sync finished", "rom", romPath,
		"uploaded", report.Uploaded, "downloaded", report.Downloaded,
		"skipped", report.Skipped, "errors", report.Errors)
	return 0
}
