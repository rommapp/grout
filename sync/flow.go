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
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func ResolveSaveSync(client *romm.Client, config *internal.Config, deviceID string) ([]SyncItem, error) {
	logger := gaba.GetLogger()
	logger.Debug("Starting save sync resolve", "deviceID", deviceID)

	localSaves := ScanSaves(config)
	logger.Debug("Scanned local saves", "count", len(localSaves))

	remoteSaves := FetchRemoteSaves(client, localSaves, deviceID)
	logger.Debug("Fetched remote saves", "count", len(remoteSaves))

	newSaves := LocalSavesWithoutRemote(localSaves, remoteSaves)
	logger.Debug("Local saves without remote", "count", len(newSaves))

	var allItems []SyncItem
	allItems = append(allItems, NewSaveUploadActions(newSaves)...)
	allItems = append(allItems, DetermineActions(localSaves, remoteSaves, deviceID, config)...)

	remoteOnly := DiscoverRemoteSaves(client, config, localSaves, deviceID)
	allItems = append(allItems, remoteOnly...)

	logger.Debug("Total sync items resolved", "count", len(allItems))
	return allItems, nil
}

func ExecuteSaveSync(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, progressFn func(current, total int)) SyncReport {
	report := ExecuteActions(client, config, deviceID, items, progressFn)

	cm := cache.GetCacheManager()
	if cm != nil {
		for _, item := range report.Items {
			if item.Action == ActionSkip || item.Action == ActionConflict {
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
func FetchRemoteSaves(client *romm.Client, localSaves []LocalSave, deviceID string) map[int][]romm.Save {
	logger := gaba.GetLogger()
	result := make(map[int][]romm.Save)

	seen := make(map[int]bool)
	for _, ls := range localSaves {
		seen[ls.RomID] = true
	}

	logger.Debug("Fetching remote saves", "romCount", len(seen))

	for romID := range seen {
		saves, err := client.GetSaves(romm.SaveQuery{RomID: romID, DeviceID: deviceID})
		if err != nil {
			logger.Error("Failed to get saves", "romID", romID, "error", err)
			continue
		}
		if len(saves) > 0 {
			result[romID] = saves
			logger.Debug("Fetched remote saves", "romID", romID, "count", len(saves))
		}
	}

	return result
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

func NewSaveUploadActions(saves []LocalSave) []SyncItem {
	var items []SyncItem
	for _, ls := range saves {
		items = append(items, SyncItem{
			LocalSave: ls,
			Action:    ActionUpload,
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
		action := determineAction(remoteSave, &ls, deviceID)

		logger.Debug("Determined sync action",
			"romID", ls.RomID,
			"romName", ls.RomName,
			"action", action.String(),
		)

		items = append(items, SyncItem{
			LocalSave:  ls,
			RemoteSave: remoteSave,
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
	localMtime := localInfo.ModTime()

	for _, ds := range remoteSave.DeviceSyncs {
		if ds.DeviceID == deviceID {
			localChanged := localMtime.After(ds.LastSyncedAt)
			remoteChanged := remoteSave.UpdatedAt.After(ds.LastSyncedAt)

			if localChanged && remoteChanged {
				logger.Debug("Both local and remote changed since last sync, conflict",
					"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteSave.UpdatedAt, "lastSyncedAt", ds.LastSyncedAt)
				return ActionConflict
			}

			if ds.IsCurrent {
				if localMtime.After(remoteSave.UpdatedAt) {
					logger.Debug("Device current, local newer, will upload",
						"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteSave.UpdatedAt)
					return ActionUpload
				}
				logger.Debug("Device current, local not newer, skipping",
					"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteSave.UpdatedAt)
				return ActionSkip
			}
			logger.Debug("Device in sync list but not current, will download",
				"romID", localSave.RomID, "deviceID", deviceID)
			return ActionDownload
		}
	}

	if localMtime.After(remoteSave.UpdatedAt) {
		logger.Debug("Device not tracked, local newer, will upload",
			"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteSave.UpdatedAt)
		return ActionUpload
	}
	if !localMtime.Before(remoteSave.UpdatedAt) {
		logger.Debug("Device not tracked, mtime matches remote, skipping",
			"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteSave.UpdatedAt)
		return ActionSkip
	}
	logger.Debug("Device not tracked, local older, will download",
		"romID", localSave.RomID, "localMtime", localMtime, "remoteUpdatedAt", remoteSave.UpdatedAt)
	return ActionDownload
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
	if item.RemoteSave != nil && item.RemoteSave.Slot != nil {
		slot = *item.RemoteSave.Slot
	}

	emulator := filepath.Base(item.LocalSave.EmulatorDir)

	query := romm.UploadSaveQuery{
		RomID:     item.LocalSave.RomID,
		DeviceID:  deviceID,
		Emulator:  emulator,
		Slot:      slot,
		Overwrite: item.ForceOverwrite,
	}

	uploadedSave, err := client.UploadSaveWithQuery(query, item.LocalSave.FilePath)
	if err != nil {
		logger.Error("Failed to upload save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "error", err)
		return false
	}

	if err := os.Chtimes(item.LocalSave.FilePath, uploadedSave.UpdatedAt, uploadedSave.UpdatedAt); err != nil {
		logger.Warn("Failed to set save file mtime after upload", "path", item.LocalSave.FilePath, "error", err)
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

	if err := os.Chtimes(savePath, item.RemoteSave.UpdatedAt, item.RemoteSave.UpdatedAt); err != nil {
		logger.Warn("Failed to set save file mtime", "path", savePath, "error", err)
	}

	if err := client.ConfirmSaveDownloaded(item.RemoteSave.ID, deviceID); err != nil {
		logger.Warn("Failed to confirm save download", "saveID", item.RemoteSave.ID, "error", err)
	}

	logger.Debug("Download successful", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "path", savePath)
	return true
}

func DiscoverRemoteSaves(client *romm.Client, config *internal.Config, localSaves []LocalSave, deviceID string) []SyncItem {
	logger := gaba.GetLogger()

	scan := cfw.ScanRoms(config)
	resolved := ResolveLocalRoms(scan)
	if len(resolved) == 0 {
		return nil
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
		return nil
	}

	logger.Debug("Checking remote saves for ROMs without local saves", "count", len(uncoveredRomIDs))

	var items []SyncItem
	for _, romID := range uncoveredRomIDs {
		saves, err := client.GetSaves(romm.SaveQuery{RomID: romID, DeviceID: deviceID})
		if err != nil {
			logger.Debug("Failed to get saves", "romID", romID, "error", err)
			continue
		}

		preferredSlot := "default"
		if config != nil {
			preferredSlot = config.GetSlotPreference(romID)
		}
		remoteSave := selectSaveForSync(saves, preferredSlot)
		if remoteSave == nil {
			continue
		}

		rom := resolved[romID]
		logger.Debug("Found remote save for ROM without local save",
			"romID", romID, "romName", rom.RomName, "saveFile", remoteSave.FileName)

		items = append(items, SyncItem{
			LocalSave: LocalSave{
				RomID:       romID,
				RomName:     rom.RomName,
				FSSlug:      rom.FSSlug,
				RomFileName: rom.FileName,
			},
			RemoteSave: remoteSave,
			Action:     ActionDownload,
		})
	}

	logger.Debug("Remote-only saves to download", "count", len(items))
	return items
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
