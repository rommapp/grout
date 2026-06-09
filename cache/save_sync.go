package cache

import (
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveSyncRecord struct {
	ID       int
	RomID    int
	RomName  string
	Action   string
	DeviceID string
	SaveID   int
	FileName string
	SyncedAt time.Time
}

func (cm *Manager) RecordSaveSync(record SaveSyncRecord) error {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, err := cm.db.Exec(`
		INSERT INTO save_sync_history (rom_id, rom_name, action, device_id, save_id, file_name, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.RomID, record.RomName, record.Action, record.DeviceID, record.SaveID, record.FileName, nowUTC())

	if err != nil {
		gaba.GetLogger().Error("Failed to record save sync", "romID", record.RomID, "error", err)
	}
	return err
}

// SaveSyncState is the current synced state of one local save (one row per
// device+rom+file). Upserted after each successful upload/download.
type SaveSyncState struct {
	RomID       int
	FileName    string
	Slot        string
	SaveID      int
	ContentHash string
	SyncedAt    time.Time
}

// UpsertSaveState records (or updates) the current synced state for a local save.
func (cm *Manager) UpsertSaveState(deviceID string, state SaveSyncState) error {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	_, err := cm.db.Exec(`
		INSERT INTO save_sync_state (device_id, rom_id, file_name, slot, save_id, content_hash, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id, rom_id, file_name) DO UPDATE SET
			slot = excluded.slot,
			save_id = excluded.save_id,
			content_hash = excluded.content_hash,
			synced_at = excluded.synced_at
	`, deviceID, state.RomID, state.FileName, state.Slot, state.SaveID, state.ContentHash, nowUTC())

	if err != nil {
		gaba.GetLogger().Error("Failed to upsert save sync state", "romID", state.RomID, "file", state.FileName, "error", err)
	}
	return err
}

// GetSaveStates returns all current save-sync states for a device.
func (cm *Manager) GetSaveStates(deviceID string) []SaveSyncState {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT rom_id, file_name, slot, save_id, content_hash, synced_at
		FROM save_sync_state
		WHERE device_id = ?
	`, deviceID)
	if err != nil {
		gaba.GetLogger().Error("Failed to get save sync states", "error", err)
		return nil
	}
	defer rows.Close()

	var states []SaveSyncState
	for rows.Next() {
		var s SaveSyncState
		var saveID *int
		var contentHash *string
		var syncedAt string
		if err := rows.Scan(&s.RomID, &s.FileName, &s.Slot, &saveID, &contentHash, &syncedAt); err != nil {
			continue
		}
		if saveID != nil {
			s.SaveID = *saveID
		}
		if contentHash != nil {
			s.ContentHash = *contentHash
		}
		if parsed, err := time.Parse(time.RFC3339, syncedAt); err == nil {
			s.SyncedAt = parsed
		}
		states = append(states, s)
	}
	if err := rows.Err(); err != nil {
		gaba.GetLogger().Error("Error iterating save sync state rows", "error", err)
	}
	return states
}

func (cm *Manager) GetSaveSyncHistory(deviceID string) []SaveSyncRecord {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT id, rom_id, rom_name, action, device_id, save_id, file_name, synced_at
		FROM save_sync_history
		WHERE device_id = ?
		ORDER BY synced_at DESC
	`, deviceID)
	if err != nil {
		gaba.GetLogger().Error("Failed to get save sync history", "error", err)
		return nil
	}
	defer rows.Close()

	var records []SaveSyncRecord
	for rows.Next() {
		var r SaveSyncRecord
		var syncedAt string
		if err := rows.Scan(&r.ID, &r.RomID, &r.RomName, &r.Action, &r.DeviceID, &r.SaveID, &r.FileName, &syncedAt); err != nil {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, syncedAt)
		if err != nil {
			gaba.GetLogger().Warn("Failed to parse synced_at timestamp", "value", syncedAt, "error", err)
		} else {
			r.SyncedAt = parsed
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		gaba.GetLogger().Error("Error iterating save sync history rows", "error", err)
	}
	return records
}

func (cm *Manager) GetSyncedRomIDs(deviceID string) []int {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	rows, err := cm.db.Query(`
		SELECT DISTINCT rom_id FROM save_sync_history WHERE device_id = ?
	`, deviceID)
	if err != nil {
		gaba.GetLogger().Error("Failed to get synced rom IDs", "error", err)
		return nil
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		gaba.GetLogger().Error("Error iterating synced rom ID rows", "error", err)
	}
	return ids
}

func (cm *Manager) GetLastSyncTime(deviceID string) *time.Time {
	if cm == nil || !cm.initialized {
		return nil
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var syncedAt string
	err := cm.db.QueryRow(`
		SELECT synced_at FROM save_sync_history
		WHERE device_id = ?
		ORDER BY synced_at DESC
		LIMIT 1
	`, deviceID).Scan(&syncedAt)
	if err != nil {
		return nil
	}

	t, err := time.Parse(time.RFC3339, syncedAt)
	if err != nil {
		return nil
	}
	return &t
}
