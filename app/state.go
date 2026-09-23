package main

import (
	"grout/cache"
	"grout/cfw"
	"grout/romm"
	"grout/settings"
	"grout/update"
	gosync "sync"
	"sync/atomic"
)

var currentAppState *AppState

type AppState struct {
	Config    *settings.Config
	Host      settings.Host
	CFW       cfw.CFW
	Platforms []romm.Platform

	RommVersion atomic.Value // string

	AutoUpdate *update.AutoUpdate
	CacheSync  *cache.BackgroundSync

	autoUpdateOnce gosync.Once
}
