package cache

import (
	"grout/romm"
	"sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const (
	iconSynced  = "\U000F0AA9"
	iconSyncing = "\U000F0CFF"
	iconAlert   = "\U000F163A"
)

type syncType int

const (
	syncFull syncType = iota
	syncCollectionsOnly
	syncPlatformsOnly
)

type syncRequest struct {
	Type      syncType
	Platforms []romm.Platform
}

type BackgroundSync struct {
	platforms []romm.Platform
	icon      *gaba.DynamicStatusBarIcon
	requests  chan syncRequest
	stop      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	running   bool
}

func NewBackgroundSync(platforms []romm.Platform) *BackgroundSync {
	return &BackgroundSync{
		platforms: platforms,
		icon:      gaba.NewDynamicStatusBarIcon(iconSyncing),
		requests:  make(chan syncRequest, 1),
		stop:      make(chan struct{}),
	}
}

func (b *BackgroundSync) Icon() gaba.StatusBarIcon {
	return gaba.StatusBarIcon{
		Dynamic: b.icon,
	}
}

func (b *BackgroundSync) Start() {
	if b.ensureWorkerRunning() {
		b.queueSync(syncRequest{Type: syncFull})
	}
}

func (b *BackgroundSync) Restart() {
	b.ensureWorkerRunning()
	b.queueSync(syncRequest{Type: syncFull})
}

func (b *BackgroundSync) SyncCollections() {
	b.ensureWorkerRunning()
	b.queueSync(syncRequest{Type: syncCollectionsOnly})
}

func (b *BackgroundSync) SyncPlatforms(platforms []romm.Platform) {
	b.ensureWorkerRunning()
	b.queueSync(syncRequest{Type: syncPlatformsOnly, Platforms: platforms})
}

// ensureWorkerRunning starts the worker if not running. Returns true if worker was started.
func (b *BackgroundSync) ensureWorkerRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return false
	}

	b.running = true
	b.requests = make(chan syncRequest, 1)
	b.stop = make(chan struct{})
	b.wg.Add(1)
	go b.worker()
	return true
}

func (b *BackgroundSync) queueSync(req syncRequest) {
	select {
	case b.requests <- req:
		// Queued
	default:
		// Already queued, skip
	}
}

func (b *BackgroundSync) IsRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *BackgroundSync) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	close(b.stop)
	b.mu.Unlock()

	gaba.GetLogger().Debug("BackgroundSync: Stop requested")
}

func (b *BackgroundSync) SetSynced() {
	b.icon.SetText(iconSynced)
}

func (b *BackgroundSync) worker() {
	logger := gaba.GetLogger()
	defer b.wg.Done()

	for {
		select {
		case <-b.stop:
			logger.Debug("BackgroundSync: Worker stopped")
			return
		case req := <-b.requests:
			b.runSync(req)
		}
	}
}

func (b *BackgroundSync) runSync(req syncRequest) {
	logger := gaba.GetLogger()

	defer func() {
		if r := recover(); r != nil {
			logger.Error("BackgroundSync: Panic recovered", "panic", r)
			b.icon.SetText(iconAlert)
		}
	}()

	// Check if stopped
	select {
	case <-b.stop:
		return
	default:
	}

	b.icon.SetText(iconSyncing)

	cm := GetCacheManager()
	if cm == nil {
		logger.Error("BackgroundSync: Cache manager not initialized")
		b.icon.SetText(iconAlert)
		return
	}

	var err error

	switch req.Type {
	case syncCollectionsOnly:
		logger.Debug("BackgroundSync: Starting collections-only sync")
		_, err = cm.SyncCollectionsOnly()

	case syncPlatformsOnly:
		logger.Debug("BackgroundSync: Starting platform games sync", "platforms", len(req.Platforms))
		_, err = cm.SyncPlatformGames(req.Platforms)

	default:
		logger.Debug("BackgroundSync: Starting full cache update")
		_, err = cm.PopulateFullCacheWithProgress(b.platforms, nil)

		// After full sync, retry any platforms that previously failed
		if err == nil {
			needSync := cm.GetPlatformsNeedingSync(b.platforms)
			if len(needSync) > 0 {
				logger.Debug("BackgroundSync: Retrying failed platforms", "count", len(needSync))
				cm.SyncPlatformGames(needSync)
			}
		}
	}

	// Check if we were stopped mid-sync
	select {
	case <-b.stop:
		return
	default:
	}

	if err != nil {
		logger.Error("BackgroundSync: Sync failed", "error", err)
		b.icon.SetText(iconAlert)
		return
	}

	b.icon.SetText(iconSynced)
	logger.Debug("BackgroundSync: Sync completed")
}
