package sync

import (
	"grout/internal"
	"grout/romm"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type AutoSync struct {
	host       romm.Host
	config     *internal.Config
	icon       *gaba.DynamicStatusBarIcon
	running    atomic.Bool
	done       chan struct{}
	showButton atomic.Bool
}

func NewAutoSync(host romm.Host, config *internal.Config) *AutoSync {
	return &AutoSync{
		host:   host,
		config: config,
		icon:   gaba.NewDynamicStatusBarIcon(icons.CloudRefresh),
		done:   make(chan struct{}),
	}
}

func (a *AutoSync) Icon() gaba.StatusBarIcon {
	return gaba.StatusBarIcon{
		Dynamic: a.icon,
	}
}

func (a *AutoSync) Start() {
	a.running.Store(true)
	a.done = make(chan struct{}) // Reinitialize channel for reuse
	go a.run()
}

func (a *AutoSync) IsRunning() bool {
	return a.running.Load()
}

func (a *AutoSync) Wait() {
	<-a.done
}

func (a *AutoSync) ShowButton() *atomic.Bool {
	return &a.showButton
}

// Trigger starts a new sync if one isn't already running.
// Returns true if a new sync was started, false if one is already in progress.
func (a *AutoSync) Trigger() bool {
	if a.running.Load() {
		return false
	}
	a.Start()
	return true
}

func (a *AutoSync) Host() romm.Host {
	return a.host
}

func (a *AutoSync) run() {
	logger := gaba.GetLogger()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("AutoSync: Panic recovered", "panic", r)
			a.icon.SetText(icons.CloudAlert)
		}
		a.running.Store(false)
		close(a.done)
	}()

	a.icon.SetText(icons.CloudRefresh)
	logger.Debug("AutoSync: Starting save sync scan")

	syncs, unmatched, _, err := FindSaveSyncs(a.host, a.config)
	if err != nil {
		logger.Error("AutoSync: Failed to find save syncs", "error", err)
		a.icon.SetText(icons.CloudAlert)
		return
	}

	if len(syncs) == 0 {
		if len(unmatched) > 0 {
			a.icon.SetText(icons.CloudAlert)
			logger.Debug("AutoSync: No syncs needed but has unmatched saves", "unmatched", len(unmatched))
		} else {
			a.icon.SetText(icons.CloudCheck)
			logger.Debug("AutoSync: No syncs needed")
		}
		return
	}

	logger.Debug("AutoSync: Found syncs", "count", len(syncs))

	hadError := false

	for i := range syncs {
		s := &syncs[i]

		switch s.Action {
		case Upload:
			a.icon.SetText(icons.CloudUpload)
			logger.Debug("AutoSync: Uploading", "game", s.GameBase)
		case Download:
			a.icon.SetText(icons.CloudDownload)
			logger.Debug("AutoSync: Downloading", "game", s.GameBase)
		case Skip:
			continue
		}

		result := s.Execute(a.host, a.config)
		if !result.Success {
			logger.Error("AutoSync: Sync failed", "game", s.GameBase, "error", result.Error)
			hadError = true
		} else {
			logger.Debug("AutoSync: Sync successful", "game", s.GameBase, "action", result.Action)
		}
	}

	if hadError || len(unmatched) > 0 {
		a.icon.SetText(icons.CloudAlert)
		if hadError {
			logger.Debug("AutoSync: Completed with errors")
		} else {
			logger.Debug("AutoSync: Completed with unmatched saves", "unmatched", len(unmatched))
		}
	} else {
		a.icon.SetText(icons.CloudCheck)
		logger.Debug("AutoSync: Completed successfully")
	}
}
