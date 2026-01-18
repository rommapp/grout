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
		if err := cm.SavePlatforms(platforms); err != nil {
			return stats, err
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
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
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
	onProgress   func(count int)
	updatedAfter string
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		collections, err := client.GetCollections(query)
		if err != nil {
			logger.Error("Failed to fetch regular collections", "error", err)
			return
		}
		mu.Lock()
		allCollections = append(allCollections, collections...)
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		collections, err := client.GetSmartCollections(query)
		if err != nil {
			logger.Error("Failed to fetch smart collections", "error", err)
			return
		}
		for i := range collections {
			collections[i].IsSmart = true
		}
		mu.Lock()
		allCollections = append(allCollections, collections...)
		mu.Unlock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		virtualCollections, err := client.GetVirtualCollections()
		if err != nil {
			logger.Error("Failed to fetch virtual collections", "error", err)
			return
		}
		mu.Lock()
		for _, vc := range virtualCollections {
			allCollections = append(allCollections, vc.ToCollection())
		}
		mu.Unlock()
	}()

	wg.Wait()

	// Update progress to 92% after fetching collection metadata, arbitrary I know
	if progress != nil {
		progress.Store(0.92)
	}

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

	logger := gaba.GetLogger()
	client := romm.NewClientFromHost(cm.host, cm.config.GetApiTimeout())

	// Get the last refresh time for incremental updates
	var updatedAfter string
	if lastRefresh, err := cm.GetLastRefreshTime(MetaKeyGamesRefreshedAt); err == nil {
		updatedAfter = lastRefresh.Format(time.RFC3339)
		logger.Debug("Using incremental refresh", "updated_after", updatedAfter)
	}

	var allGames []romm.Rom
	offset := 0
	expectedTotal := 0

	for {
		opt := romm.GetRomsQuery{
			PlatformID:   platform.ID,
			Offset:       offset,
			Limit:        DefaultRomPageSize,
			UpdatedAfter: updatedAfter,
		}

		res, err := client.GetRoms(opt)
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

		if progress != nil && expectedTotal > 0 {
			pct := float64(len(allGames)) / float64(expectedTotal)
			if pct > 1.0 {
				pct = 1.0
			}
			progress.Store(pct)
		}

		// Terminate when: got all expected, empty batch, or partial page (last batch)
		if len(allGames) >= expectedTotal || len(res.Items) == 0 || len(res.Items) < DefaultRomPageSize {
			break
		}

		offset += len(res.Items)
	}

	if updatedAfter != "" {
		logger.Info("Refreshed platform games (incremental)",
			"platform", platform.Name,
			"count", len(allGames),
			"updated_after", updatedAfter)
	} else {
		logger.Info("Refreshed platform games",
			"platform", platform.Name,
			"count", len(allGames))
	}

	if err := cm.SavePlatformGames(platform.ID, allGames); err != nil {
		return err
	}

	if progress != nil {
		progress.Store(1.0)
	}

	return nil
}
