package sync

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/internal/stringutil"
	"grout/romm"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

var ErrOrphanRom = errors.New("orphan ROM")

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
	Err            error
	FilePath       string
	UnmatchedSaves []UnmatchedSave
}

type UnmatchedSave struct {
	SavePath          string
	FSSlug            string
	RomFileName       string
	RomFilePath       string
	CRC32Hash         string
	SHA1Hash          string
	BestFuzzyMatch    string
	BestFuzzySim      float64
	CooldownActive    bool
	CooldownExpiresAt time.Time
	MatchesAttempted  []string
}

type PendingFuzzyMatch struct {
	LocalFilename string
	LocalPath     string
	SavePath      string
	FSSlug        string
	MatchedRomID  int
	MatchedName   string
	Similarity    float64
}

// MatchAttemptResult tracks diagnostic info from ROM matching attempts
type MatchAttemptResult struct {
	CRC32Hash         string
	SHA1Hash          string
	CooldownActive    bool
	CooldownExpiresAt time.Time
	BestFuzzyMatch    string
	BestFuzzySim      float64
	MatchesAttempted  []string
}

func (s *SaveSync) Execute(host romm.Host, config *internal.Config) SyncResult {
	logger := gaba.GetLogger()

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
		result.Err = err
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
	if s.RomID == 0 && s.Local == nil {
		return "", ErrOrphanRom
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
	if s.RomID == 0 {
		return "", ErrOrphanRom
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

func lookupRomID(romFile *LocalRomFile) (int, string) {
	// Look up from the games cache
	if romID, romName, found := cache.GetCachedRomIDByFilename(romFile.FSSlug, romFile.FileName); found {
		return romID, romName
	}

	return 0, ""
}

func lookupRomByHash(rc *romm.Client, romFile *LocalRomFile, matchResult *MatchAttemptResult) (int, string) {
	logger := gaba.GetLogger()

	if romFile.FilePath == "" {
		return 0, ""
	}

	shouldAttempt, nextRetry := cache.ShouldAttemptLookupWithNextRetry(romFile.FSSlug, romFile.FileName)
	if !shouldAttempt {
		if matchResult != nil {
			matchResult.CooldownActive = true
			matchResult.CooldownExpiresAt = nextRetry
		}
		return 0, ""
	}

	if matchResult != nil {
		matchResult.MatchesAttempted = append(matchResult.MatchesAttempted, "hash")
	}

	crcHash, err := fileutil.ComputeCRC32(romFile.FilePath)
	if err != nil {
		logger.Debug("Failed to compute CRC32 hash", "file", romFile.FileName, "error", err)
		return 0, ""
	}

	if matchResult != nil {
		matchResult.CRC32Hash = crcHash
	}

	rom, err := rc.GetRomByHash(romm.GetRomByHashQuery{CrcHash: crcHash})
	if err == nil && rom.ID > 0 {
		logger.Info("Found ROM by CRC32 hash",
			"file", romFile.FileName,
			"crc", crcHash,
			"romID", rom.ID,
			"romName", rom.Name)
		_ = cache.SaveFilenameMapping(romFile.FSSlug, romFile.FileName, rom.ID, rom.Name)
		_ = cache.ClearFailedLookup(romFile.FSSlug, romFile.FileName)
		return rom.ID, rom.Name
	}

	sha1Hash, err := fileutil.ComputeSHA1(romFile.FilePath)
	if err != nil {
		logger.Debug("Failed to compute SHA1 hash", "file", romFile.FileName, "error", err)
		_ = cache.RecordFailedLookup(romFile.FSSlug, romFile.FileName)
		return 0, ""
	}

	if matchResult != nil {
		matchResult.SHA1Hash = sha1Hash
	}

	rom, err = rc.GetRomByHash(romm.GetRomByHashQuery{Sha1Hash: sha1Hash})
	if err == nil && rom.ID > 0 {
		logger.Info("Found ROM by SHA1 hash",
			"file", romFile.FileName,
			"sha1", sha1Hash,
			"romID", rom.ID,
			"romName", rom.Name)
		_ = cache.SaveFilenameMapping(romFile.FSSlug, romFile.FileName, rom.ID, rom.Name)
		_ = cache.ClearFailedLookup(romFile.FSSlug, romFile.FileName)
		return rom.ID, rom.Name
	}

	// Both lookups failed - don't record yet, let fuzzy matching try first
	return 0, ""
}

const FuzzyMatchThreshold = 0.80

func lookupRomByFuzzyTitle(romFile *LocalRomFile, matchResult *MatchAttemptResult) *PendingFuzzyMatch {
	logger := gaba.GetLogger()

	if romFile.FSSlug == "" || romFile.FileName == "" {
		return nil
	}

	if matchResult != nil {
		matchResult.MatchesAttempted = append(matchResult.MatchesAttempted, "fuzzy")
	}

	games, err := cache.GetGamesForPlatform(romFile.FSSlug)
	if err != nil || len(games) == 0 {
		return nil
	}

	localNormalized := stringutil.NormalizeForComparison(romFile.FileName)
	if localNormalized == "" {
		return nil
	}

	var bestMatch *PendingFuzzyMatch
	var bestSimilarity float64
	var bestBelowThresholdName string
	var bestBelowThresholdSim float64

	for _, game := range games {
		remoteNormalized := stringutil.NormalizeForComparison(game.Name)
		if remoteNormalized == "" {
			continue
		}

		similarity := stringutil.BestSimilarity(localNormalized, remoteNormalized)

		// Track the best match regardless of threshold for diagnostics
		if similarity > bestBelowThresholdSim {
			bestBelowThresholdSim = similarity
			bestBelowThresholdName = game.Name
		}

		if similarity >= FuzzyMatchThreshold && similarity > bestSimilarity {
			bestSimilarity = similarity
			bestMatch = &PendingFuzzyMatch{
				LocalFilename: stringutil.StripExtension(romFile.FileName),
				LocalPath:     romFile.FilePath,
				FSSlug:        romFile.FSSlug,
				MatchedRomID:  game.ID,
				MatchedName:   game.Name,
				Similarity:    similarity,
			}
		}
	}

	// Store best match info for diagnostics even if below threshold
	if matchResult != nil && bestBelowThresholdName != "" {
		matchResult.BestFuzzyMatch = bestBelowThresholdName
		matchResult.BestFuzzySim = bestBelowThresholdSim
	}

	if bestMatch != nil {
		logger.Info("Fuzzy match found",
			"local", romFile.FileName,
			"matched", bestMatch.MatchedName,
			"similarity", fmt.Sprintf("%.0f%%", bestMatch.Similarity*100))
	}

	return bestMatch
}

func FindSaveSyncs(host romm.Host, config *internal.Config) ([]SaveSync, []UnmatchedSave, []PendingFuzzyMatch, error) {
	return FindSaveSyncsFromScan(host, config, ScanRoms(config))
}

func FindSaveSyncsFromScan(host romm.Host, config *internal.Config, scanLocal LocalRomScan) ([]SaveSync, []UnmatchedSave, []PendingFuzzyMatch, error) {
	logger := gaba.GetLogger()
	if config == nil {
		return nil, nil, nil, fmt.Errorf("config is nil")
	}
	rc := romm.NewClientFromHost(host, config.ApiTimeout)

	logger.Debug("FindSaveSyncs: Scanned local ROMs", "platformCount", len(scanLocal))

	cm := cache.GetCacheManager()
	var platforms []romm.Platform
	var err error

	if cm != nil {
		platforms, err = cm.GetPlatforms()
	}
	if err != nil || len(platforms) == 0 {
		platforms, err = rc.GetPlatforms()
		if err != nil {
			logger.Error("FindSaveSyncs: Could not retrieve platforms", "error", err)
			return []SaveSync{}, nil, nil, err
		}
	}

	fsSlugToPlatformID := make(map[string]int)
	for _, p := range platforms {
		fsSlugToPlatformID[p.FSSlug] = p.ID
	}

	type platformFetchResult struct {
		fsSlug   string
		saves    []romm.Save
		hasError bool
	}

	resultChan := make(chan platformFetchResult, len(scanLocal))
	var wg gosync.WaitGroup

	for fsSlug := range scanLocal {
		var platformID int
		for _, alias := range cfw.GetPlatformAliases(fsSlug) {
			if id, ok := fsSlugToPlatformID[alias]; ok {
				platformID = id
				break
			}
		}
		if platformID == 0 {
			logger.Debug("FindSaveSyncs: No platform ID for fsSlug or aliases", "fsSlug", fsSlug)
			continue
		}

		wg.Add(1)
		go func(fsSlug string, platformID int) {
			defer wg.Done()

			result := platformFetchResult{
				fsSlug: fsSlug,
			}

			platformSaves, err := rc.GetSaves(romm.SaveQuery{PlatformID: platformID})
			if err != nil {
				logger.Warn("FindSaveSyncs: Could not retrieve saves for platform", "fsSlug", fsSlug, "error", err)
				result.hasError = true
				resultChan <- result
				return
			}
			result.saves = platformSaves
			logger.Debug("FindSaveSyncs: Retrieved remote saves for platform", "fsSlug", fsSlug, "count", len(platformSaves))

			resultChan <- result
		}(fsSlug, platformID)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	savesByRomID := make(map[int][]romm.Save)
	for result := range resultChan {
		if result.hasError {
			continue
		}

		for _, s := range result.saves {
			savesByRomID[s.RomID] = append(savesByRomID[s.RomID], s)
		}
	}

	var unmatched []UnmatchedSave
	var pendingFuzzy []PendingFuzzyMatch
	for fsSlug, localRoms := range scanLocal {
		for idx := range localRoms {
			romFile := &scanLocal[fsSlug][idx]

			if romFile.SaveFile == nil && len(savesByRomID) == 0 {
				continue
			}

			// Track match attempts for diagnostics
			matchResult := &MatchAttemptResult{}

			romID, romName := lookupRomID(romFile)
			if romID > 0 {
				matchResult.MatchesAttempted = append(matchResult.MatchesAttempted, "filename")
			}

			if romID == 0 && romFile.SaveFile != nil {
				matchResult.MatchesAttempted = append(matchResult.MatchesAttempted, "filename")
				romID, romName = lookupRomByHash(rc, romFile, matchResult)
			}

			if romID == 0 && romFile.SaveFile != nil {
				fuzzyMatch := lookupRomByFuzzyTitle(romFile, matchResult)
				if fuzzyMatch != nil {
					fuzzyMatch.SavePath = romFile.SaveFile.Path
					pendingFuzzy = append(pendingFuzzy, *fuzzyMatch)
					romFile.PendingFuzzyMatch = true // Mark to skip in sync building
					logger.Info("Fuzzy match candidate found",
						"local", romFile.FileName,
						"matched", fuzzyMatch.MatchedName,
						"similarity", fmt.Sprintf("%.0f%%", fuzzyMatch.Similarity*100))
				} else {
					if err := cache.RecordFailedLookup(romFile.FSSlug, romFile.FileName); err != nil {
						logger.Warn("Failed to record failed lookup",
							"file", romFile.FileName,
							"fsSlug", romFile.FSSlug,
							"error", err)
					}

					unmatchedSave := UnmatchedSave{
						SavePath:          romFile.SaveFile.Path,
						FSSlug:            fsSlug,
						RomFileName:       romFile.FileName,
						RomFilePath:       romFile.FilePath,
						CRC32Hash:         matchResult.CRC32Hash,
						SHA1Hash:          matchResult.SHA1Hash,
						BestFuzzyMatch:    matchResult.BestFuzzyMatch,
						BestFuzzySim:      matchResult.BestFuzzySim,
						CooldownActive:    matchResult.CooldownActive,
						CooldownExpiresAt: matchResult.CooldownExpiresAt,
						MatchesAttempted:  matchResult.MatchesAttempted,
					}
					unmatched = append(unmatched, unmatchedSave)

					// Enhanced logging with full diagnostic info
					logFields := []any{
						"savePath", romFile.SaveFile.Path,
						"romFile", romFile.FileName,
						"romPath", romFile.FilePath,
						"fsSlug", fsSlug,
						"matchesAttempted", strings.Join(matchResult.MatchesAttempted, ", "),
					}

					if matchResult.CooldownActive {
						timeUntil := time.Until(matchResult.CooldownExpiresAt).Round(time.Minute)
						logFields = append(logFields,
							"hashLookupSkipped", "cooldown active",
							"hashLookupRetriesIn", timeUntil.String(),
							"hashLookupRetriesAt", matchResult.CooldownExpiresAt.Format(time.RFC3339))
					}
					if matchResult.CRC32Hash != "" {
						logFields = append(logFields, "crc32", matchResult.CRC32Hash)
					}
					if matchResult.SHA1Hash != "" {
						logFields = append(logFields, "sha1", matchResult.SHA1Hash)
					}
					if matchResult.BestFuzzyMatch != "" {
						logFields = append(logFields,
							"bestFuzzyCandidate", matchResult.BestFuzzyMatch,
							"bestFuzzySimilarity", fmt.Sprintf("%.0f%%", matchResult.BestFuzzySim*100))
					}

					logger.Info("Unmatched save: ROM not found in RomM", logFields...)
				}
				continue
			}

			if romID == 0 {
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

	syncMap := make(map[string]SaveSync)
	for fsSlug, roms := range scanLocal {
		for _, r := range roms {
			if r.PendingFuzzyMatch {
				continue
			}
			// Skip unmatched ROMs - they're already in the unmatched list
			if r.RomID == 0 {
				continue
			}
			logger.Debug("Evaluating ROM for sync",
				"romName", r.RomName,
				"romID", r.RomID,
				"hasLocalSave", r.SaveFile != nil,
				"remoteSaveCount", len(r.RemoteSaves))
			action := r.syncAction()
			if action == Upload || action == Download {
				baseName := strings.TrimSuffix(r.FileName, filepath.Ext(r.FileName))

				var key string
				if r.SaveFile != nil {
					key = r.SaveFile.Path
				} else {
					key = fmt.Sprintf("download_%d_%s", r.RomID, baseName)
				}

				if _, exists := syncMap[key]; exists {
					continue
				}

				syncMap[key] = SaveSync{
					RomID:    r.RomID,
					RomName:  r.RomName,
					FSSlug:   fsSlug,
					GameBase: baseName,
					Local:    r.SaveFile,
					Remote:   r.lastRemoteSaveForBaseName(baseName),
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
		// Count saves with active cooldown
		cooldownCount := 0
		var earliestRetry time.Time
		for _, u := range unmatched {
			if u.CooldownActive {
				cooldownCount++
				if earliestRetry.IsZero() || u.CooldownExpiresAt.Before(earliestRetry) {
					earliestRetry = u.CooldownExpiresAt
				}
			}
		}

		summaryFields := []any{
			"count", len(unmatched),
			"hint", "Check logs above for detailed match attempt info per save",
		}

		if cooldownCount > 0 {
			summaryFields = append(summaryFields,
				"hashLookupSkipped", cooldownCount,
				"earliestHashRetry", earliestRetry.Format(time.RFC3339))
		}

		logger.Info("Unmatched saves summary", summaryFields...)
	}

	if len(pendingFuzzy) > 0 {
		logger.Info("Pending fuzzy matches", "count", len(pendingFuzzy))
	}

	return syncs, unmatched, pendingFuzzy, nil
}

func normalizeExt(ext string) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		return "." + ext
	}
	return ext
}
