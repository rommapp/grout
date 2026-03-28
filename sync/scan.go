package sync

import (
	"os"
	"path/filepath"
	"strings"
)

// RomLookupFunc looks up a ROM by platform slug and filename (without extension).
// Returns rom ID, rom name, and whether a match was found.
type RomLookupFunc func(fsSlug, nameNoExt string) (int, string, bool)

// scanSavesInDir scans a base save path for save files matching ROMs.
// This is the testable core of ScanSaves, free of global dependencies.
func scanSavesInDir(baseSavePath string, emulatorMap map[string][]string, lookupRom RomLookupFunc, resolveSlug func(string) string) []LocalSave {
	var saves []LocalSave

	for fsSlug, emulatorDirs := range emulatorMap {
		rommFSSlug := fsSlug
		if resolveSlug != nil {
			rommFSSlug = resolveSlug(fsSlug)
		}

		for _, emuDir := range emulatorDirs {
			saveDir := filepath.Join(baseSavePath, emuDir)

			if _, err := os.Stat(saveDir); os.IsNotExist(err) {
				continue
			}

			entries, err := os.ReadDir(saveDir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}

				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if !ValidSaveExtensions[ext] {
					continue
				}

				nameNoExt := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

				romID, romName, found := lookupRom(rommFSSlug, nameNoExt)
				if !found {
					continue
				}

				saves = append(saves, LocalSave{
					RomID:       romID,
					RomName:     romName,
					FSSlug:      rommFSSlug,
					FileName:    entry.Name(),
					FilePath:    filepath.Join(saveDir, entry.Name()),
					EmulatorDir: emuDir,
				})
			}
		}
	}

	return saves
}

// createBackup creates a timestamped backup of a save file before overwriting.
// Returns the backup path and any error. If the source file doesn't exist, returns ("", nil).
func createBackup(savePath, fileName string) (string, error) {
	info, err := os.Stat(savePath)
	if err != nil {
		return "", nil // file doesn't exist, nothing to back up
	}

	backupDir := filepath.Join(filepath.Dir(savePath), ".backup")
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	timestamp := info.ModTime().Format("2006-01-02 15-04-05")
	backupPath := filepath.Join(backupDir, base+" ["+timestamp+"]"+ext)

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	src, err := os.ReadFile(savePath)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(backupPath, src, 0644); err != nil {
		return "", err
	}

	return backupPath, nil
}
