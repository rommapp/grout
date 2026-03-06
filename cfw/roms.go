package cfw

import (
	"grout/internal/fileutil"
	"grout/internal/stringutil"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// RomScanConfig provides configuration needed for ROM scanning.
// Implemented by internal.Config to avoid circular imports.
type RomScanConfig interface {
	GetDirectoryMapping(fsSlug string) (relativePath string, ok bool)
	ResolveRommFSSlug(cfwKey string) string
}

type LocalRomFile struct {
	RomID    int
	RomName  string
	FSSlug   string
	FileName string
	FilePath string
}

type LocalRomScan map[string][]LocalRomFile

func ScanRoms(config RomScanConfig) LocalRomScan {
	logger := gaba.GetLogger()
	result := make(map[string][]LocalRomFile)
	currentCFW := GetCFW()

	platformMap := GetPlatformMap(currentCFW)
	if platformMap == nil {
		logger.Warn("Unknown CFW, cannot scan ROMs")
		return result
	}

	baseRomDir := GetRomDirectory()
	logger.Debug("Starting ROM scan", "baseDir", baseRomDir)

	result = scanRomsByPlatform(baseRomDir, platformMap, config, currentCFW)

	totalRoms := 0
	for _, roms := range result {
		totalRoms += len(roms)
	}
	logger.Debug("Completed ROM scan", "platforms", len(result), "totalRoms", totalRoms)

	return result
}

func scanRomsByPlatform(baseRomDir string, platformMap map[string][]string, config RomScanConfig, currentCFW CFW) map[string][]LocalRomFile {
	logger := gaba.GetLogger()
	result := make(map[string][]LocalRomFile)

	if currentCFW == NextUI {
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
			tag := stringutil.ParseTag(dirName)
			if tag == "" {
				logger.Debug("No tag found in directory", "dir", dirName)
				continue
			}

			for fsSlug, cfwDirs := range platformMap {
				matched := false
				for _, cfwDir := range cfwDirs {
					cfwTag := stringutil.ParseTag(cfwDir)
					if cfwTag == tag {
						matched = true
						break
					}
				}

				if !matched {
					if config != nil {
						if relPath, ok := config.GetDirectoryMapping(fsSlug); ok {
							if stringutil.ParseTag(relPath) == tag {
								matched = true
							}
						}
					}
				}

				if matched {
					romDir := filepath.Join(baseRomDir, dirName)
					roms := scanRomDirectory(fsSlug, romDir)
					if len(roms) > 0 {
						result[fsSlug] = append(result[fsSlug], roms...)
						logger.Debug("Found ROMs for platform", "fsSlug", fsSlug, "dir", dirName, "count", len(roms))
					}
				}
			}
		}
	} else {
		type platformResult struct {
			fsSlug string
			roms   []LocalRomFile
		}

		resultChan := make(chan platformResult, len(platformMap))
		var wg gosync.WaitGroup

		for fsSlug := range platformMap {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()

				rommFSSlug := s
				if config != nil {
					rommFSSlug = config.ResolveRommFSSlug(s)
				}

				romFolderName := ""
				if config != nil {
					if relPath, ok := config.GetDirectoryMapping(rommFSSlug); ok && relPath != "" {
						romFolderName = relPath
					}
				}

				if romFolderName == "" {
					romFolderName = RomMFSSlugToCFW(s)
				}

				if romFolderName == "" {
					logger.Debug("No ROM folder mapping for fsSlug", "fsSlug", rommFSSlug)
					resultChan <- platformResult{fsSlug: rommFSSlug, roms: nil}
					return
				}

				romDir := filepath.Join(baseRomDir, romFolderName)

				if !fileutil.FileExists(romDir) {
					resultChan <- platformResult{fsSlug: rommFSSlug, roms: nil}
					return
				}

				roms := scanRomDirectory(rommFSSlug, romDir)
				resultChan <- platformResult{fsSlug: rommFSSlug, roms: roms}
				if len(roms) > 0 {
					logger.Debug("Found ROMs for platform", "fsSlug", rommFSSlug, "count", len(roms))
				}
			}(fsSlug)
		}

		go func() {
			wg.Wait()
			close(resultChan)
		}()

		for pr := range resultChan {
			if len(pr.roms) > 0 {
				result[pr.fsSlug] = pr.roms
			}
		}
	}

	return result
}

func scanRomDirectory(fsSlug, romDir string) []LocalRomFile {
	logger := gaba.GetLogger()
	var roms []LocalRomFile

	entries, err := os.ReadDir(romDir)
	if err != nil {
		logger.Error("Failed to read ROM directory", "path", romDir, "error", err)
		return roms
	}

	visibleFiles := fileutil.FilterVisibleFiles(entries)
	for _, entry := range visibleFiles {
		rom := LocalRomFile{
			FSSlug:   fsSlug,
			FileName: entry.Name(),
			FilePath: filepath.Join(romDir, entry.Name()),
		}

		roms = append(roms, rom)
	}

	return roms
}
