package sync

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/internal/pspdb"
	"grout/romm"
	"grout/version"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func ResolveSaveSync(client *romm.Client, config *internal.Config, deviceID string) (SyncResult, error) {
	logger := gaba.GetLogger()
	logger.Debug("Starting save sync resolve (negotiate)", "deviceID", deviceID)

	localSaves := ScanSaves(config)
	logger.Debug("Scanned local saves", "count", len(localSaves))

	states := buildClientSaveStates(localSaves, config)

	resp, err := client.Negotiate(romm.SyncNegotiatePayload{
		DeviceID: deviceID,
		Saves:    states,
	})
	if err != nil {
		return SyncResult{}, fmt.Errorf("negotiate failed: %w", err)
	}
	logger.Debug("Negotiate response",
		"sessionID", resp.SessionID,
		"uploads", resp.TotalUpload,
		"downloads", resp.TotalDownload,
		"conflicts", resp.TotalConflict,
		"no_ops", resp.TotalNoOp,
	)

	scan := cfw.ScanRoms(config)
	resolvedRoms := ResolveLocalRoms(scan)
	cm := cache.GetCacheManager()

	items := mapOperationsToItems(resp.Operations, localSaves, resolvedRoms, cm, config)
	logger.Debug("Total sync items resolved", "count", len(items))

	return SyncResult{Items: items, SessionID: resp.SessionID}, nil
}

// mapOperationsToItems converts negotiate operations into executable SyncItems,
// dropping no_op. Order is preserved. Downloads with no local file are resolved
// from the local ROM scan / cache for path determination.
func mapOperationsToItems(
	ops []romm.SyncOperationSchema,
	localSaves []LocalSave,
	resolvedRoms map[int]cfw.LocalRomFile,
	cm *cache.Manager,
	config *internal.Config,
) []SyncItem {
	logger := gaba.GetLogger()

	type localKey struct {
		romID    int
		fileName string
	}
	byKey := make(map[localKey]LocalSave, len(localSaves))
	for _, ls := range localSaves {
		byKey[localKey{ls.RomID, ls.FileName}] = ls
	}

	items := make([]SyncItem, 0, len(ops))
	for _, op := range ops {
		switch op.Action {
		case "upload":
			ls, ok := byKey[localKey{op.RomID, op.FileName}]
			if !ok {
				logger.Warn("Negotiate upload for unknown local save", "romID", op.RomID, "file", op.FileName)
				continue
			}
			slot := "autosave"
			if config != nil {
				slot = config.GetSlotPreference(ls.RomID)
			}
			items = append(items, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				TargetSlot: slot,
				Action:     ActionUpload,
			})

		case "download":
			ls, ok := byKey[localKey{op.RomID, op.FileName}]
			if !ok {
				ls = resolveLocalSaveForDownload(op, resolvedRoms, cm, config)
			}
			slot := "autosave"
			if config != nil {
				slot = config.GetSlotPreference(op.RomID)
			}
			items = append(items, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				TargetSlot: slot,
				Action:     ActionDownload,
			})

		case "conflict":
			ls, ok := byKey[localKey{op.RomID, op.FileName}]
			if !ok {
				logger.Warn("Negotiate conflict for unknown local save", "romID", op.RomID, "file", op.FileName)
				continue
			}
			items = append(items, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				Action:     ActionConflict,
			})

		case "no_op":
			// nothing to do
		default:
			logger.Warn("Unknown negotiate action", "action", op.Action, "romID", op.RomID)
		}
	}
	return items
}

// buildRemoteSaveStub builds a *romm.Save from a negotiate operation for execution.
func buildRemoteSaveStub(op romm.SyncOperationSchema) *romm.Save {
	if op.SaveID == nil && op.ServerUpdatedAt == nil {
		return nil
	}
	save := &romm.Save{
		RomID:    op.RomID,
		FileName: op.FileName,
		Emulator: op.Emulator,
	}
	if op.SaveID != nil {
		save.ID = *op.SaveID
	}
	if op.Slot != nil {
		save.Slot = op.Slot
	}
	if op.ServerUpdatedAt != nil {
		save.UpdatedAt = *op.ServerUpdatedAt
	}
	if ext := filepath.Ext(op.FileName); ext != "" {
		save.FileExtension = strings.TrimPrefix(ext, ".")
	}
	return save
}

// resolveLocalSaveForDownload builds a LocalSave for a download whose file does
// not exist locally yet, resolving ROM metadata for path determination.
func resolveLocalSaveForDownload(op romm.SyncOperationSchema, resolvedRoms map[int]cfw.LocalRomFile, cm *cache.Manager, config *internal.Config) LocalSave {
	ls := LocalSave{RomID: op.RomID, FileName: op.FileName}

	if rom, ok := resolvedRoms[op.RomID]; ok {
		ls.RomName = rom.RomName
		ls.FSSlug = rom.FSSlug
		ls.RomFileName = rom.FileName
	} else if cm != nil {
		if roms, err := cm.GetGamesByIDs([]int{op.RomID}); err == nil && len(roms) > 0 {
			ls.RomName = roms[0].Name
			ls.FSSlug = roms[0].PlatformFSSlug
		}
	}

	if IsDirectorySavePlatform(ls.FSSlug) {
		ls.IsDirectorySave = true
	}
	return ls
}

func ExecuteSaveSync(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, sessionID int, progressFn func(current, total int)) SyncReport {
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

	if sessionID > 0 {
		if err := client.CompleteSession(sessionID, romm.SyncCompletePayload{
			OperationsCompleted: report.Uploaded + report.Downloaded,
			OperationsFailed:    report.Errors,
		}); err != nil {
			// On-demand client has no retry queue; the server expires stale sessions.
			gaba.GetLogger().Warn("Failed to complete sync session (leaving for server to expire)", "sessionID", sessionID, "error", err)
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
		SyncMode:      "api",
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
			logger.Debug("Checking save directory", "path", saveDir, "fsSlug", rommFSSlug)

			if _, err := os.Stat(saveDir); os.IsNotExist(err) {
				continue
			}

			entries, err := os.ReadDir(saveDir)
			if err != nil {
				logger.Error("Could not read save directory", "path", saveDir, "error", err)
				continue
			}

			if IsDirectorySavePlatform(fsSlug) {
				// Directory-based saves (e.g., PPSSPP): group all directories that
				// share the same Game ID and title into a single LocalSave, so that
				// DATA00/DATA01/INSDIR etc. are synced together as one zip.
				type pspGroup struct {
					title string
					dirs  []string
				}
				groups := make(map[string]*pspGroup)

				for _, entry := range entries {
					if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
						continue
					}
					gameID := extractPSPGameID(entry.Name())
					dirPath := filepath.Join(saveDir, entry.Name())

					if _, ok := groups[gameID]; !ok {
						groups[gameID] = &pspGroup{}
					}
					groups[gameID].dirs = append(groups[gameID].dirs, dirPath)

					if groups[gameID].title == "" {
						if title, ok := ReadPSPSaveTitle(dirPath); ok {
							groups[gameID].title = title
						}
					}
				}

				for gameID, group := range groups {
					// Normalize the Game ID to match pspdb keys (no hyphens or spaces)
					cleanGameID := strings.NewReplacer("-", "", " ", "").Replace(gameID)

					// Prefer the canonical title from pspdb, fall back to PARAM.SFO
					title, inDB := pspdb.Titles[cleanGameID]
					if !inDB {
						if group.title != "" {
							title = group.title
							logger.Debug("PSP game ID not in pspdb, using PARAM.SFO title", "gameID", gameID, "title", title)
						} else {
							logger.Debug("No title found for PSP game ID, skipping", "gameID", gameID, "fsSlug", rommFSSlug)
							continue
						}
					}

					rom, err := cm.GetRomByNameLookup(rommFSSlug, title)
					if err != nil {
						logger.Debug("No cache match for PSP save group", "gameID", gameID, "title", title, "inDB", inDB, "fsSlug", rommFSSlug)
						continue
					}

					sort.Strings(group.dirs)

					logger.Debug("Matched PSP save group to ROM", "gameID", gameID, "title", group.title, "dirCount", len(group.dirs), "romID", rom.ID, "romName", rom.Name)

					saves = append(saves, LocalSave{
						RomID:           rom.ID,
						RomName:         rom.Name,
						FSSlug:          rommFSSlug,
						FileName:        gameID + ".zip",
						FilePath:        group.dirs[0],
						EmulatorDir:     emuDir,
						IsDirectorySave: true,
						GameID:          gameID,
						RelatedDirs:     group.dirs,
					})
				}
			} else {
				// File-based saves: scan for individual save files
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
	}

	logger.Debug("Completed save scan", "matched", len(saves))
	return saves
}

// buildClientSaveStates converts scanned local saves into the negotiate payload,
// computing a content hash per save (composite for directory saves, MD5 for files)
// and the slot from the user's per-ROM preference (default "autosave").
func buildClientSaveStates(localSaves []LocalSave, config *internal.Config) []romm.ClientSaveState {
	logger := gaba.GetLogger()
	states := make([]romm.ClientSaveState, 0, len(localSaves))

	for _, ls := range localSaves {
		slot := "autosave"
		if config != nil {
			slot = config.GetSlotPreference(ls.RomID)
		}

		emulator := filepath.Base(ls.EmulatorDir)
		if emulator == "." || emulator == "" {
			emulator = ""
		} else if emulator == "SAVEDATA" {
			emulator = "PPSSPP"
		}

		var updatedAt time.Time
		var size int64

		if ls.IsDirectorySave {
			dirs := ls.RelatedDirs
			if len(dirs) == 0 {
				dirs = []string{ls.FilePath}
			}
			updatedAt, size = dirNewestMtimeAndSize(dirs)
		} else {
			info, err := os.Stat(ls.FilePath)
			if err != nil {
				logger.Warn("Cannot stat local save, skipping from negotiate", "path", ls.FilePath, "error", err)
				continue
			}
			updatedAt = info.ModTime().Truncate(time.Second)
			size = info.Size()
		}

		state := romm.ClientSaveState{
			RomID:         ls.RomID,
			FileName:      ls.FileName,
			Slot:          slot,
			Emulator:      emulator,
			UpdatedAt:     updatedAt,
			FileSizeBytes: size,
		}
		if hash, err := saveContentHash(ls); err == nil {
			state.ContentHash = hash
		} else {
			logger.Warn("Failed to hash local save; sending without content_hash", "romID", ls.RomID, "error", err)
		}

		states = append(states, state)
	}

	return states
}

// saveContentHash returns the server-compatible content hash for a local save.
func saveContentHash(ls LocalSave) (string, error) {
	if ls.IsDirectorySave {
		dirs := ls.RelatedDirs
		if len(dirs) == 0 {
			dirs = []string{ls.FilePath}
		}
		return fileutil.ComputeDirsCompositeHash(dirs)
	}
	return fileutil.ComputeMD5(ls.FilePath)
}

// dirNewestMtimeAndSize returns the newest file mtime (sec-truncated) and total
// byte size across the given directories.
func dirNewestMtimeAndSize(dirs []string) (time.Time, int64) {
	var newest time.Time
	var total int64
	for _, dir := range dirs {
		filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			total += info.Size()
			if mt := info.ModTime(); mt.After(newest) {
				newest = mt
			}
			return nil
		})
	}
	return newest.Truncate(time.Second), total
}

// SelectSaveForSlot picks the latest save in preferredSlot, falling back to the
// most recently updated save across all slots. Used by the multi-slot download UI.
func SelectSaveForSlot(saves []romm.Save, preferredSlot string) *romm.Save {
	if len(saves) == 0 {
		return nil
	}
	var best *romm.Save
	for i := range saves {
		slot := "autosave"
		if saves[i].Slot != nil {
			slot = *saves[i].Slot
		}
		if slot != preferredSlot {
			continue
		}
		if best == nil || saves[i].UpdatedAt.After(best.UpdatedAt) {
			best = &saves[i]
		}
	}
	if best != nil {
		return best
	}
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
			switch upload(client, deviceID, item) {
			case uploadOK:
				item.Success = true
				report.Uploaded++
			case uploadConflict:
				item.Action = ActionConflict
				report.Conflicts++
			default:
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

type uploadOutcome int

const (
	uploadErr uploadOutcome = iota
	uploadOK
	uploadConflict
)

func upload(client *romm.Client, deviceID string, item *SyncItem) uploadOutcome {
	logger := gaba.GetLogger()
	logger.Debug("Uploading save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "file", item.LocalSave.FilePath)

	slot := "autosave"
	if item.TargetSlot != "" {
		slot = item.TargetSlot
	} else if item.RemoteSave != nil && item.RemoteSave.Slot != nil {
		slot = *item.RemoteSave.Slot
	}

	emulator := filepath.Base(item.LocalSave.EmulatorDir)
	if emulator == "." || emulator == "" {
		emulator = "unknown"
	}
	if emulator == "SAVEDATA" { // PSP folder name varies across CFW
		emulator = "PPSSPP"
	}

	query := romm.UploadSaveQuery{
		RomID:     item.LocalSave.RomID,
		DeviceID:  deviceID,
		Emulator:  emulator,
		Slot:      slot,
		Overwrite: item.ForceOverwrite || item.RemoteSave != nil,
	}
	if slot == "autosave" {
		query.Autocleanup = true
		query.AutocleanupLimit = 10
	}

	uploadPath := item.LocalSave.FilePath
	if item.LocalSave.IsDirectorySave {
		dirs := item.LocalSave.RelatedDirs
		if len(dirs) == 0 {
			dirs = []string{item.LocalSave.FilePath}
		}
		zipPath, zipErr := ZipDirectories(dirs)
		if zipErr != nil {
			logger.Error("Failed to zip directory save", "gameID", item.LocalSave.GameID, "error", zipErr)
			return uploadErr
		}
		defer os.Remove(zipPath)
		uploadPath = zipPath
	}

	uploadedSave, err := client.UploadSaveWithQuery(query, uploadPath)
	if err != nil {
		if errors.Is(err, romm.ErrConflict) {
			logger.Warn("Upload rejected with 409; surfacing as conflict", "romID", item.LocalSave.RomID, "error", err)
			return uploadConflict
		}
		logger.Error("Failed to upload save", "romID", item.LocalSave.RomID, "error", err)
		return uploadErr
	}

	// Match server precision so the next scan doesn't see a spurious change.
	t := uploadedSave.UpdatedAt.Truncate(time.Second)
	if err := os.Chtimes(item.LocalSave.FilePath, t, t); err != nil {
		logger.Warn("Failed to set save mtime after upload", "path", item.LocalSave.FilePath, "error", err)
	}
	// No MarkDeviceSynced: the server upserts last_synced_at automatically on
	// upload because device_id is supplied.

	logger.Debug("Upload successful", "romID", item.LocalSave.RomID)
	return uploadOK
}

func download(client *romm.Client, config *internal.Config, deviceID string, item *SyncItem) bool {
	logger := gaba.GetLogger()

	if item.RemoteSave == nil {
		logger.Error("No remote save to download", "romID", item.LocalSave.RomID)
		return false
	}

	logger.Debug("Downloading save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "saveID", item.RemoteSave.ID)

	if !item.LocalSave.IsDirectorySave && IsDirectorySavePlatform(item.LocalSave.FSSlug) {
		item.LocalSave.IsDirectorySave = true
	}

	if item.LocalSave.FilePath != "" {
		if info, err := os.Stat(item.LocalSave.FilePath); err == nil {
			backupDir := filepath.Join(filepath.Dir(item.LocalSave.FilePath), ".backup")
			ext := filepath.Ext(item.LocalSave.FileName)
			base := strings.TrimSuffix(item.LocalSave.FileName, ext)
			timestamp := info.ModTime().Format("2006-01-02 15-04-05")
			backupPath := filepath.Join(backupDir, fmt.Sprintf("%s [%s]%s", base, timestamp, ext))

			if err := os.MkdirAll(backupDir, 0755); err != nil {
				logger.Error("Failed to create backup directory, aborting download", "path", backupDir, "error", err)
				return false
			}

			var backupErr error
			if item.LocalSave.IsDirectorySave {
				// Zip all related directories into the backup path
				dirs := item.LocalSave.RelatedDirs
				if len(dirs) == 0 {
					dirs = []string{item.LocalSave.FilePath}
				}
				zipPath, zipErr := ZipDirectories(dirs)
				if zipErr != nil {
					backupErr = zipErr
				} else {
					defer os.Remove(zipPath)
					backupErr = fileutil.CopyFile(zipPath, backupPath)
				}
			} else {
				backupErr = fileutil.CopyFile(item.LocalSave.FilePath, backupPath)
			}

			if backupErr != nil {
				logger.Error("Failed to backup save before download, aborting download", "path", item.LocalSave.FilePath, "error", backupErr)
				return false
			}

			logger.Debug("Backed up save before download", "backup", backupPath)
			if config != nil && config.SaveBackupLimit > 0 {
				cleanupBackups(backupDir, base, config.SaveBackupLimit)
			}
		}
	}

	data, err := client.DownloadSaveByID(item.RemoteSave.ID, deviceID, false)
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

	if item.LocalSave.IsDirectorySave {
		// Write zip to temp, then extract to the save directory
		tmpZip, err := os.CreateTemp("", "grout-save-dl-*.zip")
		if err != nil {
			logger.Error("Failed to create temp file for directory save", "error", err)
			return false
		}
		tmpZipPath := tmpZip.Name()
		defer os.Remove(tmpZipPath)

		if _, err := tmpZip.Write(data); err != nil {
			tmpZip.Close()
			logger.Error("Failed to write downloaded save zip", "error", err)
			return false
		}
		tmpZip.Close()

		// Remove all existing save directories before extracting.
		// For multi-dir PSP saves (DATA00, DATA01, INSDIR…), RelatedDirs holds
		// all of them; fall back to savePath for single-dir or remote-only cases.
		dirsToRemove := item.LocalSave.RelatedDirs
		if len(dirsToRemove) == 0 {
			dirsToRemove = []string{savePath}
		}
		for _, dir := range dirsToRemove {
			os.RemoveAll(dir)
		}

		if err := UnzipToDirectory(tmpZipPath, filepath.Dir(savePath)); err != nil {
			logger.Error("Failed to extract directory save", "path", savePath, "error", err)
			return false
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
			logger.Error("Failed to create save directory", "path", filepath.Dir(savePath), "error", err)
			return false
		}

		if err := os.WriteFile(savePath, data, 0644); err != nil {
			logger.Error("Failed to write save file", "path", savePath, "error", err)
			return false
		}
	}

	t := item.RemoteSave.UpdatedAt.Truncate(time.Second)
	if !item.LocalSave.IsDirectorySave {
		if err := os.Chtimes(savePath, t, t); err != nil {
			logger.Warn("Failed to set save file mtime", "path", savePath, "error", err)
		}
	}

	if err := client.ConfirmSaveDownloaded(item.RemoteSave.ID, deviceID); err != nil {
		logger.Warn("Failed to confirm save download", "saveID", item.RemoteSave.ID, "error", err)
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

// extractPSPGameID extracts the Game ID from a PSP save directory name.
// Two rules are applied in this order:
//  1. If the name contains an underscore, the Game ID is the part before it
//     (e.g. "UCUS98751_DATA00" → "UCUS98751", "UCUS98751_INSDIR" → "UCUS98751").
//  2. Otherwise, try to shrink prefixes from longest to shortest against
//     pspdb.Titles until a known Game ID is found
//     (e.g. "UCUS98653PROFILE00" → "UCUS98653").
//
// Falls back to the full directory name if no match is found.
func extractPSPGameID(dirName string) string {
	if idx := strings.Index(dirName, "_"); idx > 0 {
		return dirName[:idx]
	}
	for l := len(dirName) - 1; l > 0; l-- {
		if _, ok := pspdb.Titles[dirName[:l]]; ok {
			return dirName[:l]
		}
	}
	return dirName
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
