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
