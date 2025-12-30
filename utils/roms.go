package utils

import (
	"grout/constants"
	"grout/romm"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type localRomFile struct {
	RomID        int
	RomName      string
	Slug         string
	Path         string
	FileName     string
	SHA1         string
	LastModified time.Time
	RemoteSaves  []romm.Save
	SaveFile     *localSave
}

func (lrf localRomFile) syncAction() SyncAction {
	if lrf.SaveFile == nil && len(lrf.RemoteSaves) == 0 {
		return Skip
	}
	if lrf.SaveFile != nil && len(lrf.RemoteSaves) == 0 {
		return Upload
	}
	if lrf.SaveFile == nil && len(lrf.RemoteSaves) > 0 {
		return Download
	}

	lastRemote := lrf.lastRemoteSave()

	// Truncate to second precision to avoid timestamp precision issues
	// API timestamps are typically second/millisecond precision, but filesystem is nanosecond
	localTime := lrf.SaveFile.LastModified.Truncate(time.Second)
	remoteTime := lastRemote.UpdatedAt.Truncate(time.Second)

	switch localTime.Compare(remoteTime) {
	case -1:
		return Download
	case 0:
		return Skip
	case 1:
		return Upload
	default:
		return Skip
	}
}

func (lrf localRomFile) lastRemoteSave() romm.Save {
	if len(lrf.RemoteSaves) == 0 {
		return romm.Save{}
	}

	slices.SortFunc(lrf.RemoteSaves, func(s1 romm.Save, s2 romm.Save) int {
		return s2.UpdatedAt.Compare(s1.UpdatedAt)
	})

	return lrf.RemoteSaves[0]
}

func scanRoms() map[string][]localRomFile {
	logger := gaba.GetLogger()
	result := make(map[string][]localRomFile)
	cfw := GetCFW()

	platformMap := GetPlatformMap(cfw)
	if platformMap == nil {
		logger.Warn("Unknown CFW, cannot scan ROMs")
		return result
	}

	baseRomDir := GetRomDirectory()
	logger.Debug("Starting ROM scan", "baseDir", baseRomDir)

	config, _ := LoadConfig()

	result = scanRomsByPlatform(baseRomDir, platformMap, config, cfw)

	totalRoms := 0
	for _, roms := range result {
		totalRoms += len(roms)
	}
	logger.Debug("Completed ROM scan", "platforms", len(result), "totalRoms", totalRoms)

	return result
}

func buildSaveFileMap(slug string) map[string]*localSave {
	saveFiles := findSaveFiles(slug)
	saveFileMap := make(map[string]*localSave)
	for i := range saveFiles {
		baseName := strings.TrimSuffix(filepath.Base(saveFiles[i].Path), filepath.Ext(saveFiles[i].Path))
		saveFileMap[baseName] = &saveFiles[i]
	}
	return saveFileMap
}

func scanRomsByPlatform(baseRomDir string, platformMap map[string][]string, config *Config, cfw constants.CFW) map[string][]localRomFile {
	logger := gaba.GetLogger()
	result := make(map[string][]localRomFile)

	if cfw == constants.NextUI {
		entries, err := os.ReadDir(baseRomDir)
		if err != nil {
			logger.Error("Failed to read ROM directory", "path", baseRomDir, "error", err)
			return result
		}

		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			dirName := entry.Name()
			tag := ParseTag(dirName)
			if tag == "" {
				logger.Debug("No tag found in directory", "dir", dirName)
				continue
			}

			for slug, cfwDirs := range platformMap {
				matched := false
				for _, cfwDir := range cfwDirs {
					cfwTag := ParseTag(cfwDir)
					if cfwTag == tag {
						matched = true
						break
					}
				}

				if !matched {
					if config != nil {
						if mapping, ok := config.DirectoryMappings[slug]; ok {
							if ParseTag(mapping.RelativePath) == tag {
								matched = true
							}
						}
					}
				}

				if matched {
					romDir := filepath.Join(baseRomDir, dirName)
					saveFileMap := buildSaveFileMap(slug)
					roms := scanRomDirectory(slug, romDir, saveFileMap)
					if len(roms) > 0 {
						result[slug] = append(result[slug], roms...)
						logger.Debug("Found ROMs for platform", "slug", slug, "dir", dirName, "count", len(roms))
					}
				}
			}
		}
	} else {
		// Parallelize platform scanning for MuOS and Knulli
		type platformResult struct {
			slug string
			roms []localRomFile
		}

		resultChan := make(chan platformResult, len(platformMap))
		var wg sync.WaitGroup

		for slug := range platformMap {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()

				romFolderName := ""
				if config != nil {
					if mapping, ok := config.DirectoryMappings[s]; ok && mapping.RelativePath != "" {
						romFolderName = mapping.RelativePath
					}
				}

				if romFolderName == "" {
					romFolderName = RomMSlugToCFW(s)
				}

				if romFolderName == "" {
					logger.Debug("No ROM folder mapping for slug", "slug", s)
					resultChan <- platformResult{slug: s, roms: nil}
					return
				}

				romDir := filepath.Join(baseRomDir, romFolderName)

				if _, err := os.Stat(romDir); os.IsNotExist(err) {
					resultChan <- platformResult{slug: s, roms: nil}
					return
				}

				saveFileMap := buildSaveFileMap(s)
				roms := scanRomDirectory(s, romDir, saveFileMap)
				resultChan <- platformResult{slug: s, roms: roms}
				if len(roms) > 0 {
					logger.Debug("Found ROMs for platform", "slug", s, "count", len(roms))
				}
			}(slug)
		}

		// Close channel once all goroutines complete
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results from all platforms
		for pr := range resultChan {
			if len(pr.roms) > 0 {
				result[pr.slug] = pr.roms
			}
		}
	}

	return result
}

func scanRomDirectory(slug, romDir string, saveFileMap map[string]*localSave) []localRomFile {
	logger := gaba.GetLogger()
	var roms []localRomFile

	entries, err := os.ReadDir(romDir)
	if err != nil {
		logger.Error("Failed to read ROM directory", "path", romDir, "error", err)
		return roms
	}

	visibleFiles := FilterVisibleFiles(entries)
	for _, entry := range visibleFiles {
		romPath := filepath.Join(romDir, entry.Name())

		fileInfo, err := entry.Info()
		if err != nil {
			logger.Warn("Failed to get file info", "file", entry.Name(), "error", err)
			continue
		}

		hash, err := calculateSHA1(romPath)
		if err != nil {
			logger.Warn("Failed to calculate SHA1 for ROM", "path", romPath, "error", err)
		}

		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		var saveFile *localSave
		if sf, found := saveFileMap[baseName]; found {
			saveFile = sf
		}

		rom := localRomFile{
			Slug:         slug,
			Path:         romPath,
			FileName:     entry.Name(),
			SHA1:         hash,
			LastModified: fileInfo.ModTime(),
			SaveFile:     saveFile,
		}

		roms = append(roms, rom)
	}

	return roms
}

func getRomDirectoriesForSlug(slug string) ([]string, error) {
	logger := gaba.GetLogger()
	cfw := GetCFW()
	baseRomDir := GetRomDirectory()
	config, _ := LoadConfig()

	var romDirs []string

	if config != nil {
		if mapping, ok := config.DirectoryMappings[slug]; ok && mapping.RelativePath != "" {
			romDir := filepath.Join(baseRomDir, mapping.RelativePath)
			romDirs = append(romDirs, romDir)
			return romDirs, nil
		}
	}

	if cfw == constants.NextUI {
		platformMap := GetPlatformMap(cfw)
		if cfwDirs, ok := platformMap[slug]; ok {
			entries, err := os.ReadDir(baseRomDir)
			if err != nil {
				return nil, err
			}

			for _, entry := range entries {
				if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}

				dirName := entry.Name()
				tag := ParseTag(dirName)
				if tag == "" {
					continue
				}

				for _, cfwDir := range cfwDirs {
					cfwTag := ParseTag(cfwDir)
					if cfwTag == tag {
						romDirs = append(romDirs, filepath.Join(baseRomDir, dirName))
						break
					}
				}
			}
		}
	} else {
		romFolderName := RomMSlugToCFW(slug)
		if romFolderName != "" {
			romDir := filepath.Join(baseRomDir, romFolderName)
			if _, err := os.Stat(romDir); err == nil {
				romDirs = append(romDirs, romDir)
			}
		}
	}

	if len(romDirs) == 0 {
		logger.Debug("No ROM directories found for platform", "slug", slug)
	}

	return romDirs, nil
}
