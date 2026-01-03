package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type RomCacheEntry struct {
	RomID    int       `json:"rom_id"`
	RomName  string    `json:"rom_name"`
	CachedAt time.Time `json:"cached_at"`
}

type PlatformRomCache struct {
	Entries map[string]RomCacheEntry `json:"entries"` // key = filename (no extension)
}

type RomCache struct {
	platforms map[string]*PlatformRomCache // key = platform slug
	mu        sync.RWMutex
}

var (
	romCache     *RomCache
	romCacheOnce sync.Once
)

func GetRomCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "roms")
	}
	return filepath.Join(wd, ".cache", "roms")
}

func getRomCacheFilePath(slug string) string {
	return filepath.Join(GetRomCacheDir(), slug+".json")
}

func getRomCache() *RomCache {
	romCacheOnce.Do(func() {
		romCache = &RomCache{
			platforms: make(map[string]*PlatformRomCache),
		}
	})
	return romCache
}

func (rc *RomCache) getPlatformCache(slug string) *PlatformRomCache {
	rc.mu.RLock()
	pc, exists := rc.platforms[slug]
	rc.mu.RUnlock()

	if exists {
		return pc
	}

	// Load from disk
	pc = rc.loadPlatform(slug)

	rc.mu.Lock()
	rc.platforms[slug] = pc
	rc.mu.Unlock()

	return pc
}

func (rc *RomCache) loadPlatform(slug string) *PlatformRomCache {
	logger := gaba.GetLogger()
	pc := &PlatformRomCache{
		Entries: make(map[string]RomCacheEntry),
	}

	data, err := os.ReadFile(getRomCacheFilePath(slug))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Debug("Failed to read ROM cache", "slug", slug, "error", err)
		}
		return pc
	}

	if err := json.Unmarshal(data, pc); err != nil {
		logger.Debug("Failed to parse ROM cache", "slug", slug, "error", err)
		return &PlatformRomCache{Entries: make(map[string]RomCacheEntry)}
	}

	if pc.Entries == nil {
		pc.Entries = make(map[string]RomCacheEntry)
	}

	logger.Debug("Loaded ROM cache", "slug", slug, "entries", len(pc.Entries))
	return pc
}

func (rc *RomCache) savePlatform(slug string, pc *PlatformRomCache) error {
	logger := gaba.GetLogger()

	if err := os.MkdirAll(GetRomCacheDir(), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(getRomCacheFilePath(slug), data, 0644); err != nil {
		return err
	}

	logger.Debug("Saved ROM cache", "slug", slug, "entries", len(pc.Entries))
	return nil
}

// GetCachedRomIDByFilename looks up a ROM ID by filename from the cache.
// Returns the ROM ID, ROM name, and whether it was found.
func GetCachedRomIDByFilename(slug, filename string) (int, string, bool) {
	rc := getRomCache()
	pc := rc.getPlatformCache(slug)

	key := filenameKey(filename)

	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if entry, ok := pc.Entries[key]; ok {
		return entry.RomID, entry.RomName, true
	}

	return 0, "", false
}

// CacheRomID stores a filename to ROM ID mapping in the cache.
func CacheRomID(slug, filename string, romID int, romName string) {
	logger := gaba.GetLogger()
	rc := getRomCache()
	pc := rc.getPlatformCache(slug)

	key := filenameKey(filename)

	rc.mu.Lock()
	pc.Entries[key] = RomCacheEntry{
		RomID:    romID,
		RomName:  romName,
		CachedAt: time.Now(),
	}

	// Make a copy of entries for serialization to avoid concurrent map access
	entriesCopy := make(map[string]RomCacheEntry, len(pc.Entries))
	for k, v := range pc.Entries {
		entriesCopy[k] = v
	}
	rc.mu.Unlock()

	// Save using the copy
	pcCopy := &PlatformRomCache{Entries: entriesCopy}
	if err := rc.savePlatform(slug, pcCopy); err != nil {
		logger.Debug("Failed to save ROM cache", "slug", slug, "error", err)
	}
}

// ClearRomCache removes all cached filename-to-ROM-ID mappings.
func ClearRomCache() error {
	cacheDir := GetRomCacheDir()

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil
	}

	// Reset in-memory cache
	rc := getRomCache()
	rc.mu.Lock()
	rc.platforms = make(map[string]*PlatformRomCache)
	rc.mu.Unlock()

	return os.RemoveAll(cacheDir)
}

// HasRomCache returns true if the ROM cache directory exists and has content.
func HasRomCache() bool {
	cacheDir := GetRomCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return false
	}

	return len(entries) > 0
}
