package main

import (
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"grout/update"
	gosync "sync"
	"sync/atomic"
)

var currentAppState *AppState

type AppState struct {
	Config    *internal.Config
	Host      romm.Host
	CFW       cfw.CFW
	Platforms []romm.Platform

	RommVersion atomic.Value // string

	AutoUpdate *update.AutoUpdate
	CacheSync  *cache.BackgroundSync

	autoUpdateOnce gosync.Once
}

func computeShowSaveSync(state *AppState) *atomic.Bool {
	if state.Config.SaveSyncMode == internal.SaveSyncModeManual {
		showSaveSync := &atomic.Bool{}
		showSaveSync.Store(true)
		return showSaveSync
	}
	return nil
}
