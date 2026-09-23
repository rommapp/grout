package cfw

import (
	"grout/files"
	"grout/settings"
	"grout/textmatch"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
)

type LocalRomFile struct {
	RomID    int
	RomName  string
	FSSlug   string
	FileName string
	FilePath string
}

type LocalRomScan map[string][]LocalRomFile

func ScanRoms(config settings.Config) LocalRomScan {
	logger := slog.Default()
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

func scanRomsByPlatform(baseRomDir string, platformMap map[string][]string, config settings.Config, currentCFW CFW) map[string][]LocalRomFile {
	logger := slog.Default()
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
			tag := textmatch.ParseTag(dirName)
			if tag == "" {
				logger.Debug("No tag found in directory", "dir", dirName)
				continue
			}

			for fsSlug, cfwDirs := range platformMap {
				matched := false
				for _, cfwDir := range cfwDirs {
					cfwTag := textmatch.ParseTag(cfwDir)
					if cfwTag == tag {
						matched = true
						break
					}
				}

				if !matched {
					rommFSSlug := config.ResolveRommFSSlug(fsSlug)
					if relPath, ok := config.GetDirectoryMapping(rommFSSlug); ok {
						matched = textmatch.ParseTag(relPath) == tag
					}
				}

				if matched {
					rommFSSlug := config.ResolveRommFSSlug(fsSlug)
					romDir := filepath.Join(baseRomDir, dirName)
					roms := scanRomDirectory(rommFSSlug, romDir)
					if len(roms) > 0 {
						result[rommFSSlug] = append(result[rommFSSlug], roms...)
						logger.Debug("Found ROMs for platform", "fsSlug", rommFSSlug, "dir", dirName, "count", len(roms))
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

				rommFSSlug := config.ResolveRommFSSlug(s)

				romFolderName := ""
				if relPath, ok := config.GetDirectoryMapping(rommFSSlug); ok && relPath != "" {
					romFolderName = relPath
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

				if !files.FileExists(romDir) {
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
	logger := slog.Default()
	var roms []LocalRomFile

	entries, err := os.ReadDir(romDir)
	if err != nil {
		logger.Error("Failed to read ROM directory", "path", romDir, "error", err)
		return roms
	}

	visibleFiles := files.FilterVisibleFiles(entries)
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
