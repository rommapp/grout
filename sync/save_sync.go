package sync

import (
	"fmt"
	"grout/cache"
	"grout/internal/fileutil"
	"grout/internal/stringutil"
	"grout/romm"
	"grout/utils"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveSync struct {
	RomID    int
	RomName  string
	Slug     string
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
	Slug     string
}

func (s *SaveSync) Execute(host romm.Host, config *utils.Config) SyncResult {
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

func (s *SaveSync) download(host romm.Host, config *utils.Config) (string, error) {
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
		destDir, err = ResolveSavePath(s.Slug, s.RomID, config)
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

func (s *SaveSync) upload(host romm.Host, config *utils.Config) (string, error) {
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

// lookupRomID looks up a ROM ID by filename, first checking cache then the provided ROM map
func lookupRomID(romFile *LocalRomFile, romsByFilename map[string]romm.Rom) (int, string) {
	logger := gaba.GetLogger()

	// Check cache first
	if romID, romName, found := cache.GetCachedRomIDByFilename(romFile.Slug, romFile.FileName); found {
		logger.Debug("ROM lookup from cache", "slug", romFile.Slug, "file", romFile.FileName, "romID", romID, "name", romName)
		return romID, romName
	}

	// Look up in the ROM map by filename (without extension)
	key := stringutil.StripExtension(romFile.FileName)
	if rom, found := romsByFilename[key]; found {
		// Cache the result for next time
		cache.StoreRomID(romFile.Slug, romFile.FileName, rom.ID, rom.Name)
		logger.Debug("ROM lookup from RomM", "slug", romFile.Slug, "file", romFile.FileName, "romID", rom.ID, "name", rom.Name)
		return rom.ID, rom.Name
	}

	logger.Debug("No ROM found for file", "slug", romFile.Slug, "file", romFile.FileName)
	return 0, ""
}

func FindSaveSyncs(host romm.Host, config *utils.Config) ([]SaveSync, []UnmatchedSave, error) {
	return FindSaveSyncsFromScan(host, config, ScanRoms())
}

func FindSaveSyncsFromScan(host romm.Host, config *utils.Config, scanLocal LocalRomScan) ([]SaveSync, []UnmatchedSave, error) {
	logger := gaba.GetLogger()
	if config == nil {
		return nil, nil, fmt.Errorf("config is nil")
	}
	rc := romm.NewClientFromHost(host, config.ApiTimeout)

	logger.Debug("FindSaveSyncs: Scanned local ROMs", "platformCount", len(scanLocal))

	// Get all platforms to build slug -> platformID map
	platforms, err := rc.GetPlatforms()
	if err != nil {
		logger.Error("FindSaveSyncs: Could not retrieve platforms", "error", err)
		return []SaveSync{}, nil, err
	}

	slugToPlatformID := make(map[string]int)
	for _, p := range platforms {
		slugToPlatformID[p.Slug] = p.ID
	}

	// Fetch saves and ROMs per platform in parallel
	type platformFetchResult struct {
		slug     string
		saves    []romm.Save
		roms     map[string]romm.Rom
		hasError bool
	}

	resultChan := make(chan platformFetchResult, len(scanLocal))
	var wg gosync.WaitGroup

	for slug := range scanLocal {
		platformID, ok := slugToPlatformID[slug]
		if !ok {
			logger.Debug("FindSaveSyncs: No platform ID for slug", "slug", slug)
			continue
		}

		wg.Add(1)
		go func(slug string, platformID int) {
			defer wg.Done()

			result := platformFetchResult{
				slug: slug,
				roms: make(map[string]romm.Rom),
			}

			// Fetch saves for this platform
			platformSaves, err := rc.GetSaves(romm.SaveQuery{PlatformID: platformID})
			if err != nil {
				logger.Warn("FindSaveSyncs: Could not retrieve saves for platform", "slug", slug, "error", err)
				result.hasError = true
				resultChan <- result
				return
			}
			result.saves = platformSaves
			logger.Debug("FindSaveSyncs: Retrieved saves for platform", "slug", slug, "count", len(platformSaves))

			// Fetch all ROMs for this platform to build filename map
			page := 1
			for {
				romsPage, err := rc.GetRoms(romm.GetRomsQuery{
					PlatformID: platformID,
					Page:       page,
					Limit:      100,
				})
				if err != nil {
					logger.Warn("FindSaveSyncs: Could not retrieve ROMs for platform", "slug", slug, "error", err)
					break
				}

				for _, rom := range romsPage.Items {
					key := stringutil.StripExtension(rom.FsNameNoExt)
					result.roms[key] = rom
				}

				if len(romsPage.Items) < 100 {
					break
				}
				page++
			}
			logger.Debug("FindSaveSyncs: Built ROM filename map", "slug", slug, "count", len(result.roms))

			resultChan <- result
		}(slug, platformID)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	savesByRomID := make(map[int][]romm.Save)
	romsByFilename := make(map[string]map[string]romm.Rom)

	for result := range resultChan {
		if result.hasError {
			continue
		}

		for _, s := range result.saves {
			savesByRomID[s.RomID] = append(savesByRomID[s.RomID], s)
		}

		romsByFilename[result.slug] = result.roms
	}

	// Match local ROMs to remote ROMs by filename
	var unmatched []UnmatchedSave
	for slug, localRoms := range scanLocal {
		platformRoms := romsByFilename[slug]
		if platformRoms == nil {
			platformRoms = make(map[string]romm.Rom)
		}

		for idx := range localRoms {
			romFile := &scanLocal[slug][idx]

			// Skip if no save file and no remote saves exist
			if romFile.SaveFile == nil && len(savesByRomID) == 0 {
				continue
			}

			romID, romName := lookupRomID(romFile, platformRoms)

			if romID == 0 {
				if romFile.SaveFile != nil {
					unmatched = append(unmatched, UnmatchedSave{
						SavePath: romFile.SaveFile.Path,
						Slug:     slug,
					})
					logger.Info("Save has local ROM but not in RomM",
						"save", filepath.Base(romFile.SaveFile.Path),
						"romFile", romFile.FileName,
						"slug", slug)
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
	// Use a map to deduplicate by save file path (multiple slugs may share saves)
	syncMap := make(map[string]SaveSync) // key: save file path or romID for downloads
	for slug, roms := range scanLocal {
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

				// Skip if already added (happens when multiple slugs share same save dir)
				if _, exists := syncMap[key]; exists {
					continue
				}

				syncMap[key] = SaveSync{
					RomID:    r.RomID,
					RomName:  r.RomName,
					Slug:     slug,
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
