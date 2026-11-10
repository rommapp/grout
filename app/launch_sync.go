package main

import (
	"errors"
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

// The command line a frontend's hook runs around a game:
//
//	grout --sync-rom <rom>                   just before the game starts
//	grout --sync-rom <rom> --upload-only     once it has been quit
//	grout --sync-rom <rom> --now             because the player asked, whatever
//	                                         Sync on Launch is set to
const (
	syncRomFlag    = "--sync-rom"
	uploadOnlyFlag = "--upload-only"
	nowFlag        = "--now"
)

// What a launch sync exits with. The hooks that run it before and after a game
// ignore all of these; the menu entry that runs it on request turns them into a
// message.
const (
	exitDone          = 0  // nothing needed doing, or nothing was asked for
	exitError         = 1  // no usable config or cache
	exitDownloaded    = 10 // a newer save came down
	exitUploaded      = 11 // the save on the device went up
	exitUnreachable   = 12 // RomM did not answer in time
	exitNotMatched    = 13 // the game is not one RomM knows about
	exitHeld          = 14 // a conflict or slot choice needs a manual sync
	exitNoDevice      = 15 // this device is not registered with RomM
	exitStateSetAside = 16 // a newer save came down and the resume state made way for it
	exitSavesInDoubt  = 17 // the game has saves in more than one core folder
)

// launchResolveBudget bounds the read-only part of a launch sync. The game is
// waiting behind it, and an unreachable server should cost seconds, not the
// API timeout of every request.
const launchResolveBudget = 20 * time.Second

// launchSync is a launch sync as asked for on the command line.
type launchSync struct {
	romPath    string
	uploadOnly bool
	now        bool
}

// parseLaunchSync reads the command line, and reports whether it asked for a
// launch sync at all.
func parseLaunchSync(args []string) (launchSync, bool) {
	if len(args) < 2 || args[0] != syncRomFlag || args[1] == "" {
		return launchSync{}, false
	}
	request := launchSync{romPath: args[1]}
	for _, arg := range args[2:] {
		switch arg {
		case uploadOnlyFlag:
			request.uploadOnly = true
		case nowFlag:
			request.now = true
		default:
			return launchSync{}, false
		}
	}
	if request.uploadOnly && request.now {
		return launchSync{}, false
	}
	return request, true
}

// wanted reports whether the Sync on Launch setting asks for this sync. A sync
// the player asked for by hand is always wanted.
func (r launchSync) wanted(mode settings.LaunchSyncMode) bool {
	switch {
	case r.now:
		return true
	case r.uploadOnly:
		return mode.AfterQuit()
	default:
		return mode.BeforeLaunch()
	}
}

// skipsWhenRecent reports whether this sync should stand down because the game
// was synced a moment ago. Only the sync run as a game starts does: one the
// player asked for always runs, and so does the upload after a game, which
// carries what was just played.
func (r launchSync) skipsWhenRecent() bool { return !r.now && !r.uploadOnly }

// launchLogLevel is the level a launch sync logs at. The default log level
// keeps only errors, which would leave nothing to read when a sync does not do
// what was expected, so a launch sync always logs at least what it did.
func launchLogLevel(configured settings.LogLevel) string {
	if configured == settings.LogLevelError || configured == "" {
		return string(settings.LogLevelInfo)
	}
	return string(configured)
}

// runLaunchSync syncs one game's save without bringing up the interface and
// returns the process exit code. It never blocks a launch: every way it can
// fail or have nothing to do is logged and left to the caller to ignore.
func runLaunchSync(request launchSync) int {
	romPath := request.romPath
	logger := gaba.GetLogger()
	started := time.Now()

	if _, err := cfw.Active(); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot sync: %v\n", err)
		return exitError
	}

	config, err := settings.LoadConfig()
	if err != nil {
		logger.Error("Launch sync: no usable config", "error", err)
		return exitError
	}
	gaba.SetRawLogLevel(launchLogLevel(config.LogLevel))
	os.Setenv(settings.RomStorageEnvVar, string(config.MuOSRomStorage))

	if !request.wanted(config.SyncOnLaunch) {
		logger.Debug("Launch sync: turned off", "mode", config.SyncOnLaunch, "uploadOnly", request.uploadOnly)
		return exitDone
	}
	if len(config.Hosts) == 0 || config.Hosts[0].DeviceID == "" {
		logger.Info("Launch sync: no registered device")
		return exitNoDevice
	}
	host := config.Hosts[0]

	recent := saves.LoadRecentSyncs()
	if request.skipsWhenRecent() {
		if age, ok := recent.SyncedRecently(romPath, started); ok {
			logger.Info("Launch sync: skipped, synced recently", "rom", romPath, "ago", age.Round(time.Second))
			return exitDone
		}
	}

	if err := cache.InitCacheManager(host, *config); err != nil {
		logger.Error("Launch sync: cache unavailable", "error", err)
		return exitError
	}
	defer func() {
		if cm := cache.GetCacheManager(); cm != nil {
			cm.Close()
		}
	}()

	probe := romm.NewClientFromHost(host, settings.ValidationTimeout)
	if _, err := probe.GetHeartbeat(); err != nil {
		logger.Info("Launch sync: server unreachable", "error", err, "elapsed", time.Since(started).Round(time.Millisecond))
		return exitUnreachable
	}
	reached := time.Since(started)

	client := romm.NewClientFromHost(host, config.ApiTimeout.Duration())

	type resolution struct {
		plan saves.LaunchPlan
		err  error
	}
	done := make(chan resolution, 1)
	go func() {
		plan, err := saves.ResolveRomOnLaunch(client, config, host.DeviceID, romPath, request.uploadOnly)
		done <- resolution{plan, err}
	}()

	var resolved resolution
	select {
	case resolved = <-done:
	case <-time.After(launchResolveBudget):
		// Nothing has been written yet, so walking away is safe.
		logger.Warn("Launch sync: gave up waiting for the server", "rom", romPath)
		return exitUnreachable
	}
	if errors.Is(resolved.err, saves.ErrAmbiguousSaves) {
		return exitSavesInDoubt
	}
	if errors.Is(resolved.err, saves.ErrRomNotMatched) {
		logger.Info("Launch sync: nothing to do", "rom", romPath, "reason", resolved.err)
		return exitNotMatched
	}
	if resolved.err != nil {
		logger.Info("Launch sync: could not work out what to do", "rom", romPath, "error", resolved.err)
		return exitUnreachable
	}
	plan := resolved.plan
	planned := time.Since(started)

	// From here on it runs to the end: a download interrupted partway is worse
	// than one that makes the game wait.
	report := saves.ExecuteSaveSync(client, config, host.DeviceID, plan.Items, plan.SessionID, nil)
	logger.Info("Launch sync finished", "rom", romPath,
		"uploaded", report.Uploaded, "downloaded", report.Downloaded,
		"skipped", report.Skipped, "errors", report.Errors, "held", plan.Held,
		"reached_server", reached.Round(time.Millisecond),
		"planned", planned.Round(time.Millisecond),
		"elapsed", time.Since(started).Round(time.Millisecond))

	if report.Errors == 0 {
		recent.RecordRom(romPath, time.Now())
		if err := recent.Save(time.Now()); err != nil {
			logger.Warn("Launch sync: could not note that the game was synced", "error", err)
		}
	}

	if report.Downloaded > 0 {
		// The game would resume from a snapshot of the progress this save
		// replaces, and the new save would never be seen.
		moved, err := cfw.ActiveFirmware().SetAsideResumeState(romPath)
		switch {
		case err != nil:
			logger.Error("Launch sync: could not set the resume state aside", "rom", romPath, "error", err)
		case moved > 0:
			logger.Info("Launch sync: set the resume state aside for the newer save", "rom", romPath, "files", moved)
			return exitStateSetAside
		}
		return exitDownloaded
	}
	if report.Uploaded > 0 {
		return exitUploaded
	}
	if plan.Held > 0 {
		return exitHeld
	}
	return exitDone
}
