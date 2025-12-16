package utils

import (
	"fmt"
	"grout/romm"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveSync struct {
	RomID            int
	RomName          string
	Slug             string
	GameBase         string
	Local            *localSave
	Remote           romm.Save
	Action           syncAction
	selectedEmulator string
}

func (s *SaveSync) NeedsEmulatorSelection() bool {
	return s.Local == nil && needsEmulatorSelection(s.Slug, false)
}

func (s *SaveSync) GetSlug() string {
	return s.Slug
}

func (s *SaveSync) GetGameBase() string {
	return s.GameBase
}

func (s *SaveSync) GetAction() syncAction {
	return s.Action
}

func (s *SaveSync) SetSelectedEmulator(emulator string) {
	gaba.GetLogger().Debug("SetSelectedEmulator called", "game", s.GameBase, "emulator", emulator, "previousValue", s.selectedEmulator)
	s.selectedEmulator = emulator
}

type syncAction string

const (
	Download syncAction = "DOWNLOAD"
	Upload              = "UPLOAD"
	Skip                = "SKIP"
)

type SyncResult struct {
	GameName       string
	RomDisplayName string
	Action         syncAction
	Success        bool
	Error          string
	FilePath       string
	UnmatchedSaves []UnmatchedSave
}

type UnmatchedSave struct {
	SavePath string
	Slug     string
}

func (s *SaveSync) Execute(host romm.Host) SyncResult {
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
		result.FilePath, err = s.upload(host)
		logger.Debug("Upload complete", "filePath", result.FilePath, "err", err)
	case Download:
		if s.Local != nil {
			err = s.Local.backup()
			if err != nil {
				result.Error = err.Error()
				return result
			}
		}
		result.FilePath, err = s.download(host)
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

func (s *SaveSync) download(host romm.Host) (string, error) {
	logger := gaba.GetLogger()
	rc := GetRommClient(host)

	logger.Debug("Downloading save", "saveID", s.Remote.ID, "downloadPath", s.Remote.DownloadPath)

	saveData, err := rc.DownloadSave(s.Remote.DownloadPath)
	if err != nil {
		return "", fmt.Errorf("failed to download save: %w", err)
	}

	var destDir string
	if s.Local != nil {
		destDir = filepath.Dir(s.Local.Path)
	} else {
		selectedEmulator := s.Remote.Emulator
		if s.selectedEmulator != "" {
			selectedEmulator = s.selectedEmulator
		}

		logger.Debug("Determining save directory for new save",
			"game", s.GameBase,
			"s.selectedEmulator", s.selectedEmulator,
			"s.Remote.Emulator", s.Remote.Emulator,
			"selectedEmulator", selectedEmulator)

		var err error
		destDir, err = getSaveDirectoryForSlug(s.Slug, selectedEmulator)
		if err != nil {
			return "", fmt.Errorf("cannot determine save location: %w", err)
		}
	}

	ext := s.Remote.FileExtension
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	filename := s.GameBase + ext
	destPath := filepath.Join(destDir, filename)

	if s.Local != nil && s.Local.Path != destPath {
		defer os.Remove(s.Local.Path)
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

func (s *SaveSync) upload(host romm.Host) (string, error) {
	if s.Local == nil {
		return "", fmt.Errorf("cannot upload: no local save file")
	}

	rc := GetRommClient(host)

	ext := filepath.Ext(s.Local.Path)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	fileInfo, err := os.Stat(s.Local.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	modTime := fileInfo.ModTime()
	timestamp := modTime.Format("[2006-01-02 15-04-05-000]")

	filename := s.GameBase + " " + timestamp + ext
	tmp := filepath.Join(TempDir(), "uploads", filename)

	err = copyFile(s.Local.Path, tmp)
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

func lookupRomID(romFile *localRomFile, rc *romm.Client) (int, string, error) {
	logger := gaba.GetLogger()

	logger.Debug("Looking up ROM by hash", "slug", romFile.Slug, "sha1", romFile.SHA1[:8])
	rom, err := rc.GetRomByHash(romm.GetRomByHashQuery{
		Sha1Hash: romFile.SHA1,
	})

	if err != nil {
		logger.Debug("No remote ROM found for hash", "sha1", romFile.SHA1[:8], "error", err)
		return 0, "", nil
	}

	logger.Debug("ROM lookup successful", "slug", romFile.Slug, "sha1", romFile.SHA1[:8], "romID", rom.ID, "name", rom.Name)
	return rom.ID, rom.Name, nil
}

func FindSaveSyncs(host romm.Host) ([]SaveSync, []UnmatchedSave, error) {
	logger := gaba.GetLogger()
	rc := GetRommClient(host)

	logger.Debug("FindSaveSyncs: Starting save sync discovery")

	// Scan all local ROMs
	scanLocal := scanRoms()
	logger.Debug("FindSaveSyncs: Scanned local ROMs", "platformCount", len(scanLocal))

	// Fetch all remote saves once
	allSaves, err := rc.GetSaves(romm.SaveQuery{})
	if err != nil {
		logger.Error("FindSaveSyncs: Could not retrieve saves", "error", err)
		return []SaveSync{}, nil, err
	}
	logger.Debug("FindSaveSyncs: Retrieved all saves", "count", len(*allSaves))

	// Group saves by RomID
	savesByRomID := make(map[int][]romm.Save)
	for _, s := range *allSaves {
		savesByRomID[s.RomID] = append(savesByRomID[s.RomID], s)
	}

	// Process each ROM and lookup ROM IDs
	var unmatched []UnmatchedSave
	for slug, localRoms := range scanLocal {
		logger.Debug("FindSaveSyncs: Processing platform", "slug", slug, "localRomCount", len(localRoms))

		for idx := range localRoms {
			// Lookup ROM ID from API
			romID, romName, err := lookupRomID(&scanLocal[slug][idx], rc)
			if err != nil {
				logger.Warn("Error looking up ROM ID", "rom", localRoms[idx].FileName, "error", err)
			}

			if romID == 0 {
				// ROM not in RomM - track as unmatched if it has a save
				if scanLocal[slug][idx].SaveFile != nil {
					unmatched = append(unmatched, UnmatchedSave{
						SavePath: scanLocal[slug][idx].SaveFile.Path,
						Slug:     slug,
					})
					logger.Info("Save has local ROM but not in RomM",
						"save", filepath.Base(scanLocal[slug][idx].SaveFile.Path),
						"romFile", scanLocal[slug][idx].FileName,
						"slug", slug)
				}
				continue
			}

			// Populate ROM metadata
			scanLocal[slug][idx].RomID = romID
			scanLocal[slug][idx].RomName = romName

			// Attach remote saves if they exist
			if saves, ok := savesByRomID[romID]; ok {
				scanLocal[slug][idx].RemoteSaves = saves
				logger.Debug("Found remote saves for ROM", "romName", romName, "saveCount", len(saves))
			}
		}
	}

	// Build sync list from ROMs that need syncing
	// Use a map to deduplicate by save file path (multiple slugs may share saves)
	syncMap := make(map[string]SaveSync) // key: save file path or romID for downloads
	for slug, roms := range scanLocal {
		for _, r := range roms {
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
					logger.Debug("Skipping duplicate sync",
						"rom", r.FileName,
						"slug", slug,
						"action", action,
						"key", key)
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
				logger.Debug("Sync action added",
					"rom", r.FileName,
					"romName", r.RomName,
					"romID", r.RomID,
					"slug", slug,
					"action", action)
			}
		}
	}

	// Convert map to slice
	var syncs []SaveSync
	for _, sync := range syncMap {
		syncs = append(syncs, sync)
	}

	// Log unmatched saves summary
	if len(unmatched) > 0 {
		logger.Info("Unmatched saves", "count", len(unmatched))
	}

	logger.Debug("FindSaveSyncs: Completed", "totalSyncs", len(syncs), "unmatchedSaves", len(unmatched))
	return syncs, unmatched, nil
}
