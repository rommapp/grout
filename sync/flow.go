package sync

import (
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/romm"
	"grout/version"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const maxConcurrentRequests = 8

func ResolveSaveSync(client *romm.Client, config *internal.Config, deviceID string) ([]SyncItem, error) {
	logger := gaba.GetLogger()
	logger.Debug("Starting save sync resolve", "deviceID", deviceID)

	localSaves := ScanSaves(config)
	logger.Debug("Scanned local saves", "count", len(localSaves))

	remoteSaves, err := FetchRemoteSaves(client, localSaves, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote saves: %w", err)
	}
	logger.Debug("Fetched remote saves", "count", len(remoteSaves))

	newSaves := LocalSavesWithoutRemote(localSaves, remoteSaves)
	logger.Debug("Local saves without remote", "count", len(newSaves))

	var allItems []SyncItem
	allItems = append(allItems, NewSaveUploadActions(newSaves, config)...)
	allItems = append(allItems, DetermineActions(localSaves, remoteSaves, deviceID, config)...)

	remoteOnly, err := DiscoverRemoteSaves(client, config, localSaves, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to discover remote saves: %w", err)
	}
	allItems = append(allItems, remoteOnly...)

	logger.Debug("Total sync items resolved", "count", len(allItems))
	return allItems, nil
}

func ExecuteSaveSync(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, progressFn func(current, total int)) SyncReport {
	report := ExecuteActions(client, config, deviceID, items, progressFn)

	cm := cache.GetCacheManager()
	if cm != nil {
		for _, item := range report.Items {
			if item.Action == ActionSkip || item.Action == ActionConflict || !item.Success {
				continue
			}
			fileName := item.LocalSave.FileName
			if fileName == "" && item.RemoteSave != nil {
				fileName = item.RemoteSave.FileName
			}
			record := cache.SaveSyncRecord{
				RomID:    item.LocalSave.RomID,
				RomName:  item.LocalSave.RomName,
				Action:   item.Action.String(),
				DeviceID: deviceID,
				FileName: fileName,
			}
			if item.RemoteSave != nil {
				record.SaveID = item.RemoteSave.ID
			}
			cm.RecordSaveSync(record)
		}
	}

	return report
}

func RegisterDevice(client *romm.Client, name string) (romm.Device, error) {
	return client.RegisterDevice(romm.RegisterDeviceRequest{
		Name:          name,
		Platform:      string(cfw.GetCFW()),
		Client:        "grout",
		ClientVersion: version.Get().Version,
	})
}

func ScanSaves(config *internal.Config) []LocalSave {
	logger := gaba.GetLogger()
	currentCFW := cfw.GetCFW()

	baseSavePath := cfw.BaseSavePath()
	if baseSavePath == "" {
		logger.Error("No save path for current CFW")
		return nil
	}

	emulatorMap := cfw.EmulatorFolderMap(currentCFW)
	if emulatorMap == nil {
		logger.Error("No emulator folder map for current CFW")
		return nil
	}

	cm := cache.GetCacheManager()
	if cm == nil {
		logger.Error("Cache manager not available for save scan")
		return nil
	}

	var saves []LocalSave

	logger.Debug("Starting save scan", "baseSavePath", baseSavePath, "platformCount", len(emulatorMap))

	for fsSlug, emulatorDirs := range emulatorMap {
		rommFSSlug := fsSlug
		if config != nil {
			rommFSSlug = config.ResolveRommFSSlug(fsSlug)
		}

		for _, emuDir := range emulatorDirs {
			saveDir := filepath.Join(baseSavePath, emuDir)

			if _, err := os.Stat(saveDir); os.IsNotExist(err) {
				continue
			}

			entries, err := os.ReadDir(saveDir)
			if err != nil {
				logger.Error("Could not read save directory", "path", saveDir, "error", err)
				continue
			}

			saveFileCount := 0
			for _, entry := range entries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}

				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if !ValidSaveExtensions[ext] {
					continue
				}

				saveFileCount++
				nameNoExt := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

				rom, err := cm.GetRomByFSLookup(rommFSSlug, nameNoExt)
				if err != nil {
					logger.Debug("No cache match for save file", "file", entry.Name(), "fsSlug", rommFSSlug, "nameNoExt", nameNoExt)
					continue
				}

				logger.Debug("Matched save to ROM", "file", entry.Name(), "romID", rom.ID, "romName", rom.Name)

				saves = append(saves, LocalSave{
					RomID:       rom.ID,
					RomName:     rom.Name,
					FSSlug:      rommFSSlug,
					FileName:    entry.Name(),
					FilePath:    filepath.Join(saveDir, entry.Name()),
					EmulatorDir: emuDir,
				})
			}

			if saveFileCount > 0 {
				logger.Debug("Scanned emulator directory", "path", saveDir, "saveFiles", saveFileCount)
			}
		}
	}

	logger.Debug("Completed save scan", "matched", len(saves))
	return saves
}

// FetchRemoteSaves fetches saves with device_id for each ROM that has a local save.
// This returns full save data including device_syncs for conflict detection.
func FetchRemoteSaves(client *romm.Client, localSaves []LocalSave, deviceID string) (map[int][]romm.Save, error) {
	logger := gaba.GetLogger()

	seen := make(map[int]bool)
	for _, ls := range localSaves {
		seen[ls.RomID] = true
	}

	romIDs := make([]int, 0, len(seen))
	for id := range seen {
		romIDs = append(romIDs, id)
	}

	logger.Debug("Fetching remote saves", "romCount", len(romIDs))

	type fetchResult struct {
		romID int
		saves []romm.Save
		err   error
	}

	results := make(chan fetchResult, len(romIDs))
	sem := make(chan struct{}, maxConcurrentRequests)
	var wg gosync.WaitGroup

	for _, romID := range romIDs {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			saves, err := client.GetSaves(romm.SaveQuery{RomID: id, DeviceID: deviceID})
			results <- fetchResult{romID: id, saves: saves, err: err}
		}(romID)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	result := make(map[int][]romm.Save)
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("rom %d: %w", r.romID, r.err)
		}
		if len(r.saves) > 0 {
			result[r.romID] = r.saves
			logger.Debug("Fetched remote saves", "romID", r.romID, "count", len(r.saves))
		}
	}

	return result, nil
}

func LocalSavesWithoutRemote(localSaves []LocalSave, remoteSaves map[int][]romm.Save) []LocalSave {
	var filtered []LocalSave
	for _, ls := range localSaves {
		if _, ok := remoteSaves[ls.RomID]; !ok {
			filtered = append(filtered, ls)
		}
	}
	return filtered
}

func NewSaveUploadActions(saves []LocalSave, config *internal.Config) []SyncItem {
	var items []SyncItem
	for _, ls := range saves {
		targetSlot := "default"
		if config != nil {
			targetSlot = config.GetSlotPreference(ls.RomID)
		}
		items = append(items, SyncItem{
			LocalSave:  ls,
			TargetSlot: targetSlot,
			Action:     ActionUpload,
		})
	}
	return items
}

func DetermineActions(localSaves []LocalSave, remoteSaves map[int][]romm.Save, deviceID string, config *internal.Config) []SyncItem {
	logger := gaba.GetLogger()
	var items []SyncItem

	for _, ls := range localSaves {
		saves, ok := remoteSaves[ls.RomID]
		if !ok {
			continue
		}

		preferredSlot := "default"
		if config != nil {
			preferredSlot = config.GetSlotPreference(ls.RomID)
		}

		remoteSave := selectSaveForSync(saves, preferredSlot)

		// Check if the selected save is actually in the preferred slot or a fallback
		remoteSlot := "default"
		if remoteSave != nil && remoteSave.Slot != nil {
			remoteSlot = *remoteSave.Slot
		}

		var action SyncAction
		if remoteSave != nil && remoteSlot != preferredSlot {
			// Fallback save from a different slot — don't compare against it.
			// Upload to populate the preferred slot instead.
			action = ActionUpload
			remoteSave = nil
		} else {
			action = determineAction(remoteSave, &ls, deviceID)
		}

		logger.Debug("Determined sync action",
			"romID", ls.RomID,
			"romName", ls.RomName,
			"action", action.String(),
			"preferredSlot", preferredSlot,
			"remoteSlot", remoteSlot,
		)

		items = append(items, SyncItem{
			LocalSave:  ls,
			RemoteSave: remoteSave,
			TargetSlot: preferredSlot,
			Action:     action,
		})
	}

	return items
}

func determineAction(remoteSave *romm.Save, localSave *LocalSave, deviceID string) SyncAction {
	logger := gaba.GetLogger()

	if remoteSave == nil {
		logger.Debug("No remote save found, will upload", "romID", localSave.RomID)
		return ActionUpload
	}

	localInfo, err := os.Stat(localSave.FilePath)
	if err != nil {
		logger.Debug("Cannot stat local save, will download", "path", localSave.FilePath, "error", err)
		return ActionDownload
	}
	// Truncate all times to second precision — the server may drop sub-second
	// precision between the upload response and subsequent fetches.
	localMtime := localInfo.ModTime().Truncate(time.Second)
	remoteUpdatedAt := remoteSave.UpdatedAt.Truncate(time.Second)

	for _, ds := range remoteSave.DeviceSyncs {
		if ds.DeviceID == deviceID {
			lastSyncedAt := ds.LastSyncedAt.Truncate(time.Second)
			localChanged := localMtime.After(lastSyncedAt)
			remoteChanged := remoteUpdatedAt.After(lastSyncedAt)

			if localChanged && remoteChanged {
				logger.Debug("Both local and remote changed since last sync, conflict",
					"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteUpdatedAt, "lastSyncedAt", lastSyncedAt)
				return ActionConflict
			}

			if ds.IsCurrent {
				if localMtime.After(remoteUpdatedAt) {
					logger.Debug("Device current, local newer, will upload",
						"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteUpdatedAt)
					return ActionUpload
				}
				logger.Debug("Device current, local not newer, skipping",
					"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteUpdatedAt)
				return ActionSkip
			}
			logger.Debug("Device in sync list but not current, will download",
				"romID", localSave.RomID, "deviceID", deviceID)
			return ActionDownload
		}
	}

	if localMtime.After(remoteUpdatedAt) {
		logger.Debug("Device not tracked, local newer, will upload",
			"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteUpdatedAt)
		return ActionUpload
	}
	if !localMtime.Before(remoteUpdatedAt) {
		logger.Debug("Device not tracked, mtime matches remote, skipping",
			"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteUpdatedAt)
		return ActionSkip
	}
	logger.Debug("Device not tracked, local older, will download",
		"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteUpdatedAt)
	return ActionDownload
}

// SelectSaveForSlot picks the latest save from the given slot.
// Falls back to the most recently updated save if the slot has no saves.
func SelectSaveForSlot(saves []romm.Save, preferredSlot string) *romm.Save {
	return selectSaveForSync(saves, preferredSlot)
}

// selectSaveForSync picks the latest save from the preferred slot.
// Falls back to the most recently updated save if the preferred slot has no saves.
func selectSaveForSync(saves []romm.Save, preferredSlot string) *romm.Save {
	if len(saves) == 0 {
		return nil
	}

	// Find the latest save in the preferred slot
	var best *romm.Save
	for i, s := range saves {
		slotName := "default"
		if s.Slot != nil {
			slotName = *s.Slot
		}
		if slotName != preferredSlot {
			continue
		}
		if best == nil || s.UpdatedAt.After(best.UpdatedAt) {
			best = &saves[i]
		}
	}
	if best != nil {
		return best
	}

	// Fallback: latest save across all slots
	best = &saves[0]
	for i := 1; i < len(saves); i++ {
		if saves[i].UpdatedAt.After(best.UpdatedAt) {
			best = &saves[i]
		}
	}
	return best
}

func ExecuteActions(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, progressFn func(current, total int)) SyncReport {
	logger := gaba.GetLogger()
	report := SyncReport{}

	actionable := 0
	for _, item := range items {
		if item.Action != ActionSkip && item.Action != ActionConflict {
			actionable++
		}
	}

	logger.Debug("Executing sync actions", "total", len(items), "actionable", actionable)

	current := 0
	for i := range items {
		item := &items[i]

		switch item.Action {
		case ActionUpload:
			current++
			if progressFn != nil {
				progressFn(current, actionable)
			}
			if upload(client, deviceID, item) {
				item.Success = true
				report.Uploaded++
			} else {
				report.Errors++
			}

		case ActionDownload:
			current++
			if progressFn != nil {
				progressFn(current, actionable)
			}
			if download(client, config, deviceID, item) {
				item.Success = true
				report.Downloaded++
			} else {
				report.Errors++
			}

		case ActionConflict:
			report.Conflicts++

		default:
			report.Skipped++
		}
	}

	report.Items = items
	logger.Debug("Sync execution complete", "uploaded", report.Uploaded, "downloaded", report.Downloaded, "skipped", report.Skipped, "errors", report.Errors)
	return report
}

func upload(client *romm.Client, deviceID string, item *SyncItem) bool {
	logger := gaba.GetLogger()

	logger.Debug("Uploading save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "file", item.LocalSave.FilePath)

	slot := "default"
	if item.TargetSlot != "" {
		slot = item.TargetSlot
	} else if item.RemoteSave != nil && item.RemoteSave.Slot != nil {
		slot = *item.RemoteSave.Slot
	}

	emulator := filepath.Base(item.LocalSave.EmulatorDir)
	if emulator == "." || emulator == "" {
		emulator = "unknown"
	}

	query := romm.UploadSaveQuery{
		RomID:     item.LocalSave.RomID,
		DeviceID:  deviceID,
		Emulator:  emulator,
		Slot:      slot,
		Overwrite: item.ForceOverwrite || item.RemoteSave != nil,
	}

	uploadedSave, err := client.UploadSaveWithQuery(query, item.LocalSave.FilePath)
	if err != nil {
		logger.Error("Failed to upload save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "error", err)
		return false
	}

	// Truncate to second precision — the server returns UpdatedAt without
	// sub-second precision on subsequent fetches, so local mtime must match.
	t := uploadedSave.UpdatedAt.Truncate(time.Second)
	if err := os.Chtimes(item.LocalSave.FilePath, t, t); err != nil {
		logger.Warn("Failed to set save file mtime after upload", "path", item.LocalSave.FilePath, "error", err)
	}

	if err := client.MarkDeviceSynced(uploadedSave.ID, deviceID); err != nil {
		logger.Warn("Failed to confirm upload sync state", "saveID", uploadedSave.ID, "error", err)
	}

	logger.Debug("Upload successful", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName)
	return true
}

func download(client *romm.Client, config *internal.Config, deviceID string, item *SyncItem) bool {
	logger := gaba.GetLogger()

	if item.RemoteSave == nil {
		logger.Error("No remote save to download", "romID", item.LocalSave.RomID)
		return false
	}

	logger.Debug("Downloading save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "saveID", item.RemoteSave.ID)

	if item.LocalSave.FilePath != "" {
		if info, err := os.Stat(item.LocalSave.FilePath); err == nil {
			backupDir := filepath.Join(filepath.Dir(item.LocalSave.FilePath), ".backup")
			ext := filepath.Ext(item.LocalSave.FileName)
			base := strings.TrimSuffix(item.LocalSave.FileName, ext)
			timestamp := info.ModTime().Format("2006-01-02 15-04-05")
			backupPath := filepath.Join(backupDir, fmt.Sprintf("%s [%s]%s", base, timestamp, ext))

			if err := os.MkdirAll(backupDir, 0755); err != nil {
				logger.Warn("Failed to create backup directory", "path", backupDir, "error", err)
			} else if err := fileutil.CopyFile(item.LocalSave.FilePath, backupPath); err != nil {
				logger.Warn("Failed to backup save before download", "path", item.LocalSave.FilePath, "error", err)
			} else {
				logger.Debug("Backed up save before download", "backup", backupPath)
				if config != nil && config.SaveBackupLimit > 0 {
					cleanupBackups(backupDir, base, config.SaveBackupLimit)
				}
			}
		}
	}

	data, err := client.DownloadSaveByID(item.RemoteSave.ID, deviceID, true)
	if err != nil {
		logger.Error("Failed to download save", "romID", item.LocalSave.RomID, "saveID", item.RemoteSave.ID, "error", err)
		return false
	}

	savePath := item.LocalSave.FilePath
	if savePath == "" {
		saveDir := ResolveSaveDirectory(item.LocalSave.FSSlug, config)
		if saveDir != "" {
			fileName := item.RemoteSave.FileName
			if item.LocalSave.RomFileName != "" {
				romNameNoExt := strings.TrimSuffix(item.LocalSave.RomFileName, filepath.Ext(item.LocalSave.RomFileName))
				fileName = romNameNoExt + "." + item.RemoteSave.FileExtension
			}
			savePath = filepath.Join(saveDir, fileName)
		}
	}
	if savePath == "" {
		logger.Error("Could not determine save path", "romID", item.LocalSave.RomID, "fsSlug", item.LocalSave.FSSlug)
		return false
	}

	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		logger.Error("Failed to create save directory", "path", filepath.Dir(savePath), "error", err)
		return false
	}

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		logger.Error("Failed to write save file", "path", savePath, "error", err)
		return false
	}

	t := item.RemoteSave.UpdatedAt.Truncate(time.Second)
	if err := os.Chtimes(savePath, t, t); err != nil {
		logger.Warn("Failed to set save file mtime", "path", savePath, "error", err)
	}

	if err := client.MarkDeviceSynced(item.RemoteSave.ID, deviceID); err != nil {
		logger.Warn("Failed to confirm save download", "saveID", item.RemoteSave.ID, "error", err)
	}

	logger.Debug("Download successful", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "path", savePath)
	return true
}

func DiscoverRemoteSaves(client *romm.Client, config *internal.Config, localSaves []LocalSave, deviceID string) ([]SyncItem, error) {
	logger := gaba.GetLogger()

	scan := cfw.ScanRoms(config)
	resolved := ResolveLocalRoms(scan)
	if len(resolved) == 0 {
		return nil, nil
	}

	coveredRomIDs := make(map[int]bool)
	for _, ls := range localSaves {
		coveredRomIDs[ls.RomID] = true
	}

	var uncoveredRomIDs []int
	for romID := range resolved {
		if !coveredRomIDs[romID] {
			uncoveredRomIDs = append(uncoveredRomIDs, romID)
		}
	}

	if len(uncoveredRomIDs) == 0 {
		logger.Debug("All local ROMs already have local saves")
		return nil, nil
	}

	logger.Debug("Checking remote saves for ROMs without local saves", "count", len(uncoveredRomIDs))

	type discoverResult struct {
		romID int
		saves []romm.Save
		err   error
	}

	results := make(chan discoverResult, len(uncoveredRomIDs))
	sem := make(chan struct{}, maxConcurrentRequests)
	var wg gosync.WaitGroup

	for _, romID := range uncoveredRomIDs {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			saves, err := client.GetSaves(romm.SaveQuery{RomID: id, DeviceID: deviceID})
			results <- discoverResult{romID: id, saves: saves, err: err}
		}(romID)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var items []SyncItem
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("rom %d: %w", r.romID, r.err)
		}

		preferredSlot := "default"
		if config != nil {
			preferredSlot = config.GetSlotPreference(r.romID)
		}
		remoteSave := selectSaveForSync(r.saves, preferredSlot)
		if remoteSave == nil {
			continue
		}

		rom := resolved[r.romID]
		logger.Debug("Found remote save for ROM without local save",
			"romID", r.romID, "romName", rom.RomName, "saveFile", remoteSave.FileName)

		item := SyncItem{
			LocalSave: LocalSave{
				RomID:       r.romID,
				RomName:     rom.RomName,
				FSSlug:      rom.FSSlug,
				RomFileName: rom.FileName,
			},
			RemoteSave: remoteSave,
			TargetSlot: preferredSlot,
			Action:     ActionDownload,
		}

		// Detect multiple distinct slots for first-time downloads
		slotSet := make(map[string]bool)
		for _, save := range r.saves {
			slot := "default"
			if save.Slot != nil {
				slot = *save.Slot
			}
			slotSet[slot] = true
		}
		if len(slotSet) > 1 {
			for slot := range slotSet {
				item.AvailableSlots = append(item.AvailableSlots, slot)
			}
			sort.Strings(item.AvailableSlots)
			item.AllRemoteSaves = r.saves
		}

		items = append(items, item)
	}

	logger.Debug("Remote-only saves to download", "count", len(items))
	return items, nil
}

func cleanupBackups(backupDir string, baseName string, limit int) {
	if limit <= 0 {
		return
	}

	logger := gaba.GetLogger()
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	// Collect backup files for this game (matching base name prefix)
	type backupFile struct {
		name    string
		modTime int64
	}
	var backups []backupFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), baseName+" [") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{name: e.Name(), modTime: info.ModTime().UnixNano()})
	}

	if len(backups) <= limit {
		return
	}

	// Sort oldest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime < backups[j].modTime
	})

	// Remove oldest until we're at the limit
	for i := 0; i < len(backups)-limit; i++ {
		path := filepath.Join(backupDir, backups[i].name)
		if err := os.Remove(path); err != nil {
			logger.Warn("Failed to remove old backup", "path", path, "error", err)
		} else {
			logger.Debug("Removed old backup", "path", path)
		}
	}
}

func ResolveSaveDirectory(fsSlug string, config *internal.Config) string {
	if config != nil && config.SaveDirectoryMappings != nil {
		if mapped, ok := config.SaveDirectoryMappings[fsSlug]; ok && mapped != "" {
			baseSavePath := cfw.BaseSavePath()
			if baseSavePath != "" {
				return filepath.Join(baseSavePath, mapped)
			}
		}
	}

	effectiveFSSlug := fsSlug
	if config != nil {
		effectiveFSSlug = config.ResolveFSSlug(fsSlug)
	}

	return cfw.GetSaveDirectory(effectiveFSSlug)
}
