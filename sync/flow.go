package sync

import (
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/romm"
	"grout/version"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

func ResolveSaveSync(client *romm.Client, config *internal.Config, deviceID string) (SyncResult, error) {
	logger := gaba.GetLogger()
	logger.Debug("Starting save sync resolve", "deviceID", deviceID)

	localSaves := ScanSaves(config)
	logger.Debug("Scanned local saves", "count", len(localSaves))

	// Build client save states for negotiate
	var clientStates []romm.ClientSaveState
	for _, ls := range localSaves {
		info, err := os.Stat(ls.FilePath)
		if err != nil {
			logger.Warn("Cannot stat local save, skipping", "path", ls.FilePath, "error", err)
			continue
		}

		state := romm.ClientSaveState{
			RomID:         ls.RomID,
			FileName:      ls.FileName,
			UpdatedAt:     info.ModTime().Truncate(time.Second),
			FileSizeBytes: info.Size(),
		}

		if config != nil {
			state.Slot = config.GetSlotPreference(ls.RomID)
		}

		emulator := filepath.Base(ls.EmulatorDir)
		if emulator != "." && emulator != "" {
			state.Emulator = emulator
		}

		if hash, err := fileutil.ComputeMD5(ls.FilePath); err == nil {
			state.ContentHash = hash
		}

		clientStates = append(clientStates, state)
	}

	logger.Debug("Built client save states", "count", len(clientStates))

	// Negotiate with server
	resp, err := client.Negotiate(romm.SyncNegotiatePayload{
		DeviceID: deviceID,
		Saves:    clientStates,
	})
	if err != nil {
		return SyncResult{}, fmt.Errorf("negotiate failed: %w", err)
	}

	logger.Debug("Negotiate response",
		"uploads", resp.TotalUpload,
		"downloads", resp.TotalDownload,
		"conflicts", resp.TotalConflict,
		"no_ops", resp.TotalNoOp,
	)

	// Build a lookup of local saves by (rom_id, file_name)
	type localKey struct {
		romID    int
		fileName string
	}
	localByKey := make(map[localKey]LocalSave)
	for _, ls := range localSaves {
		localByKey[localKey{ls.RomID, ls.FileName}] = ls
	}

	// Resolve local ROMs for download path resolution
	scan := cfw.ScanRoms(config)
	resolvedRoms := ResolveLocalRoms(scan)

	cm := cache.GetCacheManager()

	// Map operations to SyncItems
	var allItems []SyncItem
	// Track download operations by rom_id for slot filtering
	type downloadOp struct {
		op    romm.SyncOperationSchema
		index int // index in allItems where this will be placed
	}
	downloadsByRom := make(map[int][]downloadOp)

	for _, op := range resp.Operations {
		switch op.Action {
		case "upload":
			ls, ok := localByKey[localKey{op.RomID, op.FileName}]
			if !ok {
				logger.Warn("Negotiate returned upload for unknown local save", "romID", op.RomID, "fileName", op.FileName)
				continue
			}
			targetSlot := "default"
			if config != nil {
				targetSlot = config.GetSlotPreference(ls.RomID)
			}
			allItems = append(allItems, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				TargetSlot: targetSlot,
				Action:     ActionUpload,
			})

		case "download":
			// Build LocalSave from local match or ROM resolution
			ls, ok := localByKey[localKey{op.RomID, op.FileName}]
			if !ok {
				// Remote-only save — resolve from ROM cache
				ls = resolveLocalSaveForDownload(op, resolvedRoms, cm, config)
			}

			idx := len(allItems)
			allItems = append(allItems, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				Action:     ActionDownload,
			})
			downloadsByRom[op.RomID] = append(downloadsByRom[op.RomID], downloadOp{op: op, index: idx})

		case "conflict":
			ls, ok := localByKey[localKey{op.RomID, op.FileName}]
			if !ok {
				logger.Warn("Negotiate returned conflict for unknown local save", "romID", op.RomID, "fileName", op.FileName)
				continue
			}
			allItems = append(allItems, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				Action:     ActionConflict,
			})

		case "no_op":
			// Skip — nothing to do
		}
	}

	// Handle multi-slot downloads: if a ROM has downloads from multiple slots,
	// filter to the preferred slot or flag for UI selection
	for romID, ops := range downloadsByRom {
		if len(ops) <= 1 {
			continue
		}

		// Collect distinct slots
		slotSet := make(map[string]bool)
		for _, dop := range ops {
			slot := "default"
			if dop.op.Slot != nil {
				slot = *dop.op.Slot
			}
			slotSet[slot] = true
		}

		if len(slotSet) <= 1 {
			continue
		}

		// Multiple slots — check if user has a preference
		preferredSlot := "default"
		if config != nil {
			preferredSlot = config.GetSlotPreference(romID)
		}

		// Find the operation matching the preferred slot
		preferredIdx := -1
		for _, dop := range ops {
			slot := "default"
			if dop.op.Slot != nil {
				slot = *dop.op.Slot
			}
			if slot == preferredSlot {
				preferredIdx = dop.index
				break
			}
		}

		if preferredIdx >= 0 {
			// Keep only the preferred slot, mark others as skip
			for _, dop := range ops {
				if dop.index != preferredIdx {
					allItems[dop.index].Action = ActionSkip
				}
			}
		} else {
			// No preference match — keep the first one and flag available slots for UI
			var sortedSlots []string
			for slot := range slotSet {
				sortedSlots = append(sortedSlots, slot)
			}
			sort.Strings(sortedSlots)

			// Keep only the first download, mark others as skip
			kept := ops[0].index
			allItems[kept].AvailableSlots = sortedSlots
			for _, dop := range ops[1:] {
				allItems[dop.index].Action = ActionSkip
			}
		}
	}

	logger.Debug("Total sync items resolved", "count", len(allItems))
	return SyncResult{Items: allItems, SessionID: resp.SessionID}, nil
}

func ExecuteSaveSync(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, sessionID int, progressFn func(current, total int)) SyncReport {
	report := ExecuteActions(client, config, deviceID, items, sessionID, progressFn)

	cm := cache.GetCacheManager()
	if cm != nil {
		for _, item := range report.Items {
			if item.Action == ActionSkip || item.Action == ActionConflict || !item.Success {
				continue
			}
			fileName := item.LocalSave.FileName
			if fileName == "" && item.RemoteSave != nil {
				fileName = item.RemoteSave.FileName
			}
			record := cache.SaveSyncRecord{
				RomID:    item.LocalSave.RomID,
				RomName:  item.LocalSave.RomName,
				Action:   item.Action.String(),
				DeviceID: deviceID,
				FileName: fileName,
			}
			if item.RemoteSave != nil {
				record.SaveID = item.RemoteSave.ID
			}
			cm.RecordSaveSync(record)
		}
	}

	// Complete the sync session
	if sessionID > 0 {
		completed := report.Uploaded + report.Downloaded
		failed := report.Errors
		if err := client.CompleteSession(sessionID, romm.SyncCompletePayload{
			OperationsCompleted: completed,
			OperationsFailed:    failed,
		}); err != nil {
			gaba.GetLogger().Warn("Failed to complete sync session", "sessionID", sessionID, "error", err)
		}
	}

	return report
}

func RegisterDevice(client *romm.Client, name string) (romm.Device, error) {
	return client.RegisterDevice(romm.RegisterDeviceRequest{
		Name:          name,
		Platform:      string(cfw.GetCFW()),
		Client:        "grout",
		ClientVersion: version.Get().Version,
	})
}

// buildRemoteSaveStub creates a romm.Save from a negotiate operation for use in SyncItem.
func buildRemoteSaveStub(op romm.SyncOperationSchema) *romm.Save {
	if op.SaveID == nil && op.ServerUpdatedAt == nil {
		return nil
	}

	save := &romm.Save{
		RomID:    op.RomID,
		FileName: op.FileName,
		Emulator: op.Emulator,
	}

	if op.SaveID != nil {
		save.ID = *op.SaveID
	}
	if op.Slot != nil {
		save.Slot = op.Slot
	}
	if op.ServerUpdatedAt != nil {
		save.UpdatedAt = *op.ServerUpdatedAt
	}

	// Derive file extension from file name
	ext := filepath.Ext(op.FileName)
	if ext != "" {
		save.FileExtension = strings.TrimPrefix(ext, ".")
	}

	return save
}

// resolveLocalSaveForDownload builds a LocalSave for a download-only operation
// (remote save exists but no local save file).
func resolveLocalSaveForDownload(op romm.SyncOperationSchema, resolvedRoms map[int]cfw.LocalRomFile, cm *cache.Manager, config *internal.Config) LocalSave {
	ls := LocalSave{
		RomID:    op.RomID,
		FileName: op.FileName,
	}

	// Try to get ROM info from resolved local ROMs
	if rom, ok := resolvedRoms[op.RomID]; ok {
		ls.RomName = rom.RomName
		ls.FSSlug = rom.FSSlug
		ls.RomFileName = rom.FileName
	} else if cm != nil {
		// Fallback: try cache lookup by ROM ID
		if roms, err := cm.GetGamesByIDs([]int{op.RomID}); err == nil && len(roms) > 0 {
			ls.RomName = roms[0].Name
			ls.FSSlug = roms[0].PlatformFSSlug
		}
	}

	return ls
}
