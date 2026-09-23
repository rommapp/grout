package update

import (
	"grout/cfw"
	"grout/settings"
	"sync"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const updateIcon = "\U000F06B0"

type AutoUpdate struct {
	cfwType         cfw.CFW
	host            *settings.Host
	icon            *gaba.DynamicStatusBarIcon
	running         atomic.Bool
	updateAvailable atomic.Bool
	mu              sync.Mutex
	releaseChannel  settings.ReleaseChannel
	updateInfo      atomic.Pointer[Info]
}

func NewAutoUpdate(c cfw.CFW, r settings.ReleaseChannel, host *settings.Host) *AutoUpdate {
	return &AutoUpdate{
		cfwType:        c,
		releaseChannel: r,
		host:           host,
		icon:           gaba.NewDynamicStatusBarIcon(""),
	}
}

func (a *AutoUpdate) Icon() gaba.StatusBarIcon {
	return gaba.StatusBarIcon{
		Dynamic: a.icon,
	}
}

func (a *AutoUpdate) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running.Load() {
		return
	}
	a.running.Store(true)
	go a.run()
}

func (a *AutoUpdate) IsRunning() bool {
	return a.running.Load()
}

func (a *AutoUpdate) UpdateAvailable() bool {
	return a.updateAvailable.Load()
}

func (a *AutoUpdate) UpdateInfo() *Info {
	return a.updateInfo.Load()
}

// Recheck updates the release channel and re-runs the update check.
// This should be called when the user changes the release channel in settings.
func (a *AutoUpdate) Recheck(releaseChannel settings.ReleaseChannel) {
	if a.running.Load() {
		return // Already running, skip
	}

	a.mu.Lock()
	a.releaseChannel = releaseChannel
	a.mu.Unlock()

	a.updateAvailable.Store(false)
	a.updateInfo.Store(nil)
	a.icon.SetText("") // Clear the icon

	a.Start()
}

func (a *AutoUpdate) run() {
	logger := gaba.GetLogger()
	defer a.running.Store(false)

	logger.Debug("AutoUpdate: Checking for updates in background")

	a.mu.Lock()
	channel := a.releaseChannel
	a.mu.Unlock()

	info, err := CheckForUpdate(a.cfwType, channel, a.host)
	if err != nil {
		logger.Debug("AutoUpdate: Failed to check for updates", "error", err)
		return
	}

	a.updateInfo.Store(info)

	if info.UpdateAvailable {
		logger.Debug("AutoUpdate: Update available", "current", info.CurrentVersion, "latest", info.LatestVersion)
		a.updateAvailable.Store(true)
		a.icon.SetText(updateIcon)
	} else {
		logger.Debug("AutoUpdate: Already up to date", "version", info.CurrentVersion)
	}
}
