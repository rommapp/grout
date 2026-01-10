package sync

import (
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/romm"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveSync struct {
	RomID    int
	RomName  string
	FSSlug   string
	GameBase string
	Local    *LocalSave
	Remote   romm.Save
	Action   SyncAction
}

type SyncAction string

const (
	Download SyncAction = "DOWNLOAD"
	Upload   SyncAction = "UPLOAD"
	Skip     SyncAction = "SKIP"
)

type SyncResult struct {
	GameName       string
	RomDisplayName string
	Action         SyncAction
	Success        bool
	Error          string
	FilePath       string
	UnmatchedSaves []UnmatchedSave
}

type UnmatchedSave struct {
	SavePath string
	FSSlug   string
}

func (s *SaveSync) Execute(host romm.Host, config *internal.Config) SyncResult {
	logger := gaba.GetLogger()

	// Strip file extension from ROM name for cleaner display
	displayName := s.RomName
	if displayName != "" {
		displayName = strings.TrimSuffix(displayName, filepath.Ext(displayName))
	}

	result := SyncResult{
		GameName:       s.GameBase,
		RomDisplayName: displayName,
		Action:         s.Action,
		Success:        false,
	}

	logger.Debug("Executing sync",
		"action", s.Action,
		"gameBase", s.GameBase,
		"romName", s.RomName,
		"romID", s.RomID)

	var err error
	switch s.Action {
	case Upload:
		result.FilePath, err = s.upload(host, config)
		logger.Debug("Upload complete", "filePath", result.FilePath, "err", err)
	case Download:
		if s.Local != nil {
			err = s.Local.backup()
			if err != nil {
				result.Error = err.Error()
				return result
			}
		}
		result.FilePath, err = s.download(host, config)
	case Skip:
		result.Success = true
		return result
	}

	if err != nil {
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	return result
}

func (s *SaveSync) download(host romm.Host, config *internal.Config) (string, error) {
	logger := gaba.GetLogger()
	if config == nil {
		return "", fmt.Errorf("config is nil")
	}
	rc := romm.NewClientFromHost(host, config.ApiTimeout)

	logger.Debug("Downloading save", "saveID", s.Remote.ID, "downloadPath", s.Remote.DownloadPath)

	saveData, err := rc.DownloadSave(s.Remote.DownloadPath)
	if err != nil {
		return "", fmt.Errorf("failed to download save: %w", err)
	}

	var destDir string
	if s.Local != nil {
		// If there's already a local save, use its directory
		destDir = filepath.Dir(s.Local.Path)
	} else {
		var err error
		destDir, err = ResolveSavePath(s.FSSlug, s.RomID, config)
		if err != nil {
			return "", fmt.Errorf("cannot determine save location: %w", err)
		}
	}

	ext := normalizeExt(s.Remote.FileExtension)
	filename := s.GameBase + ext
	destPath := filepath.Join(destDir, filename)

	if s.Local != nil && s.Local.Path != destPath {
		defer func() { _ = os.Remove(s.Local.Path) }()
	}

	err = os.WriteFile(destPath, saveData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write save file: %w", err)
	}

	err = os.Chtimes(destPath, s.Remote.UpdatedAt, s.Remote.UpdatedAt)
	if err != nil {
		return "", fmt.Errorf("failed to update file timestamp: %w", err)
	}

	logger.Debug("Downloaded save and set timestamp",
		"path", destPath,
		"remoteUpdatedAt", s.Remote.UpdatedAt)

	return destPath, nil
}

func (s *SaveSync) upload(host romm.Host, config *internal.Config) (string, error) {
	if s.Local == nil {
		return "", fmt.Errorf("cannot upload: no local save file")
	}
	if config == nil {
		return "", fmt.Errorf("config is nil")
	}

	rc := romm.NewClientFromHost(host, config.ApiTimeout)

	ext := normalizeExt(filepath.Ext(s.Local.Path))

	fileInfo, err := os.Stat(s.Local.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	modTime := fileInfo.ModTime()
	timestamp := modTime.Format("[2006-01-02 15-04-05-000]")

	filename := s.GameBase + " " + timestamp + ext
	tmp := filepath.Join(fileutil.TempDir(), "uploads", filename)

	err = fileutil.CopyFile(s.Local.Path, tmp)
	if err != nil {
		return "", err
	}

	// Get emulator from the save folder path
	emulator := filepath.Base(filepath.Dir(s.Local.Path))

	uploadedSave, err := rc.UploadSave(s.RomID, tmp, emulator)
	if err != nil {
		return "", err
	}

	err = os.Chtimes(s.Local.Path, uploadedSave.UpdatedAt, uploadedSave.UpdatedAt)
	if err != nil {
		return "", fmt.Errorf("failed to update file timestamp: %w", err)
	}

	return s.Local.Path, nil
}

// lookupRomID looks up a ROM ID by filename from the cache
func lookupRomID(romFile *LocalRomFile) (int, string) {
	logger := gaba.GetLogger()

	// Look up from the games cache
	if romID, romName, found := cache.GetCachedRomIDByFilename(romFile.FSSlug, romFile.FileName); found {
		logger.Debug("ROM lookup from cache", "fsSlug", romFile.FSSlug, "file", romFile.FileName, "romID", romID, "name", romName)
		return romID, romName
	}

	logger.Debug("No ROM found for file", "fsSlug", romFile.FSSlug, "file", romFile.FileName)
	return 0, ""
}

func lookupRomByHash(rc *romm.Client, romFile *LocalRomFile) (int, string) {
	logger := gaba.GetLogger()

	if romFile.FilePath == "" {
		return 0, ""
	}

	crcHash, err := fileutil.ComputeCRC32(romFile.FilePath)
	if err != nil {
		logger.Debug("Failed to compute CRC32 hash", "file", romFile.FileName, "error", err)
		return 0, ""
	}

	logger.Debug("Looking up ROM by CRC32 hash", "file", romFile.FileName, "crc", crcHash)

	rom, err := rc.GetRomByHash(romm.GetRomByHashQuery{CrcHash: crcHash})
	if err != nil {
		logger.Debug("ROM not found by hash", "file", romFile.FileName, "crc", crcHash, "error", err)
		return 0, ""
	}

	if rom.ID > 0 {
		logger.Info("Found ROM by CRC32 hash",
			"file", romFile.FileName,
			"crc", crcHash,
			"romID", rom.ID,
			"romName", rom.Name)
		return rom.ID, rom.Name
	}

	return 0, ""
}

func FindSaveSyncs(host romm.Host, config *internal.Config) ([]SaveSync, []UnmatchedSave, error) {
	return FindSaveSyncsFromScan(host, config, ScanRoms())
}

func FindSaveSyncsFromScan(host romm.Host, config *internal.Config, scanLocal LocalRomScan) ([]SaveSync, []UnmatchedSave, error) {
	logger := gaba.GetLogger()
	if config == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}
	rc := romm.NewClientFromHost(host, config.ApiTimeout)

	logger.Debug("FindSaveSyncs: Scanned local ROMs", "platformCount", len(scanLocal))

	// Get platforms from cache or API to build fsSlug -> platformID map
	cm := cache.GetCacheManager()
	var platforms []romm.Platform
	var err error

	if cm != nil {
		platforms, err = cm.GetPlatforms()
	}
	if err != nil || len(platforms) == 0 {
		// Fall back to API if cache miss
		platforms, err = rc.GetPlatforms()
		if err != nil {
			logger.Error("FindSaveSyncs: Could not retrieve platforms", "error", err)
			return []SaveSync{}, nil, err
		}
	}

	fsSlugToPlatformID := make(map[string]int)
	for _, p := range platforms {
		fsSlugToPlatformID[p.FSSlug] = p.ID
	}

	// Fetch saves per platform in parallel (saves are not cached - always fresh from API)
	type platformFetchResult struct {
		fsSlug   string
		saves    []romm.Save
		hasError bool
	}

	resultChan := make(chan platformFetchResult, len(scanLocal))
	var wg gosync.WaitGroup

	for fsSlug := range scanLocal {
		platformID, ok := fsSlugToPlatformID[fsSlug]
		if !ok {
			logger.Debug("FindSaveSyncs: No platform ID for fsSlug", "fsSlug", fsSlug)
			continue
		}

		wg.Add(1)
		go func(fsSlug string, platformID int) {
			defer wg.Done()

			result := platformFetchResult{
				fsSlug: fsSlug,
			}

			// Fetch saves for this platform (always from API - saves need to be fresh)
			platformSaves, err := rc.GetSaves(romm.SaveQuery{PlatformID: platformID})
			if err != nil {
				logger.Warn("FindSaveSyncs: Could not retrieve saves for platform", "fsSlug", fsSlug, "error", err)
				result.hasError = true
				resultChan <- result
				return
			}
			result.saves = platformSaves
			logger.Debug("FindSaveSyncs: Retrieved saves for platform", "fsSlug", fsSlug, "count", len(platformSaves))

			resultChan <- result
		}(fsSlug, platformID)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect saves by ROM ID
	savesByRomID := make(map[int][]romm.Save)
	for result := range resultChan {
		if result.hasError {
			continue
		}

		for _, s := range result.saves {
			savesByRomID[s.RomID] = append(savesByRomID[s.RomID], s)
		}
	}

	// Match local ROMs to cached ROMs by filename
	var unmatched []UnmatchedSave
	for fsSlug, localRoms := range scanLocal {
		for idx := range localRoms {
			romFile := &scanLocal[fsSlug][idx]

			// Skip if no save file and no remote saves exist
			if romFile.SaveFile == nil && len(savesByRomID) == 0 {
				continue
			}

			// Look up ROM ID from the games cache
			romID, romName := lookupRomID(romFile)

			if romID == 0 && romFile.SaveFile != nil {
				// Try to find ROM by CRC32 hash as fallback
				romID, romName = lookupRomByHash(rc, romFile)
			}

			if romID == 0 {
				if romFile.SaveFile != nil {
					unmatched = append(unmatched, UnmatchedSave{
						SavePath: romFile.SaveFile.Path,
						FSSlug:   fsSlug,
					})
					logger.Info("Save has local ROM but not in RomM",
						"save", filepath.Base(romFile.SaveFile.Path),
						"romFile", romFile.FileName,
						"fsSlug", fsSlug)
				}
				continue
			}

			romFile.RomID = romID
			romFile.RomName = romName

			if saves, ok := savesByRomID[romID]; ok {
				romFile.RemoteSaves = saves
				logger.Debug("Found remote saves for ROM", "romName", romName, "saveCount", len(saves))
			}
		}
	}

	// Build sync list from ROMs that need syncing
	// Use a map to deduplicate by save file path (multiple fs_slugs may share saves)
	syncMap := make(map[string]SaveSync) // key: save file path or romID for downloads
	for fsSlug, roms := range scanLocal {
		for _, r := range roms {
			if r.RomID > 0 {
				logger.Debug("Evaluating ROM for sync",
					"romName", r.RomName,
					"romID", r.RomID,
					"hasLocalSave", r.SaveFile != nil,
					"remoteSaveCount", len(r.RemoteSaves))
			}
			action := r.syncAction()
			if action == Upload || action == Download {
				baseName := strings.TrimSuffix(r.FileName, filepath.Ext(r.FileName))

				// Create unique key for deduplication
				var key string
				if r.SaveFile != nil {
					// For uploads, key by local save path to avoid duplicates
					key = r.SaveFile.Path
				} else {
					// For downloads, key by romID to avoid duplicate downloads
					key = fmt.Sprintf("download_%d", r.RomID)
				}

				// Skip if already added (happens when multiple fs_slugs share same save dir)
				if _, exists := syncMap[key]; exists {
					continue
				}

				syncMap[key] = SaveSync{
					RomID:    r.RomID,
					RomName:  r.RomName,
					FSSlug:   fsSlug,
					GameBase: baseName,
					Local:    r.SaveFile,
					Remote:   r.lastRemoteSave(),
					Action:   action,
				}
			}
		}
	}

	var syncs []SaveSync
	for _, s := range syncMap {
		syncs = append(syncs, s)
	}

	if len(unmatched) > 0 {
		logger.Info("Unmatched saves", "count", len(unmatched))
	}

	return syncs, unmatched, nil
}

// normalizeExt ensures the extension has a leading dot
func normalizeExt(ext string) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		return "." + ext
	}
	return ext
}
