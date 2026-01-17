package cache

import (
	"database/sql"
	"grout/internal/fileutil"
	"grout/romm"
	"os"
	"path/filepath"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"go.uber.org/atomic"
	_ "modernc.org/sqlite"
)

type Manager struct {
	db          *sql.DB
	dbPath      string
	mu          sync.RWMutex
	host        romm.Host
	config      Config
	initialized bool

	stats *CacheStats
}

type CacheStats struct {
	mu         sync.Mutex
	Hits       int64
	Misses     int64
	Errors     int64
	LastAccess time.Time
}

func (s *CacheStats) recordHit() {
	s.mu.Lock()
	s.Hits++
	s.LastAccess = time.Now()
	s.mu.Unlock()
}

func (s *CacheStats) recordMiss() {
	s.mu.Lock()
	s.Misses++
	s.LastAccess = time.Now()
	s.mu.Unlock()
}

func (s *CacheStats) recordError() {
	s.mu.Lock()
	s.Errors++
	s.mu.Unlock()
}

var (
	cacheManager     *Manager
	cacheManagerOnce sync.Once
	cacheManagerErr  error
)

func GetCacheManager() *Manager {
	return cacheManager
}

func InitCacheManager(host romm.Host, config Config) error {
	cacheManagerOnce.Do(func() {
		cacheManager, cacheManagerErr = newCacheManager(host, config)
	})
	return cacheManagerErr
}

func newCacheManager(host romm.Host, config Config) (*Manager, error) {
	logger := gaba.GetLogger()

	dbPath := getCacheDBPath()

	cacheDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, newCacheError("init", "", "", err)
	}

	cleanupLegacyCache()

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, newCacheError("init", "", "", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := createTables(db); err != nil {
		db.Close()
		return nil, newCacheError("init", "", "", err)
	}

	cm := &Manager{
		db:          db,
		dbPath:      dbPath,
		host:        host,
		config:      config,
		initialized: true,
		stats:       &CacheStats{},
	}

	logger.Debug("Cache manager initialized", "path", dbPath)
	return cm, nil
}

func (cm *Manager) Close() error {
	if cm == nil || cm.db == nil {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.initialized = false
	return cm.db.Close()
}

func (cm *Manager) IsFirstRun() bool {
	if cm == nil || !cm.initialized {
		return true
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var count int
	err := cm.db.QueryRow("SELECT COUNT(*) FROM games").Scan(&count)
	if err != nil {
		return true
	}

	return count == 0
}

func (cm *Manager) HasCache() bool {
	if cm == nil || !cm.initialized {
		return false
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var count int
	err := cm.db.QueryRow("SELECT COUNT(*) FROM games").Scan(&count)
	return err == nil && count > 0
}

func (cm *Manager) Clear() error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tables := []string{"games", "game_collections", "collections", "platforms", "bios_availability", "filename_mappings"}

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("clear", "", "", err)
	}
	defer tx.Rollback()

	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return newCacheError("clear", table, "", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("clear", "", "", err)
	}

	artworkDir := GetArtworkCacheDir()
	if fileutil.FileExists(artworkDir) {
		os.RemoveAll(artworkDir)
	}

	logger.Info("Cache cleared")
	return nil
}

func (cm *Manager) ClearGames() error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("clear_games", "", "", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM game_collections"); err != nil {
		return newCacheError("clear_games", "game_collections", "", err)
	}

	if _, err := tx.Exec("DELETE FROM games"); err != nil {
		return newCacheError("clear_games", "games", "", err)
	}

	return tx.Commit()
}

func (cm *Manager) ClearCollections() error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("clear_collections", "", "", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM game_collections"); err != nil {
		return newCacheError("clear_collections", "game_collections", "", err)
	}

	if _, err := tx.Exec("DELETE FROM collections"); err != nil {
		return newCacheError("clear_collections", "collections", "", err)
	}

	return tx.Commit()
}

func (cm *Manager) HasCollections() bool {
	if cm == nil || !cm.initialized {
		return false
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var count int
	err := cm.db.QueryRow("SELECT COUNT(*) FROM collections").Scan(&count)
	return err == nil && count > 0
}

const (
	MetaKeyGamesRefreshedAt       = "games_refreshed_at"
	MetaKeyCollectionsRefreshedAt = "collections_refreshed_at"
)

func (cm *Manager) SetMetadata(key, value string) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, err := cm.db.Exec(`
		INSERT OR REPLACE INTO cache_metadata (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`, key, value)
	if err != nil {
		return newCacheError("set_metadata", key, "", err)
	}

	return nil
}

func (cm *Manager) GetMetadata(key string) (string, error) {
	if cm == nil || !cm.initialized {
		return "", ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var value string
	err := cm.db.QueryRow(`SELECT value FROM cache_metadata WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", newCacheError("get_metadata", key, "", err)
	}

	return value, nil
}

func (cm *Manager) GetLastRefreshTime(key string) (time.Time, error) {
	value, err := cm.GetMetadata(key)
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(time.RFC3339, value)
}

func (cm *Manager) RecordRefreshTime(key string) error {
	return cm.SetMetadata(key, time.Now().Format(time.RFC3339))
}

func (cm *Manager) GetAllRefreshTimes() map[string]time.Time {
	result := make(map[string]time.Time)

	keys := []string{MetaKeyGamesRefreshedAt, MetaKeyCollectionsRefreshedAt}
	for _, key := range keys {
		if t, err := cm.GetLastRefreshTime(key); err == nil {
			result[key] = t
		}
	}

	return result
}

func (cm *Manager) PopulateFullCacheWithProgress(platforms []romm.Platform, progress *atomic.Float64) (SyncStats, error) {
	if cm == nil || !cm.initialized {
		return SyncStats{}, ErrNotInitialized
	}

	return cm.populateCache(platforms, progress)
}

func getCacheDBPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "grout.db")
	}
	return filepath.Join(wd, ".cache", "grout.db")
}

func GetArtworkCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache", "artwork")
	}
	return filepath.Join(wd, ".cache", "artwork")
}

func GetCacheDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Join(os.TempDir(), ".cache")
	}
	return filepath.Join(wd, ".cache")
}

func DeleteCacheFolder() error {
	logger := gaba.GetLogger()

	if cacheManager != nil {
		cacheManager.Close()
		cacheManager = nil
	}

	cacheManagerOnce = sync.Once{}
	cacheManagerErr = nil

	cacheDir := GetCacheDir()
	if err := os.RemoveAll(cacheDir); err != nil {
		logger.Error("Failed to delete cache folder", "path", cacheDir, "error", err)
		return err
	}

	logger.Info("Cache folder deleted", "path", cacheDir)
	return nil
}

func cleanupLegacyCache() {
	logger := gaba.GetLogger()

	wd, err := os.Getwd()
	if err != nil {
		return
	}

	gamesDir := filepath.Join(wd, ".cache", "games")
	if fileutil.FileExists(gamesDir) {
		if err := os.RemoveAll(gamesDir); err != nil {
			logger.Debug("Failed to remove legacy games cache", "error", err)
		} else {
			logger.Debug("Removed legacy games cache directory")
		}
	}

	romsDir := filepath.Join(wd, ".cache", "roms")
	if fileutil.FileExists(romsDir) {
		if err := os.RemoveAll(romsDir); err != nil {
			logger.Debug("Failed to remove legacy roms cache", "error", err)
		} else {
			logger.Debug("Removed legacy roms cache directory")
		}
	}
}
