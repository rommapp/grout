package cache

import (
	"grout/romm"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"go.uber.org/atomic"
)

const (
	DefaultRomPageSize           = 200
	MaxConcurrentPlatformFetches = 5
)

type SyncStats struct {
	Platforms         int
	GamesUpdated      int
	Collectionssynced int
}

func (cm *Manager) populateCache(platforms []romm.Platform, progress *atomic.Float64) (SyncStats, error) {
	logger := gaba.GetLogger()
	stats := SyncStats{Platforms: len(platforms)}

	if len(platforms) == 0 {
		if progress != nil {
			progress.Store(1.0)
		}
		return stats, nil
	}

	// Get the last refresh time to use for incremental updates
	// Only use incremental update if cache has games, otherwise do full refresh
	var updatedAfter string
	if cm.HasCache() {
		if lastRefresh, err := cm.GetLastRefreshTime(MetaKeyGamesRefreshedAt); err == nil {
			updatedAfter = lastRefresh.Format(time.RFC3339)
			logger.Debug("Using incremental cache update", "updated_after", updatedAfter)
		}

		// Fetch only updated platforms if we have a previous refresh time
		if platformsRefresh, err := cm.GetLastRefreshTime(MetaKeyPlatformsRefreshedAt); err == nil {
			client := romm.NewClientFromHost(cm.host, cm.config.GetApiTimeout())
			updatedPlatforms, err := client.GetPlatforms(romm.GetPlatformsQuery{UpdatedAfter: platformsRefresh.Format(time.RFC3339)})
			if err != nil {
				logger.Error("Failed to fetch updated platforms", "error", err)
			} else {
				if len(updatedPlatforms) > 0 {
					if err := cm.SavePlatforms(updatedPlatforms); err != nil {
						logger.Error("Failed to save updated platforms", "error", err)
					} else {
						logger.Debug("Saved updated platforms", "count", len(updatedPlatforms))
					}
				}
				cm.RecordRefreshTime(MetaKeyPlatformsRefreshedAt)
			}
		} else {
			// No previous platforms refresh time - record it now for future incremental syncs
			cm.RecordRefreshTime(MetaKeyPlatformsRefreshedAt)
		}
	} else {
		// Save all platforms on first run / empty cache
		// Fetch all platforms from API, not just mapped ones
		client := romm.NewClientFromHost(cm.host, cm.config.GetApiTimeout())
		allPlatforms, err := client.GetPlatforms()
		if err != nil {
			logger.Error("Failed to fetch all platforms", "error", err)
			// Fall back to saving just the mapped platforms
			if err := cm.SavePlatforms(platforms); err != nil {
				return stats, err
			}
		} else {
			if err := cm.SavePlatforms(allPlatforms); err != nil {
				return stats, err
			}
		}
		cm.RecordRefreshTime(MetaKeyPlatformsRefreshedAt)
	}

	totalExpectedGames := int64(0)
	for _, p := range platforms {
		totalExpectedGames += int64(p.ROMCount)
	}
	if totalExpectedGames == 0 {
		totalExpectedGames = int64(len(platforms))
	}

	gamesFetched := &atomic.Int64{}
	updateProgress := func(count int) {
		if progress != nil {
			fetched := gamesFetched.Add(int64(count))
			// Cap at 90% for games phase, reserve 10% for collections
			pct := float64(fetched) / float64(totalExpectedGames) * 0.9
			if pct > 0.9 {
				pct = 0.9
			}
			progress.Store(pct)
		}
	}

	sem := make(chan struct{}, MaxConcurrentPlatformFetches)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for _, platform := range platforms {
		wg.Add(1)
		go func(p romm.Platform) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := cm.fetchPlatformGames(p, &fetchOpts{onProgress: updateProgress, updatedAfter: updatedAfter}); err != nil {
				logger.Error("Failed to cache platform", "platform", p.Name, "error", err)
				cm.RecordPlatformSyncFailure(p.ID)
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			} else {
				cm.RecordPlatformSyncSuccess(p.ID, p.ROMCount)
			}
		}(platform)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		cm.fetchBIOSAvailability(platforms)
	}()

	wg.Wait()

	if firstErr == nil {
		cm.RecordRefreshTime(MetaKeyGamesRefreshedAt)
		cm.PurgeStaleFilenameMappings()
	}

	stats.Collectionssynced = cm.fetchAndCacheCollectionsWithProgress(progress)

	cm.RecordRefreshTime(MetaKeyCollectionsRefreshedAt)

	if progress != nil {
		progress.Store(1.0)
	}

	stats.GamesUpdated = int(gamesFetched.Load())
	logger.Debug("Cache population completed", "platforms", stats.Platforms, "games", stats.GamesUpdated)
	return stats, firstErr
}

type fetchOpts struct {
	onProgress    func(count int) // Called with count of games fetched (for batch progress)
	onPctProgress *atomic.Float64 // Set with percentage 0.0-1.0 (for UI progress bars)
	updatedAfter  string
}

func (cm *Manager) fetchPlatformGames(platform romm.Platform, opts *fetchOpts) error {
	if opts == nil {
		opts = &fetchOpts{}
	}

	logger := gaba.GetLogger()
	client := romm.NewClientFromHost(cm.host, cm.config.GetApiTimeout())

	var allGames []romm.Rom
	offset := 0
	expectedTotal := 0

	for {
		q := romm.GetRomsQuery{
			PlatformID:   platform.ID,
			Offset:       offset,
			Limit:        DefaultRomPageSize,
			UpdatedAfter: opts.updatedAfter,
		}

		res, err := client.GetRoms(q)
		if err != nil {
			logger.Error("Failed to fetch games",
				"platform", platform.Name,
				"offset", offset,
				"error", err)
			return err
		}

		if offset == 0 {
			expectedTotal = res.Total
		}

		allGames = append(allGames, res.Items...)

		if opts.onProgress != nil && len(res.Items) > 0 {
			opts.onProgress(len(res.Items))
		}
		if opts.onPctProgress != nil && expectedTotal > 0 {
			pct := float64(len(allGames)) / float64(expectedTotal)
			if pct > 1.0 {
				pct = 1.0
			}
			opts.onPctProgress.Store(pct)
		}

		if len(allGames) >= expectedTotal || len(res.Items) == 0 || len(res.Items) < DefaultRomPageSize {
			break
		}

		offset += len(res.Items)
	}

	if opts.updatedAfter != "" {
		logger.Debug("Fetched updated platform games",
			"platform", platform.Name,
			"count", len(allGames),
			"updated_after", opts.updatedAfter)
	} else {
		logger.Debug("Cached platform games",
			"platform", platform.Name,
			"count", len(allGames))
	}

	return cm.SavePlatformGames(platform.ID, allGames)
}

func (cm *Manager) fetchAndCacheCollectionsWithProgress(progress *atomic.Float64) int {
	logger := gaba.GetLogger()

	showRegular := cm.config.GetShowCollections()
	showSmart := cm.config.GetShowSmartCollections()
	showVirtual := cm.config.GetShowVirtualCollections()

	if !showRegular && !showSmart && !showVirtual {
		logger.Debug("Skipping collection sync - no collection types enabled")
		if progress != nil {
			progress.Store(0.98)
		}
		return 0
	}

	client := romm.NewClientFromHost(cm.host, cm.config.GetApiTimeout())

	var updatedAfter string
	if lastRefresh, err := cm.GetLastRefreshTime(MetaKeyCollectionsRefreshedAt); err == nil {
		updatedAfter = lastRefresh.Format(time.RFC3339)
		logger.Debug("Using incremental collection update", "updated_after", updatedAfter)
	}

	var query romm.GetCollectionsQuery
	if updatedAfter != "" {
		query = romm.GetCollectionsQuery{UpdatedAfter: updatedAfter}
	}

	var allCollections []romm.Collection
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Calculate progress increment per collection type (90% to 92% range)
	enabledCount := 0
	if showRegular {
		enabledCount++
	}
	if showSmart {
		enabledCount++
	}
	if showVirtual {
		enabledCount++
	}
	progressPerType := 0.02 / float64(enabledCount) // 2% divided among enabled types
	completed := &atomic.Int32{}

	updateFetchProgress := func() {
		if progress != nil {
			done := completed.Add(1)
			progress.Store(0.90 + float64(done)*progressPerType)
		}
	}

	if showRegular {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collections, err := client.GetCollections(query)
			if err != nil {
				logger.Error("Failed to fetch regular collections", "error", err)
				updateFetchProgress()
				return
			}
			mu.Lock()
			allCollections = append(allCollections, collections...)
			mu.Unlock()
			updateFetchProgress()
		}()
	}

	if showSmart {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collections, err := client.GetSmartCollections(query)
			if err != nil {
				logger.Error("Failed to fetch smart collections", "error", err)
				updateFetchProgress()
				return
			}
			for i := range collections {
				collections[i].IsSmart = true
			}
			mu.Lock()
			allCollections = append(allCollections, collections...)
			mu.Unlock()
			updateFetchProgress()
		}()
	}

	if showVirtual {
		wg.Add(1)
		go func() {
			defer wg.Done()
			virtualCollections, err := client.GetVirtualCollections()
			if err != nil {
				logger.Error("Failed to fetch virtual collections", "error", err)
				updateFetchProgress()
				return
			}
			mu.Lock()
			for _, vc := range virtualCollections {
				allCollections = append(allCollections, vc.ToCollection())
			}
			mu.Unlock()
			updateFetchProgress()
		}()
	}

	wg.Wait()

	if len(allCollections) == 0 {
		return 0
	}

	if err := cm.SaveCollections(allCollections); err != nil {
		logger.Error("Failed to save collections", "error", err)
	}

	if progress != nil {
		progress.Store(0.94)
	}

	if err := cm.SaveAllCollectionMappings(allCollections); err != nil {
		logger.Error("Failed to save collection mappings", "error", err)
	}

	if progress != nil {
		progress.Store(0.98)
	}

	logger.Debug("Cached collections", "count", len(allCollections))
	return len(allCollections)
}

func (cm *Manager) fetchBIOSAvailability(platforms []romm.Platform) {
	logger := gaba.GetLogger()

	client := romm.NewClientFromHost(cm.host, cm.config.GetApiTimeout())

	var wg sync.WaitGroup
	sem := make(chan struct{}, MaxConcurrentPlatformFetches)

	for _, platform := range platforms {
		wg.Add(1)
		go func(p romm.Platform) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			firmware, err := client.GetFirmware(p.ID)
			if err != nil {
				logger.Debug("Failed to fetch BIOS info", "platform", p.Name, "error", err)
				cm.SetBIOSAvailability(p.ID, false)
				return
			}

			hasBIOS := len(firmware) > 0
			cm.SetBIOSAvailability(p.ID, hasBIOS)
		}(platform)
	}

	wg.Wait()
}

func (cm *Manager) RefreshPlatformGames(platform romm.Platform) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	return cm.fetchPlatformGames(platform, nil)
}

func (cm *Manager) RefreshPlatformGamesWithProgress(platform romm.Platform, progress *atomic.Float64) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	var updatedAfter string
	if lastRefresh, err := cm.GetLastRefreshTime(MetaKeyGamesRefreshedAt); err == nil {
		updatedAfter = lastRefresh.Format(time.RFC3339)
		gaba.GetLogger().Debug("Using incremental refresh", "updated_after", updatedAfter)
	}

	err := cm.fetchPlatformGames(platform, &fetchOpts{
		onPctProgress: progress,
		updatedAfter:  updatedAfter,
	})

	if progress != nil {
		progress.Store(1.0)
	}

	return err
}
