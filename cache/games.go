package cache

import (
	"encoding/json"
	"grout/romm"
	"os"
	"path/filepath"
	"strconv"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type Metadata struct {
	Entries map[string]Entry `json:"entries"`
}

type Entry struct {
	LastUpdatedAt time.Time `json:"last_updated_at"`
	CachedAt      time.Time `json:"cached_at"`
}

type Type string

const (
	Platform          Type = "platform"
	Collection        Type = "collection"
	SmartCollection   Type = "smart_collection"
	VirtualCollection Type = "virtual_collection"
)

func GetGamesCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "games")
	}
	return filepath.Join(wd, ".cache", "games")
}

func GetCacheKey(cacheType Type, id string) string {
	return string(cacheType) + "_" + id
}

func GetPlatformCacheKey(platformID int) string {
	return GetCacheKey(Platform, strconv.Itoa(platformID))
}

func GetCollectionCacheKey(collection romm.Collection) string {
	if collection.IsVirtual {
		return GetCacheKey(VirtualCollection, collection.VirtualID)
	}
	if collection.IsSmart {
		return GetCacheKey(SmartCollection, strconv.Itoa(collection.ID))
	}
	return GetCacheKey(Collection, strconv.Itoa(collection.ID))
}

func getCacheFilePath(cacheKey string) string {
	return filepath.Join(GetGamesCacheDir(), cacheKey+".json")
}

func getMetadataPath() string {
	return filepath.Join(GetGamesCacheDir(), "metadata.json")
}

func loadMetadata() (Metadata, error) {
	metadata := Metadata{Entries: make(map[string]Entry)}

	data, err := os.ReadFile(getMetadataPath())
	if err != nil {
		if os.IsNotExist(err) {
			return metadata, nil
		}
		return metadata, err
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{Entries: make(map[string]Entry)}, err
	}

	if metadata.Entries == nil {
		metadata.Entries = make(map[string]Entry)
	}

	return metadata, nil
}

func saveMetadata(metadata Metadata) error {
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
func CheckCacheFreshness(host romm.Host, config Config, cacheKey string, query romm.GetRomsQuery) (bool, error) {
	logger := gaba.GetLogger()

	if cr := GetRefresh(); cr != nil {
		if isFresh, wasValidated := cr.IsCacheFresh(cacheKey); wasValidated {
			logger.Debug("Using pre-validated cache freshness", "key", cacheKey, "fresh", isFresh)
			return isFresh, nil
		}
	}

	cachePath := getCacheFilePath(cacheKey)
	if _, err := os.Stat(cachePath); err == nil {
		logger.Debug("Cache not yet validated but exists locally, assuming fresh", "key", cacheKey)
		return true, nil
	}

	logger.Debug("No local cache found", "key", cacheKey)
	return false, nil
}

func checkCacheFreshnessInternal(host romm.Host, config Config, cacheKey string, query romm.GetRomsQuery) (bool, error) {
	logger := gaba.GetLogger()

	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("Failed to load cache metadata", "error", err)
		return false, nil
	}

	entry, exists := metadata.Entries[cacheKey]
	if !exists {
		logger.Debug("No cache entry found", "key", cacheKey)
		return false, nil
	}

	cachePath := getCacheFilePath(cacheKey)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		logger.Debug("Cache file not found", "key", cacheKey)
		return false, nil
	}

	client := romm.NewClientFromHost(host, config.GetApiTimeout())

	checkQuery := romm.GetRomsQuery{
		Limit:    1,
		OrderBy:  "updated_at",
		OrderDir: "desc",
	}

	checkQuery.PlatformID = query.PlatformID
	checkQuery.CollectionID = query.CollectionID
	checkQuery.SmartCollectionID = query.SmartCollectionID
	checkQuery.VirtualCollectionID = query.VirtualCollectionID

	res, err := client.GetRoms(checkQuery)
	if err != nil {
		logger.Debug("Failed to check cache freshness", "error", err)
		return false, err
	}

	if len(res.Items) == 0 {
		cached, err := LoadCachedGames(cacheKey)
		if err == nil && len(cached) == 0 {
			logger.Debug("Cache is fresh (empty collection)", "key", cacheKey)
			return true, nil
		}
		return false, nil
	}

	latestUpdatedAt := res.Items[0].UpdatedAt

	if latestUpdatedAt.Equal(entry.LastUpdatedAt) || latestUpdatedAt.Before(entry.LastUpdatedAt) {
		return true, nil
	}

	return false, nil
}

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

func SaveGamesToCache(cacheKey string, games []romm.Rom) error {
	logger := gaba.GetLogger()

	if err := os.MkdirAll(GetGamesCacheDir(), 0755); err != nil {
		return err
	}

	var latestUpdatedAt time.Time
	for _, game := range games {
		if game.UpdatedAt.After(latestUpdatedAt) {
			latestUpdatedAt = game.UpdatedAt
		}
	}

	cachePath := getCacheFilePath(cacheKey)
	data, err := json.Marshal(games)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return err
	}

	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("Failed to load metadata for update", "error", err)
		metadata = Metadata{Entries: make(map[string]Entry)}
	}

	metadata.Entries[cacheKey] = Entry{
		LastUpdatedAt: latestUpdatedAt,
		CachedAt:      time.Now(),
	}

	if err := saveMetadata(metadata); err != nil {
		logger.Debug("Failed to save metadata", "error", err)
		return err
	}

	if cr := GetRefresh(); cr != nil {
		cr.MarkCacheFresh(cacheKey)
	}

	logger.Debug("Saved games to cache", "key", cacheKey, "count", len(games), "latestUpdatedAt", latestUpdatedAt)
	return nil
}

func saveCollectionToCache(cacheKey string, games []romm.Rom, updatedAt time.Time) error {
	logger := gaba.GetLogger()

	if err := os.MkdirAll(GetGamesCacheDir(), 0755); err != nil {
		return err
	}

	cachePath := getCacheFilePath(cacheKey)
	data, err := json.Marshal(games)
	if err != nil {
		return err
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return err
	}

	metadata, err := loadMetadata()
	if err != nil {
		logger.Debug("Failed to load metadata for update", "error", err)
		metadata = Metadata{Entries: make(map[string]Entry)}
	}

	metadata.Entries[cacheKey] = Entry{
		LastUpdatedAt: updatedAt,
		CachedAt:      time.Now(),
	}

	if err := saveMetadata(metadata); err != nil {
		logger.Debug("Failed to save metadata", "error", err)
		return err
	}

	if cr := GetRefresh(); cr != nil {
		cr.MarkCacheFresh(cacheKey)
	}

	logger.Debug("Saved collection to cache", "key", cacheKey, "count", len(games), "updatedAt", updatedAt)
	return nil
}

func ClearGamesCache() error {
	cacheDir := GetGamesCacheDir()

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(cacheDir)
}

func HasGamesCache() bool {
	cacheDir := GetGamesCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	return len(entries) > 0
}
