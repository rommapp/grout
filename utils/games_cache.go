package utils

import (
	"encoding/json"
	"grout/romm"
	"os"
	"path/filepath"
	"strconv"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// CacheMetadata stores the last_updated_at timestamp for each cached platform/collection
type CacheMetadata struct {
	Entries map[string]CacheEntry `json:"entries"`
}

// CacheEntry represents a single cache entry's metadata
type CacheEntry struct {
	LastUpdatedAt time.Time `json:"last_updated_at"`
	CachedAt      time.Time `json:"cached_at"`
}

// CacheType represents the type of cache entry
type CacheType string

const (
	CacheTypePlatform          CacheType = "platform"
	CacheTypeCollection        CacheType = "collection"
	CacheTypeSmartCollection   CacheType = "smart_collection"
	CacheTypeVirtualCollection CacheType = "virtual_collection"
)

// GetGamesCacheDir returns the path to the games cache directory
func GetGamesCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "games")
	}
	return filepath.Join(wd, ".cache", "games")
}

// GetCacheKey generates a cache key for a platform or collection
func GetCacheKey(cacheType CacheType, id string) string {
	return string(cacheType) + ":" + id
}

// GetPlatformCacheKey generates a cache key for a platform
func GetPlatformCacheKey(platformID int) string {
	return GetCacheKey(CacheTypePlatform, strconv.Itoa(platformID))
}

// GetCollectionCacheKey generates a cache key for a collection based on its type
func GetCollectionCacheKey(collection romm.Collection) string {
	if collection.IsVirtual {
		return GetCacheKey(CacheTypeVirtualCollection, collection.VirtualID)
	}
	if collection.IsSmart {
		return GetCacheKey(CacheTypeSmartCollection, strconv.Itoa(collection.ID))
	}
	return GetCacheKey(CacheTypeCollection, strconv.Itoa(collection.ID))
}

// getCacheFilePath returns the file path for a cache key
func getCacheFilePath(cacheKey string) string {
	// Replace ":" with "_" for file system compatibility
	filename := cacheKey + ".json"
	for i, c := range filename {
		if c == ':' {
			filename = filename[:i] + "_" + filename[i+1:]
		}
	}
	return filepath.Join(GetGamesCacheDir(), filename)
}

// getMetadataPath returns the path to the metadata file
func getMetadataPath() string {
	return filepath.Join(GetGamesCacheDir(), "metadata.json")
}

// loadMetadata loads the cache metadata from disk
func loadMetadata() (CacheMetadata, error) {
	metadata := CacheMetadata{Entries: make(map[string]CacheEntry)}

	data, err := os.ReadFile(getMetadataPath())
	if err != nil {
		if os.IsNotExist(err) {
			return metadata, nil
		}
		return metadata, err
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return CacheMetadata{Entries: make(map[string]CacheEntry)}, err
	}

	if metadata.Entries == nil {
		metadata.Entries = make(map[string]CacheEntry)
	}

	return metadata, nil
}

// saveMetadata saves the cache metadata to disk
func saveMetadata(metadata CacheMetadata) error {
	if err := os.MkdirAll(GetGamesCacheDir(), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getMetadataPath(), data, 0644)
}

// CheckCacheFreshness checks if the cache for a given key is still fresh
// Returns true if cache is fresh (can use cached data), false if stale (need to refetch)
func CheckCacheFreshness(host romm.Host, config *Config, cacheKey string, query romm.GetRomsQuery) (bool, error) {
	logger := gaba.GetLogger()

	// Load metadata to get the stored last_updated_at
	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("Failed to load cache metadata", "error", err)
		return false, nil // Treat as stale if we can't read metadata
	}

	entry, exists := metadata.Entries[cacheKey]
	if !exists {
		logger.Debug("No cache entry found", "key", cacheKey)
		return false, nil // No cache entry, need to fetch
	}

	// Check if the cache file exists
	cachePath := getCacheFilePath(cacheKey)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		logger.Debug("Cache file not found", "key", cacheKey)
		return false, nil
	}

	// Make a lightweight API call to get the most recently updated ROM
	client := GetRommClient(host, config.ApiTimeout)

	// Create a query for just the most recently updated ROM
	checkQuery := romm.GetRomsQuery{
		Limit:    1,
		OrderBy:  "updated_at",
		OrderDir: "desc",
	}

	// Copy the relevant ID fields from the original query
	checkQuery.PlatformID = query.PlatformID
	checkQuery.CollectionID = query.CollectionID
	checkQuery.SmartCollectionID = query.SmartCollectionID
	checkQuery.VirtualCollectionID = query.VirtualCollectionID

	res, err := client.GetRoms(checkQuery)
	if err != nil {
		logger.Debug("Failed to check cache freshness", "error", err)
		return false, err
	}

	// If no ROMs returned, check if we have an empty cache (which is valid)
	if len(res.Items) == 0 {
		// Check if we cached an empty result
		cached, err := LoadCachedGames(cacheKey)
		if err == nil && len(cached) == 0 {
			logger.Debug("Cache is fresh (empty collection)", "key", cacheKey)
			return true, nil
		}
		// Cache had items but now there are none - stale
		return false, nil
	}

	// Compare the most recent ROM's updated_at with our cached value
	latestUpdatedAt := res.Items[0].UpdatedAt

	if latestUpdatedAt.Equal(entry.LastUpdatedAt) || latestUpdatedAt.Before(entry.LastUpdatedAt) {
		return true, nil
	}

	return false, nil
}

// LoadCachedGames loads games from the cache file
func LoadCachedGames(cacheKey string) ([]romm.Rom, error) {
	cachePath := getCacheFilePath(cacheKey)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var games []romm.Rom
	if err := json.Unmarshal(data, &games); err != nil {
		return nil, err
	}

	return games, nil
}

// SaveGamesToCache saves games to the cache file and updates metadata
func SaveGamesToCache(cacheKey string, games []romm.Rom) error {
	logger := gaba.GetLogger()

	if err := os.MkdirAll(GetGamesCacheDir(), 0755); err != nil {
		return err
	}

	// Find the most recent updated_at from the games
	var latestUpdatedAt time.Time
	for _, game := range games {
		if game.UpdatedAt.After(latestUpdatedAt) {
			latestUpdatedAt = game.UpdatedAt
		}
	}

	// Save the games to the cache file
	cachePath := getCacheFilePath(cacheKey)
	data, err := json.Marshal(games)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return err
	}

	// Update metadata
	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("Failed to load metadata for update", "error", err)
		metadata = CacheMetadata{Entries: make(map[string]CacheEntry)}
	}

	metadata.Entries[cacheKey] = CacheEntry{
		LastUpdatedAt: latestUpdatedAt,
		CachedAt:      time.Now(),
	}

	if err := saveMetadata(metadata); err != nil {
		logger.Debug("Failed to save metadata", "error", err)
		return err
	}

	logger.Debug("Saved games to cache", "key", cacheKey, "count", len(games), "latestUpdatedAt", latestUpdatedAt)
	return nil
}

// ClearGamesCache removes all cached game data
func ClearGamesCache() error {
	cacheDir := GetGamesCacheDir()

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil // Nothing to clear
	}

	return os.RemoveAll(cacheDir)
}

// HasGamesCache returns true if the games cache directory exists and has content
func HasGamesCache() bool {
	cacheDir := GetGamesCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	return len(entries) > 0
}
