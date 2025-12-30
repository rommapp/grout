package utils

import (
	"grout/romm"
	"os"
	"sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// CacheRefresh handles startup cache validation, BIOS pre-fetching, and prefetching missing platforms/collections
type CacheRefresh struct {
	host   romm.Host
	config *Config

	// Cache freshness results - map of cache key to freshness state
	freshnessCache map[string]bool
	freshnessMu    sync.RWMutex

	// BIOS availability results - map of platform ID to hasBIOS
	biosCache map[int]bool
	biosMu    sync.RWMutex

	// Prefetch tracking - map of cache key to completion channel
	prefetchInProgress map[string]chan struct{}
	prefetchMu         sync.RWMutex

	// Collections cache - stores fetched collections for reuse
	collections   []romm.Collection
	collectionsMu sync.RWMutex

	done    chan struct{}
	running bool
}

var (
	cacheRefreshInstance *CacheRefresh
	cacheRefreshOnce     sync.Once
)

// GetCacheRefresh returns the singleton CacheRefresh instance
func GetCacheRefresh() *CacheRefresh {
	return cacheRefreshInstance
}

// InitCacheRefresh initializes and starts the cache refresh process
func InitCacheRefresh(host romm.Host, config *Config, platforms []romm.Platform) {
	cacheRefreshOnce.Do(func() {
		cacheRefreshInstance = &CacheRefresh{
			host:               host,
			config:             config,
			freshnessCache:     make(map[string]bool),
			biosCache:          make(map[int]bool),
			prefetchInProgress: make(map[string]chan struct{}),
			done:               make(chan struct{}),
		}
		cacheRefreshInstance.start(platforms)
	})
}

func (c *CacheRefresh) start(platforms []romm.Platform) {
	c.running = true
	go c.run(platforms)
}

func (c *CacheRefresh) run(platforms []romm.Platform) {
	logger := gaba.GetLogger()
	defer func() {
		c.running = false
		close(c.done)
	}()

	logger.Debug("CacheRefresh: Starting background cache validation and prefetch")

	// Validate artwork cache in background
	ValidateArtworkCache()

	var wg sync.WaitGroup

	// Pre-fetch BIOS availability for all platforms in parallel (fast, do first)
	for _, platform := range platforms {
		wg.Add(1)
		go func(p romm.Platform) {
			defer wg.Done()
			c.fetchBIOSAvailability(p)
		}(platform)
	}

	// Validate and prefetch platforms in parallel
	for _, platform := range platforms {
		wg.Add(1)
		go func(p romm.Platform) {
			defer wg.Done()
			c.validateAndPrefetchPlatform(p)
		}(platform)
	}

	// Fetch and prefetch collections in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.fetchAndPrefetchCollections()
	}()

	wg.Wait()
	logger.Debug("CacheRefresh: Completed background cache validation and prefetch",
		"platforms", len(platforms),
		"collections", len(c.collections),
		"freshness_entries", len(c.freshnessCache),
		"bios_entries", len(c.biosCache))
}

func (c *CacheRefresh) validateAndPrefetchPlatform(platform romm.Platform) {
	logger := gaba.GetLogger()
	cacheKey := GetPlatformCacheKey(platform.ID)

	query := romm.GetRomsQuery{PlatformID: platform.ID}
	isFresh, err := checkCacheFreshnessInternal(c.host, c.config, cacheKey, query)

	if err != nil {
		logger.Debug("CacheRefresh: Failed to validate cache", "platform", platform.Name, "error", err)
		c.freshnessMu.Lock()
		c.freshnessCache[cacheKey] = false
		c.freshnessMu.Unlock()
		// Still try to prefetch on error
		c.prefetchPlatform(platform, cacheKey)
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = isFresh
	c.freshnessMu.Unlock()

	if isFresh {
		logger.Debug("CacheRefresh: Cache is fresh, skipping prefetch", "platform", platform.Name)
		return
	}

	// Cache is stale or missing, prefetch it
	c.prefetchPlatform(platform, cacheKey)
}

func (c *CacheRefresh) prefetchPlatform(platform romm.Platform, cacheKey string) {
	logger := gaba.GetLogger()

	// Create a completion channel for this prefetch
	done := make(chan struct{})

	c.prefetchMu.Lock()
	c.prefetchInProgress[cacheKey] = done
	c.prefetchMu.Unlock()

	defer func() {
		// Signal completion and remove from in-progress map
		close(done)
		c.prefetchMu.Lock()
		delete(c.prefetchInProgress, cacheKey)
		c.prefetchMu.Unlock()
	}()

	logger.Debug("CacheRefresh: Prefetching platform", "platform", platform.Name)

	// Fetch the games
	games, err := c.fetchPlatformGames(platform.ID)
	if err != nil {
		logger.Debug("CacheRefresh: Failed to prefetch platform", "platform", platform.Name, "error", err)
		return
	}

	// Save to cache
	if err := SaveGamesToCache(cacheKey, games); err != nil {
		logger.Debug("CacheRefresh: Failed to save prefetched games", "platform", platform.Name, "error", err)
		return
	}

	logger.Debug("CacheRefresh: Prefetched platform", "platform", platform.Name, "games", len(games))
}

func (c *CacheRefresh) fetchPlatformGames(platformID int) ([]romm.Rom, error) {
	rc := GetRommClient(c.host, c.config.ApiTimeout)

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

func (c *CacheRefresh) fetchAndPrefetchCollections() {
	logger := gaba.GetLogger()
	rc := GetRommClient(c.host, c.config.ApiTimeout)

	var allCollections []romm.Collection
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Fetch regular collections if enabled
	if c.config.ShowCollections {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collections, err := rc.GetCollections()
			if err != nil {
				logger.Debug("CacheRefresh: Failed to fetch regular collections", "error", err)
				return
			}
			mu.Lock()
			allCollections = append(allCollections, collections...)
			mu.Unlock()
		}()
	}

	// Fetch smart collections if enabled
	if c.config.ShowSmartCollections {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collections, err := rc.GetSmartCollections()
			if err != nil {
				logger.Debug("CacheRefresh: Failed to fetch smart collections", "error", err)
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

	// Fetch virtual collections if enabled
	if c.config.ShowVirtualCollections {
		wg.Add(1)
		go func() {
			defer wg.Done()
			virtualCollections, err := rc.GetVirtualCollections()
			if err != nil {
				logger.Debug("CacheRefresh: Failed to fetch virtual collections", "error", err)
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

	// Store collections for reuse
	c.collectionsMu.Lock()
	c.collections = allCollections
	c.collectionsMu.Unlock()

	logger.Debug("CacheRefresh: Fetched collections", "count", len(allCollections))

	// Validate and prefetch each collection in parallel
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

func (c *CacheRefresh) validateAndPrefetchCollection(collection romm.Collection) {
	logger := gaba.GetLogger()
	cacheKey := GetCollectionCacheKey(collection)

	// Load cache metadata to check UpdatedAt
	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("CacheRefresh: Failed to load metadata for collection", "collection", collection.Name, "error", err)
		c.prefetchCollection(collection, cacheKey)
		return
	}

	entry, exists := metadata.Entries[cacheKey]
	if !exists {
		logger.Debug("CacheRefresh: No cache entry for collection, prefetching", "collection", collection.Name)
		c.prefetchCollection(collection, cacheKey)
		return
	}

	// Check if cache file exists
	cachePath := getCacheFilePath(cacheKey)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		logger.Debug("CacheRefresh: Cache file missing for collection, prefetching", "collection", collection.Name)
		c.prefetchCollection(collection, cacheKey)
		return
	}

	// Compare UpdatedAt - if collection was updated after our cache, it's stale
	// For virtual collections, UpdatedAt is zero, so we use the ROM-based freshness check
	if collection.IsVirtual {
		// Virtual collections don't have UpdatedAt, use ROM-based check
		query := romm.GetRomsQuery{VirtualCollectionID: collection.VirtualID}
		isFresh, _ := checkCacheFreshnessInternal(c.host, c.config, cacheKey, query)
		c.freshnessMu.Lock()
		c.freshnessCache[cacheKey] = isFresh
		c.freshnessMu.Unlock()
		if !isFresh {
			c.prefetchCollection(collection, cacheKey)
		} else {
			logger.Debug("CacheRefresh: Virtual collection cache is fresh", "collection", collection.Name)
		}
		return
	}

	// For regular and smart collections, compare UpdatedAt
	if collection.UpdatedAt.After(entry.LastUpdatedAt) {
		logger.Debug("CacheRefresh: Collection updated since cache, prefetching",
			"collection", collection.Name,
			"cached_at", entry.LastUpdatedAt,
			"updated_at", collection.UpdatedAt)
		c.freshnessMu.Lock()
		c.freshnessCache[cacheKey] = false
		c.freshnessMu.Unlock()
		c.prefetchCollection(collection, cacheKey)
		return
	}

	// Cache is fresh
	logger.Debug("CacheRefresh: Collection cache is fresh", "collection", collection.Name)
	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = true
	c.freshnessMu.Unlock()
}

func (c *CacheRefresh) prefetchCollection(collection romm.Collection, cacheKey string) {
	logger := gaba.GetLogger()

	// Create a completion channel for this prefetch
	done := make(chan struct{})

	c.prefetchMu.Lock()
	c.prefetchInProgress[cacheKey] = done
	c.prefetchMu.Unlock()

	defer func() {
		// Signal completion and remove from in-progress map
		close(done)
		c.prefetchMu.Lock()
		delete(c.prefetchInProgress, cacheKey)
		c.prefetchMu.Unlock()
	}()

	logger.Debug("CacheRefresh: Prefetching collection", "collection", collection.Name)

	// Fetch the games
	games, err := c.fetchCollectionGames(collection)
	if err != nil {
		logger.Debug("CacheRefresh: Failed to prefetch collection", "collection", collection.Name, "error", err)
		return
	}

	// Save to cache with collection's UpdatedAt as the timestamp
	if err := saveCollectionToCache(cacheKey, games, collection.UpdatedAt); err != nil {
		logger.Debug("CacheRefresh: Failed to save prefetched collection", "collection", collection.Name, "error", err)
		return
	}

	// Mark as fresh
	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = true
	c.freshnessMu.Unlock()

	logger.Debug("CacheRefresh: Prefetched collection", "collection", collection.Name, "games", len(games))
}

func (c *CacheRefresh) fetchCollectionGames(collection romm.Collection) ([]romm.Rom, error) {
	rc := GetRommClient(c.host, c.config.ApiTimeout)

	opt := romm.GetRomsQuery{
		Limit: 10000,
	}

	// Use appropriate ID based on collection type
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

// GetCollections returns the cached collections list
func (c *CacheRefresh) GetCollections() []romm.Collection {
	if c == nil {
		return nil
	}

	c.collectionsMu.RLock()
	defer c.collectionsMu.RUnlock()

	return c.collections
}

func (c *CacheRefresh) fetchBIOSAvailability(platform romm.Platform) {
	logger := gaba.GetLogger()
	rc := GetRommClient(c.host, c.config.ApiTimeout)

	firmware, err := rc.GetFirmware(platform.ID)

	c.biosMu.Lock()
	if err != nil {
		logger.Debug("CacheRefresh: Failed to fetch BIOS info", "platform", platform.Name, "error", err)
		c.biosCache[platform.ID] = false
	} else {
		hasBIOS := len(firmware) > 0
		c.biosCache[platform.ID] = hasBIOS
		logger.Debug("CacheRefresh: Fetched BIOS info", "platform", platform.Name, "hasBIOS", hasBIOS)
	}
	c.biosMu.Unlock()
}

// IsCacheFresh returns the pre-validated freshness state for a cache key
// Returns (isFresh, wasValidated) - if wasValidated is false, caller should do network check
func (c *CacheRefresh) IsCacheFresh(cacheKey string) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.freshnessMu.RLock()
	defer c.freshnessMu.RUnlock()

	isFresh, exists := c.freshnessCache[cacheKey]
	return isFresh, exists
}

// HasBIOS returns the pre-fetched BIOS availability for a platform
// Returns (hasBIOS, wasFetched) - if wasFetched is false, caller should do network check
func (c *CacheRefresh) HasBIOS(platformID int) (bool, bool) {
	if c == nil {
		return false, false
	}

	c.biosMu.RLock()
	defer c.biosMu.RUnlock()

	hasBIOS, exists := c.biosCache[platformID]
	return hasBIOS, exists
}

// MarkCacheStale marks a cache key as stale (e.g., after fetching fresh data)
func (c *CacheRefresh) MarkCacheStale(cacheKey string) {
	if c == nil {
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = false
	c.freshnessMu.Unlock()
}

// MarkCacheFresh marks a cache key as fresh (e.g., after saving new data)
func (c *CacheRefresh) MarkCacheFresh(cacheKey string) {
	if c == nil {
		return
	}

	c.freshnessMu.Lock()
	c.freshnessCache[cacheKey] = true
	c.freshnessMu.Unlock()
}

// WaitForPrefetch waits for an in-progress prefetch to complete
// Returns true if there was a prefetch in progress that we waited for
func (c *CacheRefresh) WaitForPrefetch(cacheKey string) bool {
	if c == nil {
		return false
	}

	c.prefetchMu.RLock()
	done, exists := c.prefetchInProgress[cacheKey]
	c.prefetchMu.RUnlock()

	if !exists {
		return false
	}

	// Wait for the prefetch to complete
	<-done
	return true
}

// IsPrefetchInProgress returns true if a prefetch is currently running for the given cache key
func (c *CacheRefresh) IsPrefetchInProgress(cacheKey string) bool {
	if c == nil {
		return false
	}

	c.prefetchMu.RLock()
	_, exists := c.prefetchInProgress[cacheKey]
	c.prefetchMu.RUnlock()

	return exists
}
