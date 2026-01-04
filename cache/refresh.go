package cache

import (
	"grout/romm"
	"os"
	"sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// Refresh handles startup cache validation, BIOS pre-fetching, and prefetching missing platforms/collections
type Refresh struct {
	host   romm.Host
	config Config

	freshnessCache map[string]bool
	freshnessMu    sync.RWMutex

	biosCache map[int]bool
	biosMu    sync.RWMutex

	prefetchInProgress map[string]chan struct{}
	prefetchMu         sync.RWMutex

	collections   []romm.Collection
	collectionsMu sync.RWMutex

	done    chan struct{}
	running bool
}

var (
	refreshInstance *Refresh
	refreshOnce     sync.Once
)

func GetRefresh() *Refresh {
	return refreshInstance
}

func InitRefresh(host romm.Host, config Config, platforms []romm.Platform) {
	refreshOnce.Do(func() {
		refreshInstance = &Refresh{
			host:               host,
			config:             config,
			freshnessCache:     make(map[string]bool),
			biosCache:          make(map[int]bool),
			prefetchInProgress: make(map[string]chan struct{}),
			done:               make(chan struct{}),
		}
		refreshInstance.start(platforms)
	})
}

func (c *Refresh) start(platforms []romm.Platform) {
	c.running = true
	go c.run(platforms)
}

func (c *Refresh) run(platforms []romm.Platform) {
	logger := gaba.GetLogger()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Refresh: Panic recovered", "panic", r)
		}
		c.running = false
		close(c.done)
	}()

	logger.Debug("Refresh: Starting background cache validation and prefetch")

	var wg sync.WaitGroup

	for _, platform := range platforms {
		wg.Add(1)
		go func(p romm.Platform) {
			defer wg.Done()
			c.fetchBIOSAvailability(p)
		}(platform)
	}

	for _, platform := range platforms {
		wg.Add(1)
		go func(p romm.Platform) {
			defer wg.Done()
			c.validateAndPrefetchPlatform(p)
		}(platform)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		c.fetchAndPrefetchCollections()
	}()

	wg.Wait()
	logger.Debug("Refresh: Completed background cache validation and prefetch",
		"platforms", len(platforms),
		"collections", len(c.collections),
		"freshness_entries", len(c.freshnessCache),
		"bios_entries", len(c.biosCache))
}

func (c *Refresh) validateAndPrefetchPlatform(platform romm.Platform) {
	logger := gaba.GetLogger()
	cacheKey := GetPlatformCacheKey(platform.ID)

	query := romm.GetRomsQuery{PlatformID: platform.ID}
	isFresh, err := checkCacheFreshnessInternal(c.host, c.config, cacheKey, query)

	if err != nil {
		logger.Debug("Refresh: Failed to validate cache", "platform", platform.Name, "error", err)
		c.freshnessMu.Lock()
		c.freshnessCache[cacheKey] = false
		c.freshnessMu.Unlock()
		c.prefetchPlatform(platform, cacheKey)
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = isFresh
	c.freshnessMu.Unlock()

	if isFresh {
		return
	}

	c.prefetchPlatform(platform, cacheKey)
}

func (c *Refresh) prefetchPlatform(platform romm.Platform, cacheKey string) {
	logger := gaba.GetLogger()

	done := make(chan struct{})

	c.prefetchMu.Lock()
	c.prefetchInProgress[cacheKey] = done
	c.prefetchMu.Unlock()

	defer func() {
		close(done)
		c.prefetchMu.Lock()
		delete(c.prefetchInProgress, cacheKey)
		c.prefetchMu.Unlock()
	}()

	logger.Debug("Refresh: Prefetching platform", "platform", platform.Name)

	games, err := c.fetchPlatformGames(platform.ID)
	if err != nil {
		logger.Debug("Refresh: Failed to prefetch platform", "platform", platform.Name, "error", err)
		return
	}

	if err := SaveGamesToCache(cacheKey, games); err != nil {
		logger.Debug("Refresh: Failed to save prefetched games", "platform", platform.Name, "error", err)
		return
	}

	logger.Debug("Refresh: Prefetched platform", "platform", platform.Name, "games", len(games))
}

func (c *Refresh) fetchPlatformGames(platformID int) ([]romm.Rom, error) {
	rc := romm.NewClientFromHost(c.host, c.config.GetApiTimeout())

	var allGames []romm.Rom
	page := 1
	const pageSize = 1000

	for {
		opt := romm.GetRomsQuery{
			PlatformID: platformID,
			Page:       page,
			Limit:      pageSize,
		}

		res, err := rc.GetRoms(opt)
		if err != nil {
			return nil, err
		}

		allGames = append(allGames, res.Items...)

		if len(allGames) >= res.Total || len(res.Items) == 0 {
			break
		}

		page++
	}

	return allGames, nil
}

func (c *Refresh) fetchAndPrefetchCollections() {
	logger := gaba.GetLogger()
	rc := romm.NewClientFromHost(c.host, c.config.GetApiTimeout())

	var allCollections []romm.Collection
	var mu sync.Mutex
	var wg sync.WaitGroup

	if c.config.GetShowCollections() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collections, err := rc.GetCollections()
			if err != nil {
				logger.Debug("Refresh: Failed to fetch regular collections", "error", err)
				return
			}
			mu.Lock()
			allCollections = append(allCollections, collections...)
			mu.Unlock()
		}()
	}

	if c.config.GetShowSmartCollections() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collections, err := rc.GetSmartCollections()
			if err != nil {
				logger.Debug("Refresh: Failed to fetch smart collections", "error", err)
				return
			}
			for i := range collections {
				collections[i].IsSmart = true
			}
			mu.Lock()
			allCollections = append(allCollections, collections...)
			mu.Unlock()
		}()
	}

	if c.config.GetShowVirtualCollections() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			virtualCollections, err := rc.GetVirtualCollections()
			if err != nil {
				logger.Debug("Refresh: Failed to fetch virtual collections", "error", err)
				return
			}
			mu.Lock()
			for _, vc := range virtualCollections {
				allCollections = append(allCollections, vc.ToCollection())
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	c.collectionsMu.Lock()
	c.collections = allCollections
	c.collectionsMu.Unlock()

	logger.Debug("Refresh: Fetched collections", "count", len(allCollections))

	var prefetchWg sync.WaitGroup
	for _, collection := range allCollections {
		prefetchWg.Add(1)
		go func(col romm.Collection) {
			defer prefetchWg.Done()
			c.validateAndPrefetchCollection(col)
		}(collection)
	}
	prefetchWg.Wait()
}

func (c *Refresh) validateAndPrefetchCollection(collection romm.Collection) {
	logger := gaba.GetLogger()
	cacheKey := GetCollectionCacheKey(collection)

	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("Refresh: Failed to load metadata for collection", "collection", collection.Name, "error", err)
		c.prefetchCollection(collection, cacheKey)
		return
	}

	entry, exists := metadata.Entries[cacheKey]
	if !exists {
		logger.Debug("Refresh: No cache entry for collection, prefetching", "collection", collection.Name)
		c.prefetchCollection(collection, cacheKey)
		return
	}

	cachePath := getCacheFilePath(cacheKey)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		logger.Debug("Refresh: Cache file missing for collection, prefetching", "collection", collection.Name)
		c.prefetchCollection(collection, cacheKey)
		return
	}

	if collection.IsVirtual {
		query := romm.GetRomsQuery{VirtualCollectionID: collection.VirtualID}
		isFresh, _ := checkCacheFreshnessInternal(c.host, c.config, cacheKey, query)
		c.freshnessMu.Lock()
		c.freshnessCache[cacheKey] = isFresh
		c.freshnessMu.Unlock()
		if !isFresh {
			c.prefetchCollection(collection, cacheKey)
		} else {
			logger.Debug("Refresh: Virtual collection cache is fresh", "collection", collection.Name)
		}
		return
	}

	if collection.UpdatedAt.After(entry.LastUpdatedAt) {
		logger.Debug("Refresh: Collection updated since cache, prefetching",
			"collection", collection.Name,
			"cached_at", entry.LastUpdatedAt,
			"updated_at", collection.UpdatedAt)
		c.freshnessMu.Lock()
		c.freshnessCache[cacheKey] = false
		c.freshnessMu.Unlock()
		c.prefetchCollection(collection, cacheKey)
		return
	}

	logger.Debug("Refresh: Collection cache is fresh", "collection", collection.Name)
	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = true
	c.freshnessMu.Unlock()
}

func (c *Refresh) prefetchCollection(collection romm.Collection, cacheKey string) {
	logger := gaba.GetLogger()

	done := make(chan struct{})

	c.prefetchMu.Lock()
	c.prefetchInProgress[cacheKey] = done
	c.prefetchMu.Unlock()

	defer func() {
		close(done)
		c.prefetchMu.Lock()
		delete(c.prefetchInProgress, cacheKey)
		c.prefetchMu.Unlock()
	}()

	logger.Debug("Refresh: Prefetching collection", "collection", collection.Name)

	games, err := c.fetchCollectionGames(collection)
	if err != nil {
		logger.Debug("Refresh: Failed to prefetch collection", "collection", collection.Name, "error", err)
		return
	}

	if err := saveCollectionToCache(cacheKey, games, collection.UpdatedAt); err != nil {
		logger.Debug("Refresh: Failed to save prefetched collection", "collection", collection.Name, "error", err)
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = true
	c.freshnessMu.Unlock()

	logger.Debug("Refresh: Prefetched collection", "collection", collection.Name, "games", len(games))
}

func (c *Refresh) fetchCollectionGames(collection romm.Collection) ([]romm.Rom, error) {
	rc := romm.NewClientFromHost(c.host, c.config.GetApiTimeout())

	opt := romm.GetRomsQuery{
		Limit: 10000,
	}

	if collection.IsVirtual {
		opt.VirtualCollectionID = collection.VirtualID
	} else if collection.IsSmart {
		opt.SmartCollectionID = collection.ID
	} else {
		opt.CollectionID = collection.ID
	}

	res, err := rc.GetRoms(opt)
	if err != nil {
		return nil, err
	}

	return res.Items, nil
}

func (c *Refresh) GetCollections() []romm.Collection {
	if c == nil {
		return nil
	}

	c.collectionsMu.RLock()
	defer c.collectionsMu.RUnlock()

	return c.collections
}

func (c *Refresh) fetchBIOSAvailability(platform romm.Platform) {
	logger := gaba.GetLogger()
	rc := romm.NewClientFromHost(c.host, c.config.GetApiTimeout())

	firmware, err := rc.GetFirmware(platform.ID)

	c.biosMu.Lock()
	if err != nil {
		logger.Debug("Refresh: Failed to fetch BIOS info", "platform", platform.Name, "error", err)
		c.biosCache[platform.ID] = false
	} else {
		hasBIOS := len(firmware) > 0
		c.biosCache[platform.ID] = hasBIOS
		logger.Debug("Refresh: Fetched BIOS info", "platform", platform.Name, "hasBIOS", hasBIOS)
	}
	c.biosMu.Unlock()
}

func (c *Refresh) IsCacheFresh(cacheKey string) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.freshnessMu.RLock()
	defer c.freshnessMu.RUnlock()

	isFresh, exists := c.freshnessCache[cacheKey]
	return isFresh, exists
}

func (c *Refresh) HasBIOS(platformID int) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.biosMu.RLock()
	defer c.biosMu.RUnlock()

	hasBIOS, exists := c.biosCache[platformID]
	return hasBIOS, exists
}

func (c *Refresh) MarkCacheStale(cacheKey string) {
	if c == nil {
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = false
	c.freshnessMu.Unlock()
}

func (c *Refresh) MarkCacheFresh(cacheKey string) {
	if c == nil {
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = true
	c.freshnessMu.Unlock()
}

func (c *Refresh) WaitForPrefetch(cacheKey string) bool {
	if c == nil {
		return false
	}

	c.prefetchMu.RLock()
	done, exists := c.prefetchInProgress[cacheKey]
	c.prefetchMu.RUnlock()

	if !exists {
		return false
	}

	<-done
	return true
}

func (c *Refresh) IsPrefetchInProgress(cacheKey string) bool {
	if c == nil {
		return false
	}

	c.prefetchMu.RLock()
	_, exists := c.prefetchInProgress[cacheKey]
	c.prefetchMu.RUnlock()

	return exists
}
