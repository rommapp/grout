package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

const backupTimestampFormat = "2006-01-02 15-04-05"

type localSave struct {
	Slug         string
	Path         string
	LastModified time.Time
}

type EmulatorDirectoryInfo struct {
	DirectoryName string
	FullPath      string
	HasSaves      bool
	SaveCount     int
}

func (lc localSave) timestampedFilename() string {
	ext := filepath.Ext(lc.Path)
	base := strings.ReplaceAll(filepath.Base(lc.Path), ext, "")

	lm := lc.LastModified.Format(backupTimestampFormat)

	return fmt.Sprintf("%s [%s]%s", base, lm, ext)
}

func (lc localSave) backup() error {
	dest := filepath.Join(filepath.Dir(lc.Path), ".backup", lc.timestampedFilename())
	return copyFile(lc.Path, dest)
}

func ResolveSavePath(slug string, gameID int, config *Config) (string, error) {
	logger := gaba.GetLogger()
	logger.Debug("ResolveSavePath called", "slug", slug, "gameID", gameID)
	basePath := BaseSavePath()

	emulatorFolders := EmulatorFoldersForSlug(slug)

	if len(emulatorFolders) == 0 {
		return "", fmt.Errorf("no save folder mapping for slug: %s", slug)
	}

	selectedFolder := emulatorFolders[0]
	logger.Debug("Initial selectedFolder (default)", "selectedFolder", selectedFolder, "allFolders", emulatorFolders)

	// Priority 1: Check per-game override
	if config != nil && gameID > 0 && config.GameSaveOverrides != nil {
		if override, ok := config.GameSaveOverrides[gameID]; ok && override != "" {
			// Verify the override is a valid folder for this slug
			for _, folder := range emulatorFolders {
				if folder == override {
					selectedFolder = override
					logger.Debug("Using per-game override", "gameID", gameID, "folder", override)
					goto createDir
				}
			}
			logger.Warn("Per-game override not valid for slug, ignoring", "gameID", gameID, "override", override, "slug", slug)
		}
	}

	// Priority 2: Check platform-level mapping from config
	if config != nil && config.SaveDirectoryMappings != nil {
		if mapping, ok := config.SaveDirectoryMappings[slug]; ok && mapping != "" {
			for _, folder := range emulatorFolders {
				if folder == mapping {
					selectedFolder = mapping
					logger.Debug("Using platform mapping from config", "slug", slug, "folder", mapping)
					goto createDir
				}
			}
			logger.Warn("Platform mapping not valid for slug, ignoring", "mapping", mapping, "slug", slug)
		}
	}

createDir:
	logger.Debug("Final selectedFolder", "selectedFolder", selectedFolder)
	saveDir := filepath.Join(basePath, selectedFolder)

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		logger.Error("Failed to create save directory", "path", saveDir, "error", err)
		return "", fmt.Errorf("failed to create save directory: %w", err)
	}

	return saveDir, nil
}

func findSaveFiles(slug string) []localSave {
	logger := gaba.GetLogger()

	basePath := BaseSavePath()
	emulatorFolders := EmulatorFoldersForSlug(slug)

	if len(emulatorFolders) == 0 {
		logger.Debug("No save folder mapping for slug", "slug", slug)
		return []localSave{}
	}

	// Use channels and goroutines to scan directories in parallel
	type scanResult struct {
		saves []localSave
		path  string
		count int
	}

	resultChan := make(chan scanResult, len(emulatorFolders))
	var wg sync.WaitGroup

	for _, folder := range emulatorFolders {
		wg.Add(1)
		go func(folder string) {
			defer wg.Done()

			sd := filepath.Join(basePath, folder)
			result := scanResult{path: sd, saves: []localSave{}}

			if _, err := os.Stat(sd); os.IsNotExist(err) {
				resultChan <- result
				return
			}

			entries, err := os.ReadDir(sd)
			if err != nil {
				logger.Error("Failed to read save directory", "path", sd, "error", err)
				resultChan <- result
				return
			}

			visibleFiles := FilterVisibleFiles(entries)
			result.count = len(entries)
			result.saves = make([]localSave, 0, len(visibleFiles))

			for _, entry := range visibleFiles {
				savePath := filepath.Join(sd, entry.Name())

				fileInfo, err := entry.Info()
				if err != nil {
					logger.Warn("Failed to get file info", "file", entry.Name(), "error", err)
					continue
				}

				saveFile := localSave{
					Slug:         slug,
					Path:         savePath,
					LastModified: fileInfo.ModTime(),
				}

				result.saves = append(result.saves, saveFile)
			}

			resultChan <- result
		}(folder)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allSaveFiles []localSave
	for result := range resultChan {
		allSaveFiles = append(allSaveFiles, result.saves...)
		if result.count > 0 {
			logger.Debug("Found save files in directory", "path", result.path, "count", result.count)
		}
	}

	return allSaveFiles
}
