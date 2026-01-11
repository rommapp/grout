package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"grout/internal/stringutil"
	"grout/romm"
	"strconv"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type Type string

const (
	Platform          Type = "platform"
	Collection        Type = "collection"
	SmartCollection   Type = "smart_collection"
	VirtualCollection Type = "virtual_collection"
)

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

func (cm *Manager) GetPlatformGames(platformID int) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT data_json FROM games WHERE platform_id = ? ORDER BY name
	`, platformID)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetPlatformCacheKey(platformID), err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

func (cm *Manager) SavePlatformGames(platformID int, games []romm.Rom) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO games (id, platform_id, platform_fs_slug, name, fs_name, fs_name_no_ext, crc_hash, md5_hash, sha1_hash, data_json, updated_at, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, game := range games {
		dataJSON, err := json.Marshal(game)
		if err != nil {
			return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
		}

		_, err = stmt.Exec(
			game.ID,
			game.PlatformID,
			game.PlatformFSSlug,
			game.Name,
			game.FsName,
			game.FsNameNoExt,
			game.CrcHash,
			game.Md5Hash,
			game.Sha1Hash,
			string(dataJSON),
			game.UpdatedAt,
			now,
		)
		if err != nil {
			return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "games", GetPlatformCacheKey(platformID), err)
	}

	logger.Debug("Saved platform games to cache", "platformID", platformID, "count", len(games))
	return nil
}

func (cm *Manager) GetCollectionGames(collection romm.Collection) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	collectionID, err := cm.getCollectionInternalID(collection)
	if err != nil {
		return nil, err
	}

	rows, err := cm.db.Query(`
		SELECT g.data_json FROM games g
		INNER JOIN game_collections gc ON g.id = gc.game_id
		WHERE gc.collection_id = ?
		ORDER BY g.name
	`, collectionID)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", GetCollectionCacheKey(collection), err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

func (cm *Manager) SaveCollectionGames(collection romm.Collection, games []romm.Rom) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	if len(games) == 0 {
		return nil
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Get the collection's internal ID (collection should already be saved)
	collectionID, err := cm.getCollectionInternalIDLocked(collection)
	if err != nil {
		return err
	}

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
	}
	defer tx.Rollback()

	// Delete existing game-collection mappings for this collection
	_, err = tx.Exec(`DELETE FROM game_collections WHERE collection_id = ?`, collectionID)
	if err != nil {
		return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
	}

	// Batch insert mappings for better performance
	// SQLite supports up to 999 variables, so batch in groups
	const batchSize = 400 // 2 params per row = 800 variables max
	for i := 0; i < len(games); i += batchSize {
		end := i + batchSize
		if end > len(games) {
			end = len(games)
		}
		batch := games[i:end]

		// Build batch insert query
		query := "INSERT OR IGNORE INTO game_collections (game_id, collection_id) VALUES "
		args := make([]interface{}, 0, len(batch)*2)
		for j, game := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?)"
			args = append(args, game.ID, collectionID)
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "games", GetCollectionCacheKey(collection), err)
	}

	logger.Debug("Saved collection game mappings", "collection", collection.Name, "count", len(games))
	return nil
}

func (cm *Manager) getCollectionInternalID(collection romm.Collection) (int64, error) {
	var id int64
	var err error

	if collection.IsVirtual {
		err = cm.db.QueryRow(`SELECT id FROM collections WHERE virtual_id = ?`, collection.VirtualID).Scan(&id)
	} else {
		collType := "regular"
		if collection.IsSmart {
			collType = "smart"
		}
		err = cm.db.QueryRow(`SELECT id FROM collections WHERE romm_id = ? AND type = ?`, collection.ID, collType).Scan(&id)
	}

	if err == sql.ErrNoRows {
		cm.stats.recordMiss()
		return 0, ErrCacheMiss
	}
	if err != nil {
		cm.stats.recordError()
		return 0, newCacheError("get", "collections", GetCollectionCacheKey(collection), err)
	}

	return id, nil
}

func (cm *Manager) getCollectionInternalIDLocked(collection romm.Collection) (int64, error) {
	return cm.getCollectionInternalID(collection)
}

func (cm *Manager) SaveAllCollectionMappings(collections []romm.Collection) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	if len(collections) == 0 {
		return nil
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "collection_mappings", "", err)
	}
	defer tx.Rollback()

	// Clear all existing mappings
	if _, err := tx.Exec(`DELETE FROM game_collections`); err != nil {
		return newCacheError("save", "collection_mappings", "", err)
	}

	// Build a map of collection identifiers to internal IDs
	collectionIDs := make(map[string]int64)
	for _, coll := range collections {
		var id int64
		var err error

		if coll.IsVirtual {
			err = tx.QueryRow(`SELECT id FROM collections WHERE virtual_id = ?`, coll.VirtualID).Scan(&id)
		} else {
			collType := "regular"
			if coll.IsSmart {
				collType = "smart"
			}
			err = tx.QueryRow(`SELECT id FROM collections WHERE romm_id = ? AND type = ?`, coll.ID, collType).Scan(&id)
		}

		if err == nil {
			collectionIDs[GetCollectionCacheKey(coll)] = id
		}
	}

	// Batch insert all mappings
	const batchSize = 400
	var allMappings []struct {
		gameID       int
		collectionID int64
	}

	for _, coll := range collections {
		collID, ok := collectionIDs[GetCollectionCacheKey(coll)]
		if !ok {
			continue
		}
		for _, romID := range coll.ROMIDs {
			allMappings = append(allMappings, struct {
				gameID       int
				collectionID int64
			}{romID, collID})
		}
	}

	for i := 0; i < len(allMappings); i += batchSize {
		end := i + batchSize
		if end > len(allMappings) {
			end = len(allMappings)
		}
		batch := allMappings[i:end]

		query := "INSERT OR IGNORE INTO game_collections (game_id, collection_id) VALUES "
		args := make([]interface{}, 0, len(batch)*2)
		for j, m := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?)"
			args = append(args, m.gameID, m.collectionID)
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return newCacheError("save", "collection_mappings", "", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "collection_mappings", "", err)
	}

	logger.Debug("Saved all collection mappings", "collections", len(collections), "mappings", len(allMappings))
	return nil
}

func (cm *Manager) GetGamesByIDs(gameIDs []int) ([]romm.Rom, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	if len(gameIDs) == 0 {
		return nil, nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Build query with placeholders
	placeholders := make([]string, len(gameIDs))
	args := make([]interface{}, len(gameIDs))
	for i, id := range gameIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT data_json FROM games WHERE id IN (" + strings.Join(placeholders, ",") + ") ORDER BY name"

	rows, err := cm.db.Query(query, args...)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", "batch", err)
	}
	defer rows.Close()

	var games []romm.Rom
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", "batch", err)
		}

		var game romm.Rom
		if err := json.Unmarshal([]byte(dataJSON), &game); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "games", "batch", err)
		}
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "games", "batch", err)
	}

	if len(games) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return games, nil
}

func (cm *Manager) GetCachedGameIDs() map[int]bool {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`SELECT id FROM games`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	gameIDs := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			gameIDs[id] = true
		}
	}

	return gameIDs
}

func (cm *Manager) GetRomIDByFilename(fsSlug, filename string) (int, string, bool) {
	if cm == nil || !cm.initialized {
		return 0, "", false
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	key := stringutil.StripExtension(filename)

	var romID int
	var romName string

	// First, try to find in the games table (exact filename match from RomM)
	err := cm.db.QueryRow(`
		SELECT id, name FROM games
		WHERE platform_fs_slug = ? AND fs_name_no_ext = ?
	`, fsSlug, key).Scan(&romID, &romName)

	if err == nil {
		cm.stats.recordHit()
		return romID, romName, true
	}

	if err != sql.ErrNoRows {
		cm.stats.recordError()
		gaba.GetLogger().Debug("ROM lookup error", "fsSlug", fsSlug, "filename", filename, "error", err)
		return 0, "", false
	}

	// Fallback: check filename_mappings table for orphan matches
	err = cm.db.QueryRow(`
		SELECT rom_id, rom_name FROM filename_mappings
		WHERE platform_fs_slug = ? AND local_filename_no_ext = ?
	`, fsSlug, key).Scan(&romID, &romName)

	if err == sql.ErrNoRows {
		cm.stats.recordMiss()
		return 0, "", false
	}
	if err != nil {
		cm.stats.recordError()
		gaba.GetLogger().Debug("Filename mapping lookup error", "fsSlug", fsSlug, "filename", filename, "error", err)
		return 0, "", false
	}

	cm.stats.recordHit()
	return romID, romName, true
}

// SaveFilenameMapping saves a mapping between a local filename and a RomM ROM ID.
// This is used when orphan ROMs are matched by hash to remember the association.
func (cm *Manager) SaveFilenameMapping(fsSlug, localFilename string, romID int, romName string) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	key := stringutil.StripExtension(localFilename)

	_, err := cm.db.Exec(`
		INSERT OR REPLACE INTO filename_mappings (platform_fs_slug, local_filename_no_ext, rom_id, rom_name, matched_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, fsSlug, key, romID, romName)

	if err != nil {
		return newCacheError("save", "filename_mapping", fmt.Sprintf("%s/%s", fsSlug, key), err)
	}

	logger.Debug("Saved filename mapping", "fsSlug", fsSlug, "localFilename", key, "romID", romID, "romName", romName)
	return nil
}

func (cm *Manager) GetRomByHash(md5, sha1, crc string) (int, string, bool) {
	if cm == nil || !cm.initialized {
		return 0, "", false
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var romID int
	var romName string

	if md5 != "" {
		err := cm.db.QueryRow(`SELECT id, name FROM games WHERE md5_hash = ?`, md5).Scan(&romID, &romName)
		if err == nil {
			cm.stats.recordHit()
			return romID, romName, true
		}
	}

	if sha1 != "" {
		err := cm.db.QueryRow(`SELECT id, name FROM games WHERE sha1_hash = ?`, sha1).Scan(&romID, &romName)
		if err == nil {
			cm.stats.recordHit()
			return romID, romName, true
		}
	}

	if crc != "" {
		err := cm.db.QueryRow(`SELECT id, name FROM games WHERE crc_hash = ?`, crc).Scan(&romID, &romName)
		if err == nil {
			cm.stats.recordHit()
			return romID, romName, true
		}
	}

	cm.stats.recordMiss()
	return 0, "", false
}

func GetCachedRomIDByFilename(fsSlug, filename string) (int, string, bool) {
	cm := GetCacheManager()
	if cm == nil {
		return 0, "", false
	}
	return cm.GetRomIDByFilename(fsSlug, filename)
}

// SaveFilenameMapping is a public wrapper to save a filename-to-ROM mapping
func SaveFilenameMapping(fsSlug, localFilename string, romID int, romName string) error {
	cm := GetCacheManager()
	if cm == nil {
		return ErrNotInitialized
	}
	return cm.SaveFilenameMapping(fsSlug, localFilename, romID, romName)
}
