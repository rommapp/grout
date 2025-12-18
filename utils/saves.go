package utils

import (
	"fmt"
	"grout/constants"
	"os"
	"path/filepath"
	"strings"
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

func getSaveDirectoryForSlug(slug string, emulator string) (string, error) {
	logger := gaba.GetLogger()
	logger.Debug("getSaveDirectoryForSlug called", "slug", slug, "emulator", emulator)
	bsd := getSaveDirectory()

	var saveFolders []string

	switch GetCFW() {
	case constants.MuOS:
		saveFolders = constants.MuOSSaveDirectories[slug]
	case constants.Knulli:
		saveFolders = constants.KnulliSaveDirectories[slug]
	case constants.NextUI:
		saveFolders = constants.NextUISaveDirectories[slug]
	}

	if len(saveFolders) == 0 {
		return "", fmt.Errorf("no save folder mapping for slug: %s", slug)
	}

	selectedFolder := saveFolders[0]
	logger.Debug("Initial selectedFolder (default)", "selectedFolder", selectedFolder, "allFolders", saveFolders)
	if emulator != "" {
		matched := false
		for _, folder := range saveFolders {
			if folder == emulator {
				selectedFolder = folder
				matched = true
				logger.Debug("Exact match for emulator folder", "emulator", emulator, "folder", folder)
				break
			}
		}

		if !matched {
			for _, folder := range saveFolders {
				if strings.Contains(strings.ToLower(folder), strings.ToLower(emulator)) {
					selectedFolder = folder
					logger.Debug("Matched emulator to save folder (substring)", "emulator", emulator, "folder", folder)
					break
				}
			}
		}
	}

	logger.Debug("Final selectedFolder", "selectedFolder", selectedFolder)
	saveDir := filepath.Join(bsd, selectedFolder)

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		logger.Error("Failed to create save directory", "path", saveDir, "error", err)
		return "", fmt.Errorf("failed to create save directory: %w", err)
	}

	return saveDir, nil
}

func findSaveFiles(slug string) []localSave {
	logger := gaba.GetLogger()

	bsd := getSaveDirectory()
	var saveFolders []string

	switch GetCFW() {
	case constants.MuOS:
		saveFolders = constants.MuOSSaveDirectories[slug]
	case constants.NextUI:
		saveFolders = constants.NextUISaveDirectories[slug]
	}

	if len(saveFolders) == 0 {
		logger.Debug("No save folder mapping for slug", "slug", slug)
		return []localSave{}
	}

	var allSaveFiles []localSave

	for _, saveFolder := range saveFolders {
		sd := filepath.Join(bsd, saveFolder)

		if _, err := os.Stat(sd); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(sd)
		if err != nil {
			logger.Error("Failed to read save directory", "path", sd, "error", err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
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

				allSaveFiles = append(allSaveFiles, saveFile)
			}
		}

		logger.Debug("Found save files in directory", "path", sd, "count", len(entries))
	}

	logger.Debug("Found total save files", "slug", slug, "count", len(allSaveFiles))
	return allSaveFiles
}

func GetEmulatorDirectoriesWithStatus(slug string) []EmulatorDirectoryInfo {
	logger := gaba.GetLogger()
	bsd := getSaveDirectory()

	var saveFolders []string

	switch GetCFW() {
	case constants.MuOS:
		saveFolders = constants.MuOSSaveDirectories[slug]
	case constants.NextUI:
		saveFolders = constants.NextUISaveDirectories[slug]
	}

	if len(saveFolders) == 0 {
		logger.Debug("No save folder mapping for slug", "slug", slug)
		return []EmulatorDirectoryInfo{}
	}

	dirInfos := make([]EmulatorDirectoryInfo, 0, len(saveFolders))

	for _, saveFolder := range saveFolders {
		fullPath := filepath.Join(bsd, saveFolder)
		info := EmulatorDirectoryInfo{
			DirectoryName: saveFolder,
			FullPath:      fullPath,
			HasSaves:      false,
			SaveCount:     0,
		}

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			dirInfos = append(dirInfos, info)
			continue
		}

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			logger.Warn("Failed to read directory", "path", fullPath, "error", err)
			dirInfos = append(dirInfos, info)
			continue
		}

		count := 0
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				count++
			}
		}

		info.SaveCount = count
		info.HasSaves = count > 0

		dirInfos = append(dirInfos, info)
	}

	return dirInfos
}

func needsEmulatorSelection(slug string, hasLocalSave bool) bool {
	if hasLocalSave {
		return false
	}

	dirInfos := GetEmulatorDirectoriesWithStatus(slug)

	if len(dirInfos) <= 1 {
		return false
	}

	nonEmptyCount := 0
	for _, info := range dirInfos {
		if info.HasSaves {
			nonEmptyCount++
		}
	}

	if nonEmptyCount > 1 {
		return true
	}

	if nonEmptyCount == 0 {
		return true
	}

	return false
}
