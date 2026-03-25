package sync

import (
	"fmt"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/romm"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func ExecuteActions(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, sessionID int, progressFn func(current, total int)) SyncReport {
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
			if upload(client, deviceID, sessionID, item) {
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
			if download(client, config, deviceID, sessionID, item) {
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

func upload(client *romm.Client, deviceID string, sessionID int, item *SyncItem) bool {
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
		SessionID: sessionID,
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

	logger.Debug("Upload successful", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName)
	return true
}

func download(client *romm.Client, config *internal.Config, deviceID string, sessionID int, item *SyncItem) bool {
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

	data, err := client.DownloadSaveByID(item.RemoteSave.ID, deviceID, true, sessionID)
	if err != nil {
		logger.Error("Failed to download save", "romID", item.LocalSave.RomID, "saveID", item.RemoteSave.ID, "error", err)
		return false
	}

	savePath := item.LocalSave.FilePath
	if savePath == "" {
		saveDir := resolveSaveDirectory(item.LocalSave.FSSlug, config)
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

	logger.Debug("Download successful", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "path", savePath)
	return true
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
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[j].modTime < backups[i].modTime {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	for i := 0; i < len(backups)-limit; i++ {
		path := filepath.Join(backupDir, backups[i].name)
		if err := os.Remove(path); err != nil {
			logger.Warn("Failed to remove old backup", "path", path, "error", err)
		} else {
			logger.Debug("Removed old backup", "path", path)
		}
	}
}

func resolveSaveDirectory(fsSlug string, config *internal.Config) string {
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
