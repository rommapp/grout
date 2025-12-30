package utils

import (
	"grout/constants"
	"grout/update"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const updateIcon = "\U000F06B0"

type AutoUpdate struct {
	cfw             constants.CFW
	icon            *gaba.DynamicStatusBarIcon
	running         atomic.Bool
	updateAvailable atomic.Bool
	done            chan struct{}
	updateInfo      *update.Info
}

func NewAutoUpdate(cfw constants.CFW) *AutoUpdate {
	return &AutoUpdate{
		cfw:  cfw,
		done: make(chan struct{}),
	}
}

func (a *AutoUpdate) Icon() gaba.StatusBarIcon {
	if a.icon == nil {
		return gaba.StatusBarIcon{}
	}
	return gaba.StatusBarIcon{
		Dynamic: a.icon,
	}
}

func (a *AutoUpdate) Start() {
	a.running.Store(true)
	a.done = make(chan struct{})
	go a.run()
}

func (a *AutoUpdate) IsRunning() bool {
	return a.running.Load()
}

func (a *AutoUpdate) UpdateAvailable() bool {
	return a.updateAvailable.Load()
}

func (a *AutoUpdate) UpdateInfo() *update.Info {
	return a.updateInfo
}

func (a *AutoUpdate) run() {
	logger := gaba.GetLogger()
	defer func() {
		a.running.Store(false)
		close(a.done)
	}()

	logger.Debug("AutoUpdate: Checking for updates in background")

	info, err := update.CheckForUpdate(a.cfw)
	if err != nil {
		logger.Debug("AutoUpdate: Failed to check for updates", "error", err)
		return
	}

	a.updateInfo = info

	if info.UpdateAvailable {
		logger.Debug("AutoUpdate: Update available", "current", info.CurrentVersion, "latest", info.LatestVersion)
		a.updateAvailable.Store(true)
		a.icon = gaba.NewDynamicStatusBarIcon(updateIcon)
		AddIcon(a.Icon())
	} else {
		logger.Debug("AutoUpdate: Already up to date", "version", info.CurrentVersion)
	}
}
