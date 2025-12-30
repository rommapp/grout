package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type HashCacheEntry struct {
	RomID    int       `json:"rom_id"`
	RomName  string    `json:"rom_name"`
	CachedAt time.Time `json:"cached_at"`
}

type PlatformHashCache struct {
	Entries map[string]HashCacheEntry `json:"entries"` // key = SHA1 hash
}

type HashCache struct {
	platforms map[string]*PlatformHashCache // key = platform slug
	mu        sync.RWMutex
}

var (
	hashCache     *HashCache
	hashCacheOnce sync.Once
)

func GetHashCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "hashes")
	}
	return filepath.Join(wd, ".cache", "hashes")
}

func getHashCacheFilePath(slug string) string {
	return filepath.Join(GetHashCacheDir(), slug+".json")
}

func getHashCache() *HashCache {
	hashCacheOnce.Do(func() {
		hashCache = &HashCache{
			platforms: make(map[string]*PlatformHashCache),
		}
	})
	return hashCache
}

func (hc *HashCache) getPlatformCache(slug string) *PlatformHashCache {
	hc.mu.RLock()
	pc, exists := hc.platforms[slug]
	hc.mu.RUnlock()

	if exists {
		return pc
	}

	// Load from disk
	pc = hc.loadPlatform(slug)

	hc.mu.Lock()
	hc.platforms[slug] = pc
	hc.mu.Unlock()

	return pc
}

func (hc *HashCache) loadPlatform(slug string) *PlatformHashCache {
	logger := gaba.GetLogger()
	pc := &PlatformHashCache{
		Entries: make(map[string]HashCacheEntry),
	}

	data, err := os.ReadFile(getHashCacheFilePath(slug))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Debug("Failed to read hash cache", "slug", slug, "error", err)
		}
		return pc
	}

	if err := json.Unmarshal(data, pc); err != nil {
		logger.Debug("Failed to parse hash cache", "slug", slug, "error", err)
		return &PlatformHashCache{Entries: make(map[string]HashCacheEntry)}
	}

	if pc.Entries == nil {
		pc.Entries = make(map[string]HashCacheEntry)
	}

	logger.Debug("Loaded hash cache", "slug", slug, "entries", len(pc.Entries))
	return pc
}

func (hc *HashCache) savePlatform(slug string, pc *PlatformHashCache) error {
	logger := gaba.GetLogger()

	if err := os.MkdirAll(GetHashCacheDir(), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(getHashCacheFilePath(slug), data, 0644); err != nil {
		return err
	}

	logger.Debug("Saved hash cache", "slug", slug, "entries", len(pc.Entries))
	return nil
}

// GetCachedRomID looks up a ROM ID by SHA1 hash from the cache.
// Returns the ROM ID, ROM name, and whether it was found.
func GetCachedRomID(slug, sha1 string) (int, string, bool) {
	hc := getHashCache()
	pc := hc.getPlatformCache(slug)

	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if entry, ok := pc.Entries[sha1]; ok {
		return entry.RomID, entry.RomName, true
	}

	return 0, "", false
}

// CacheRomID stores a SHA1 hash to ROM ID mapping in the cache.
func CacheRomID(slug, sha1 string, romID int, romName string) {
	logger := gaba.GetLogger()
	hc := getHashCache()
	pc := hc.getPlatformCache(slug)

	hc.mu.Lock()
	pc.Entries[sha1] = HashCacheEntry{
		RomID:    romID,
		RomName:  romName,
		CachedAt: time.Now(),
	}
	hc.mu.Unlock()

	if err := hc.savePlatform(slug, pc); err != nil {
		logger.Debug("Failed to save hash cache", "slug", slug, "error", err)
	}
}

// ClearHashCache removes all cached hash-to-ROM-ID mappings.
func ClearHashCache() error {
	cacheDir := GetHashCacheDir()

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil
	}

	// Reset in-memory cache
	hc := getHashCache()
	hc.mu.Lock()
	hc.platforms = make(map[string]*PlatformHashCache)
	hc.mu.Unlock()

	return os.RemoveAll(cacheDir)
}

// HasHashCache returns true if the hash cache directory exists and has content.
func HasHashCache() bool {
	cacheDir := GetHashCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	return len(entries) > 0
}
