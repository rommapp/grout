package main

import (
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"grout/sync"
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

	AutoSync   *sync.AutoSync
	AutoUpdate *update.AutoUpdate
	CacheSync  *cache.BackgroundSync

	autoSyncOnce   gosync.Once
	autoUpdateOnce gosync.Once
}

func computeShowSaveSync(state *AppState) *atomic.Bool {
	switch state.Config.SaveSyncMode {
	case internal.SaveSyncModeManual:
		showSaveSync := &atomic.Bool{}
		showSaveSync.Store(true)
		return showSaveSync
	case internal.SaveSyncModeAutomatic:
		if state.AutoSync != nil {
			return state.AutoSync.ShowButton()
		}
	}
	return nil
}
