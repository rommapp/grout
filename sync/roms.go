package sync

import (
	"grout/cache"
	"grout/cfw"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// ResolveLocalRoms scans local ROM files and resolves them against the cache
// to get ROM IDs. Returns a map of ROM ID to LocalRomFile for matched ROMs.
func ResolveLocalRoms(scan cfw.LocalRomScan) map[int]cfw.LocalRomFile {
	logger := gaba.GetLogger()
	cm := cache.GetCacheManager()
	if cm == nil {
		logger.Error("Cache manager not available for ROM resolution")
		return nil
	}

	resolved := make(map[int]cfw.LocalRomFile)
	for fsSlug, files := range scan {
		for _, f := range files {
			nameNoExt := strings.TrimSuffix(f.FileName, filepath.Ext(f.FileName))
			rom, err := cm.GetRomByFSLookup(fsSlug, nameNoExt)
			if err != nil {
				continue
			}
			f.RomID = rom.ID
			f.RomName = rom.Name
			resolved[rom.ID] = f
		}
	}

	logger.Debug("Resolved local ROMs against cache", "matched", len(resolved))
	return resolved
}
