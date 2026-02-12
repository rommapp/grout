package cache

import (
	"encoding/json"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func (cm *Manager) GetCollections() ([]romm.Collection, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT data_json FROM collections ORDER BY name
	`)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "collections", "", err)
	}
	defer rows.Close()

	var collections []romm.Collection
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "collections", "", err)
		}

		var collection romm.Collection
		if err := json.Unmarshal([]byte(dataJSON), &collection); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "collections", "", err)
		}
		collections = append(collections, collection)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "collections", "", err)
	}

	if len(collections) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return collections, nil
}

func (cm *Manager) GetCollectionsByType(collType string) ([]romm.Collection, error) {
	if cm == nil || !cm.initialized {
		return nil, ErrNotInitialized
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT data_json FROM collections WHERE type = ? ORDER BY name
	`, collType)
	if err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "collections", collType, err)
	}
	defer rows.Close()

	var collections []romm.Collection
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "collections", collType, err)
		}

		var collection romm.Collection
		if err := json.Unmarshal([]byte(dataJSON), &collection); err != nil {
			cm.stats.recordError()
			return nil, newCacheError("get", "collections", collType, err)
		}
		collections = append(collections, collection)
	}

	if err := rows.Err(); err != nil {
		cm.stats.recordError()
		return nil, newCacheError("get", "collections", collType, err)
	}

	if len(collections) > 0 {
		cm.stats.recordHit()
	} else {
		cm.stats.recordMiss()
	}

	return collections, nil
}

// PurgeDeletedCollections removes cached regular collections whose RomM IDs are
// not in the provided list of valid IDs from the server. Also cleans up
// game_collections mappings for the deleted collections.
func (cm *Manager) PurgeDeletedCollections(validIDs []int) (int64, error) {
	if cm == nil || !cm.initialized {
		return 0, ErrNotInitialized
	}
	if len(validIDs) == 0 {
		return 0, nil
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return 0, newCacheError("purge", "collections", "", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("CREATE TEMP TABLE _valid_collection_ids (id INTEGER PRIMARY KEY)"); err != nil {
		return 0, newCacheError("purge", "collections", "", err)
	}

	const batchSize = 400
	for i := 0; i < len(validIDs); i += batchSize {
		end := i + batchSize
		if end > len(validIDs) {
			end = len(validIDs)
		}
		batch := validIDs[i:end]

		query := "INSERT OR IGNORE INTO _valid_collection_ids (id) VALUES "
		args := make([]any, len(batch))
		for j, id := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?)"
			args[j] = id
		}

		if _, err := tx.Exec(query, args...); err != nil {
			return 0, newCacheError("purge", "collections", "", err)
		}
	}

	// Delete game_collections for collections that will be removed
	if _, err := tx.Exec(`
		DELETE FROM game_collections WHERE collection_id IN (
			SELECT id FROM collections WHERE type = 'regular' AND romm_id NOT IN (SELECT id FROM _valid_collection_ids)
		)
	`); err != nil {
		return 0, newCacheError("purge", "collections", "game_collections", err)
	}

	result, err := tx.Exec("DELETE FROM collections WHERE type = 'regular' AND romm_id NOT IN (SELECT id FROM _valid_collection_ids)")
	if err != nil {
		return 0, newCacheError("purge", "collections", "", err)
	}

	deleted, _ := result.RowsAffected()

	tx.Exec("DROP TABLE IF EXISTS _valid_collection_ids")

	if err := tx.Commit(); err != nil {
		return 0, newCacheError("purge", "collections", "", err)
	}

	if deleted > 0 {
		logger.Debug("Purged deleted collections from cache", "count", deleted)
	}

	return deleted, nil
}

func (cm *Manager) SaveCollections(collections []romm.Collection) error {
	if cm == nil || !cm.initialized {
		return ErrNotInitialized
	}

	logger := gaba.GetLogger()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	tx, err := cm.db.Begin()
	if err != nil {
		return newCacheError("save", "collections", "", err)
	}
	defer tx.Rollback()

	now := nowUTC()
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO collections
		(romm_id, virtual_id, type, name, rom_count, data_json, updated_at, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return newCacheError("save", "collections", "", err)
	}
	defer stmt.Close()

	for _, c := range collections {
		dataJSON, err := json.Marshal(c)
		if err != nil {
			return newCacheError("save", "collections", c.Name, err)
		}

		collType := "regular"
		if c.IsVirtual {
			collType = "virtual"
		} else if c.IsSmart {
			collType = "smart"
		}

		var virtualID interface{}
		var rommID interface{}

		if c.IsVirtual {
			virtualID = c.VirtualID
			rommID = nil
		} else {
			virtualID = nil
			rommID = c.ID
		}

		_, err = stmt.Exec(
			rommID,
			virtualID,
			collType,
			c.Name,
			c.ROMCount,
			string(dataJSON),
			c.UpdatedAt,
			now,
		)
		if err != nil {
			return newCacheError("save", "collections", c.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return newCacheError("save", "collections", "", err)
	}

	logger.Debug("Saved collections to cache", "count", len(collections))
	return nil
}
