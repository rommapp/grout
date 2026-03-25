package sync

import (
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func ScanSaves(config *internal.Config) []LocalSave {
	logger := gaba.GetLogger()
	currentCFW := cfw.GetCFW()

	baseSavePath := cfw.BaseSavePath()
	if baseSavePath == "" {
		logger.Error("No save path for current CFW")
		return nil
	}

	emulatorMap := cfw.EmulatorFolderMap(currentCFW)
	if emulatorMap == nil {
		logger.Error("No emulator folder map for current CFW")
		return nil
	}

	cm := cache.GetCacheManager()
	if cm == nil {
		logger.Error("Cache manager not available for save scan")
		return nil
	}

	var saves []LocalSave

	logger.Debug("Starting save scan", "baseSavePath", baseSavePath, "platformCount", len(emulatorMap))

	for fsSlug, emulatorDirs := range emulatorMap {
		rommFSSlug := fsSlug
		if config != nil {
			rommFSSlug = config.ResolveRommFSSlug(fsSlug)
		}

		for _, emuDir := range emulatorDirs {
			saveDir := filepath.Join(baseSavePath, emuDir)

			if _, err := os.Stat(saveDir); os.IsNotExist(err) {
				continue
			}

			entries, err := os.ReadDir(saveDir)
			if err != nil {
				logger.Error("Could not read save directory", "path", saveDir, "error", err)
				continue
			}

			saveFileCount := 0
			for _, entry := range entries {
				if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}

				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if !ValidSaveExtensions[ext] {
					continue
				}

				saveFileCount++
				nameNoExt := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

				rom, err := cm.GetRomByFSLookup(rommFSSlug, nameNoExt)
				if err != nil {
					logger.Debug("No cache match for save file", "file", entry.Name(), "fsSlug", rommFSSlug, "nameNoExt", nameNoExt)
					continue
				}

				logger.Debug("Matched save to ROM", "file", entry.Name(), "romID", rom.ID, "romName", rom.Name)

				saves = append(saves, LocalSave{
					RomID:       rom.ID,
					RomName:     rom.Name,
					FSSlug:      rommFSSlug,
					FileName:    entry.Name(),
					FilePath:    filepath.Join(saveDir, entry.Name()),
					EmulatorDir: emuDir,
				})
			}

			if saveFileCount > 0 {
				logger.Debug("Scanned emulator directory", "path", saveDir, "saveFiles", saveFileCount)
			}
		}
	}

	logger.Debug("Completed save scan", "matched", len(saves))
	return saves
}

// SelectSaveForSlot picks the latest save from the given slot.
// Falls back to the most recently updated save if the slot has no saves.
func SelectSaveForSlot(saves []romm.Save, preferredSlot string) *romm.Save {
	if len(saves) == 0 {
		return nil
	}

	var best *romm.Save
	for i, s := range saves {
		slotName := "default"
		if s.Slot != nil {
			slotName = *s.Slot
		}
		if slotName != preferredSlot {
			continue
		}
		if best == nil || s.UpdatedAt.After(best.UpdatedAt) {
			best = &saves[i]
		}
	}
	if best != nil {
		return best
	}

	best = &saves[0]
	for i := 1; i < len(saves); i++ {
		if saves[i].UpdatedAt.After(best.UpdatedAt) {
			best = &saves[i]
		}
	}
	return best
}
