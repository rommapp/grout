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

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO collections
		(romm_id, virtual_id, type, name, rom_count, data_json, updated_at, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
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
