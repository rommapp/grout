package sync

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/internal/pspdb"
	"grout/romm"
	"grout/version"
	"os"
	"path/filepath"
	"sort"
	"strings"
	gosync "sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// maxConcurrentRequests bounds the per-ROM save fetches in the discovery fallback.
const maxConcurrentRequests = 8

func ResolveSaveSync(client *romm.Client, config *internal.Config, deviceID string) (SyncResult, error) {
	logger := gaba.GetLogger()
	logger.Debug("Starting save sync resolve (negotiate)", "deviceID", deviceID)

	localSaves := ScanSaves(config)
	logger.Debug("Scanned local saves", "count", len(localSaves))

	recordedSlots := loadRecordedSlots(deviceID)
	states := buildClientSaveStates(localSaves, config, recordedSlots)

	// Diagnostic: log exactly what we send the orchestrator (rom/slot/hash).
	for _, s := range states {
		logger.Debug("Negotiate request save",
			"romID", s.RomID, "file", s.FileName, "slot", s.Slot,
			"emulator", s.Emulator, "hasHash", s.ContentHash != "", "size", s.FileSizeBytes)
	}

	resp, err := client.Negotiate(romm.SyncNegotiatePayload{
		DeviceID: deviceID,
		Saves:    states,
	})
	if err != nil {
		return SyncResult{}, fmt.Errorf("negotiate failed: %w", err)
	}
	logger.Debug("Negotiate response",
		"sessionID", resp.SessionID,
		"uploads", resp.TotalUpload,
		"downloads", resp.TotalDownload,
		"conflicts", resp.TotalConflict,
		"no_ops", resp.TotalNoOp,
		"operations", len(resp.Operations),
	)
	// Diagnostic: log every operation the orchestrator returned (action/slot/reason).
	for _, op := range resp.Operations {
		saveID := 0
		if op.SaveID != nil {
			saveID = *op.SaveID
		}
		slot := ""
		if op.Slot != nil {
			slot = *op.Slot
		}
		logger.Debug("Negotiate op",
			"action", op.Action, "romID", op.RomID, "saveID", saveID,
			"file", op.FileName, "slot", slot, "reason", op.Reason)
	}

	scan := cfw.ScanRoms(config)
	resolvedRoms := ResolveLocalRoms(scan)
	cm := cache.GetCacheManager()

	items := mapOperationsToItems(resp.Operations, localSaves, resolvedRoms, cm, config, recordedSlots)

	// Discovery fallback: the orchestrator only volunteers downloads for non-null-slot
	// saves the device hasn't already synced, and never surfaces null-slot ("archival" /
	// web-UI) saves at all. So for locally-present ROMs that have no local save and no
	// negotiate op, query the server directly and pull the best server save. Discovery
	// only runs when there is no local file, so it safely restores saves after an SD
	// reflash / fresh install (persistent device_id, lost local files).
	discovered := discoverRemoteOnlySaves(client, config, deviceID, localSaves, items, resolvedRoms)
	if len(discovered) > 0 {
		logger.Debug("Discovery fallback found remote-only saves", "count", len(discovered))
		items = append(items, discovered...)
	}

	logger.Debug("Total sync items resolved", "count", len(items))

	// TEMP diagnostic: final per-game decision. Remove once save sync is validated.
	for i := range items {
		it := items[i]
		remoteID := 0
		remoteSlot := "<nil>"
		if it.RemoteSave != nil {
			remoteID = it.RemoteSave.ID
			if it.RemoteSave.Slot != nil {
				remoteSlot = *it.RemoteSave.Slot
			}
		}
		logger.Debug("Sync decision",
			"romID", it.LocalSave.RomID,
			"romName", it.LocalSave.RomName,
			"action", it.Action.String(),
			"targetSlot", it.TargetSlot,
			"remoteSaveID", remoteID,
			"remoteSlot", remoteSlot,
			"localFile", it.LocalSave.FileName,
			"hasLocalPath", it.LocalSave.FilePath != "")
	}

	return SyncResult{Items: items, SessionID: resp.SessionID}, nil
}

// buildDiscoveryItems turns server saves for uncovered ROMs into download items.
// Discovery only runs for ROMs that have NO local save, so there is nothing to
// clobber: we pull the best server save regardless of this device's prior sync
// state. This restores saves after an SD reflash / fresh install (where the
// device_id persists but local files are gone), which the orchestrator's
// deletion-propagation model would otherwise suppress. Null-slot ("archival" /
// web-UI) saves are included, since negotiate never surfaces them. Pure function:
// the caller supplies the fetched saves keyed by ROM ID.
func buildDiscoveryItems(uncovered map[int]cfw.LocalRomFile, savesByRom map[int][]romm.Save, config *internal.Config) []SyncItem {
	logger := gaba.GetLogger()
	var items []SyncItem

	for romID, rom := range uncovered {
		saves := savesByRom[romID]
		if len(saves) == 0 {
			// TEMP diagnostic: downloaded game with no local save AND no server save.
			logger.Debug("Discovery: no server saves for ROM",
				"romID", romID, "romName", rom.RomName)
			continue
		}

		preferredSlot := "autosave"
		if config != nil {
			preferredSlot = config.GetSlotPreference(romID)
		}
		best := SelectSaveForSlot(saves, preferredSlot)
		if best == nil {
			continue
		}

		ls := LocalSave{
			RomID:       romID,
			RomName:     rom.RomName,
			FSSlug:      rom.FSSlug,
			RomFileName: rom.FileName,
		}
		if IsDirectorySavePlatform(ls.FSSlug) {
			ls.IsDirectorySave = true
		}

		logger.Debug("Discovery: remote-only save for ROM without local save",
			"romID", romID, "romName", rom.RomName, "saveID", best.ID,
			"file", best.FileName, "fetched", len(saves))

		items = append(items, SyncItem{
			LocalSave:  ls,
			RemoteSave: best,
			TargetSlot: preferredSlot,
			Action:     ActionDownload,
		})
	}

	return items
}

// discoverRemoteOnlySaves finds locally-present ROMs that have no local save and were
// not covered by a negotiate operation, fetches their server saves, and builds download
// items for any save this device has never synced.
func discoverRemoteOnlySaves(client *romm.Client, config *internal.Config, deviceID string, localSaves []LocalSave, items []SyncItem, resolvedRoms map[int]cfw.LocalRomFile) []SyncItem {
	logger := gaba.GetLogger()

	covered := make(map[int]bool, len(localSaves)+len(items))
	for _, ls := range localSaves {
		covered[ls.RomID] = true
	}
	for _, it := range items {
		covered[it.LocalSave.RomID] = true
	}

	uncovered := make(map[int]cfw.LocalRomFile)
	for romID, rom := range resolvedRoms {
		if !covered[romID] {
			uncovered[romID] = rom
		}
	}
	if len(uncovered) == 0 {
		return nil
	}

	logger.Debug("Discovery: checking remote saves for ROMs without local saves", "count", len(uncovered))

	savesByRom := fetchSavesForRoms(client, deviceID, uncovered)
	return buildDiscoveryItems(uncovered, savesByRom, config)
}

// fetchSavesForRoms queries the server for each ROM's saves with bounded concurrency.
// ROMs whose fetch errors are logged and omitted.
func fetchSavesForRoms(client *romm.Client, deviceID string, uncovered map[int]cfw.LocalRomFile) map[int][]romm.Save {
	logger := gaba.GetLogger()

	type result struct {
		romID int
		saves []romm.Save
		err   error
	}

	results := make(chan result, len(uncovered))
	sem := make(chan struct{}, maxConcurrentRequests)
	var wg gosync.WaitGroup

	for romID := range uncovered {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			saves, err := client.GetSaves(romm.SaveQuery{RomID: id, DeviceID: deviceID})
			results <- result{romID: id, saves: saves, err: err}
		}(romID)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make(map[int][]romm.Save, len(uncovered))
	for r := range results {
		if r.err != nil {
			logger.Warn("Discovery: failed to fetch saves for ROM", "romID", r.romID, "error", r.err)
			continue
		}
		if len(r.saves) > 0 {
			out[r.romID] = r.saves
		}
	}
	return out
}

// mapOperationsToItems converts negotiate operations into executable SyncItems,
// dropping no_op. Order is preserved. Downloads with no local file are resolved
// from the local ROM scan / cache for path determination.
func mapOperationsToItems(
	ops []romm.SyncOperationSchema,
	localSaves []LocalSave,
	resolvedRoms map[int]cfw.LocalRomFile,
	cm *cache.Manager,
	config *internal.Config,
	recordedSlots map[saveKey]string,
) []SyncItem {
	logger := gaba.GetLogger()

	type localKey struct {
		romID    int
		fileName string
	}
	byKey := make(map[localKey]LocalSave, len(localSaves))
	// localByRom holds one local save per ROM. grout manages a single save (one slot)
	// per ROM, so a ROM that already has a local save only ever syncs that save's slot.
	localByRom := make(map[int]LocalSave, len(localSaves))
	for _, ls := range localSaves {
		byKey[localKey{ls.RomID, ls.FileName}] = ls
		if _, ok := localByRom[ls.RomID]; !ok {
			localByRom[ls.RomID] = ls
		}
	}

	// A ROM is "installed" if it has a local save or a local ROM file. Downloads are
	// gated on this: negotiate enumerates the whole RomM library and offers downloads
	// for every never-synced save, but grout must only pull saves for games actually
	// present on this device.
	installed := make(map[int]bool, len(localSaves)+len(resolvedRoms))
	for _, ls := range localSaves {
		installed[ls.RomID] = true
	}
	for romID := range resolvedRoms {
		installed[romID] = true
	}

	items := make([]SyncItem, 0, len(ops))
	// Download ops are collected per ROM and resolved to a single item below, so a ROM
	// with saves in multiple slots doesn't download several files to the same path.
	downloadOps := make(map[int][]romm.SyncOperationSchema)

	for _, op := range ops {
		switch op.Action {
		case "upload":
			ls, ok := byKey[localKey{op.RomID, op.FileName}]
			if !ok {
				logger.Warn("Negotiate upload for unknown local save", "romID", op.RomID, "file", op.FileName)
				continue
			}
			slot := "autosave"
			if config != nil {
				slot = config.GetSlotPreference(ls.RomID)
			}
			items = append(items, SyncItem{
				LocalSave:  ls,
				RemoteSave: buildRemoteSaveStub(op),
				TargetSlot: slot,
				Action:     ActionUpload,
			})

		case "download":
			if !installed[op.RomID] {
				logger.Debug("Skipping download: ROM not downloaded locally", "romID", op.RomID, "file", op.FileName)
				continue
			}
			if buildRemoteSaveStub(op) == nil {
				logger.Error("Negotiate download op missing save identity (no save_id/server_updated_at)", "romID", op.RomID, "file", op.FileName)
				continue
			}
			// grout keeps one save (one slot) per ROM. If the ROM already has a local
			// save, only its own slot is managed — a download for any other slot is the
			// orchestrator offering an alternate-slot save we don't use here, and pulling
			// it would clobber the local save and flip-flop on every sync. Skip it.
			if ls, ok := localByRom[op.RomID]; ok {
				managedSlot := resolveReportedSlot(ls, config, recordedSlots)
				opSlot := "autosave"
				if op.Slot != nil && *op.Slot != "" {
					opSlot = *op.Slot
				}
				if opSlot != managedSlot {
					logger.Debug("Skipping download: ROM's local save is in a different slot",
						"romID", op.RomID, "opSlot", opSlot, "managedSlot", managedSlot, "file", op.FileName)
					continue
				}
			}
			downloadOps[op.RomID] = append(downloadOps[op.RomID], op)

		case "conflict":
			stub := buildRemoteSaveStub(op)
			if stub == nil {
				logger.Error("Negotiate conflict op missing save identity (no save_id/server_updated_at)", "romID", op.RomID, "file", op.FileName)
				continue
			}
			ls, ok := byKey[localKey{op.RomID, op.FileName}]
			if !ok {
				logger.Warn("Negotiate conflict for unknown local save", "romID", op.RomID, "file", op.FileName)
				continue
			}
			slot := "autosave"
			if config != nil {
				slot = config.GetSlotPreference(ls.RomID)
			}
			items = append(items, SyncItem{
				LocalSave:  ls,
				RemoteSave: stub,
				TargetSlot: slot,
				Action:     ActionConflict,
			})

		case "no_op":
			// nothing to do
		default:
			logger.Warn("Unknown negotiate action", "action", op.Action, "romID", op.RomID)
		}
	}

	// Resolve each installed ROM's download ops to a single item, preferring the slot
	// the ROM is reported under (explicit pref / autosave), else the latest save.
	for romID, dops := range downloadOps {
		preferred := "autosave"
		if config != nil {
			preferred = config.GetSlotPreference(romID)
		}
		op := pickDownloadOp(dops, preferred)

		ls, ok := byKey[localKey{op.RomID, op.FileName}]
		if !ok {
			ls = resolveLocalSaveForDownload(op, resolvedRoms, cm)
		}
		items = append(items, SyncItem{
			LocalSave:  ls,
			RemoteSave: buildRemoteSaveStub(op),
			TargetSlot: preferred,
			Action:     ActionDownload,
		})
	}

	return items
}

// pickDownloadOp chooses one download op for a ROM that has saves in possibly
// multiple slots: the op matching preferredSlot if present, otherwise the one with
// the latest server_updated_at.
func pickDownloadOp(ops []romm.SyncOperationSchema, preferredSlot string) romm.SyncOperationSchema {
	for _, op := range ops {
		slot := "autosave"
		if op.Slot != nil && *op.Slot != "" {
			slot = *op.Slot
		}
		if slot == preferredSlot {
			return op
		}
	}
	best := ops[0]
	for _, op := range ops[1:] {
		if op.ServerUpdatedAt == nil {
			continue
		}
		if best.ServerUpdatedAt == nil || op.ServerUpdatedAt.After(*best.ServerUpdatedAt) {
			best = op
		}
	}
	return best
}

// buildRemoteSaveStub builds a *romm.Save from a negotiate operation for execution.
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
	if ext := filepath.Ext(op.FileName); ext != "" {
		save.FileExtension = strings.TrimPrefix(ext, ".")
	}
	return save
}

// resolveLocalSaveForDownload builds a LocalSave for a download whose file does
// not exist locally yet, resolving ROM metadata for path determination.
func resolveLocalSaveForDownload(op romm.SyncOperationSchema, resolvedRoms map[int]cfw.LocalRomFile, cm *cache.Manager) LocalSave {
	ls := LocalSave{RomID: op.RomID, FileName: op.FileName}

	if rom, ok := resolvedRoms[op.RomID]; ok {
		ls.RomName = rom.RomName
		ls.FSSlug = rom.FSSlug
		ls.RomFileName = rom.FileName
	} else if cm != nil {
		if roms, err := cm.GetGamesByIDs([]int{op.RomID}); err == nil && len(roms) > 0 {
			ls.RomName = roms[0].Name
			ls.FSSlug = roms[0].PlatformFSSlug
		}
	}

	if IsDirectorySavePlatform(ls.FSSlug) {
		ls.IsDirectorySave = true
	}
	return ls
}

func ExecuteSaveSync(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, sessionID int, progressFn func(current, total int)) SyncReport {
	report := ExecuteActions(client, config, deviceID, items, progressFn)

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

	if sessionID > 0 {
		if err := client.CompleteSession(sessionID, romm.SyncCompletePayload{
			OperationsCompleted: report.Uploaded + report.Downloaded,
			OperationsFailed:    report.Errors,
		}); err != nil {
			// On-demand client has no retry queue; the server expires stale sessions.
			gaba.GetLogger().Warn("Failed to complete sync session (leaving for server to expire)", "sessionID", sessionID, "error", err)
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
		SyncMode:      "api",
	})
}

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
			logger.Debug("Checking save directory", "path", saveDir, "fsSlug", rommFSSlug)

			if _, err := os.Stat(saveDir); os.IsNotExist(err) {
				continue
			}

			entries, err := os.ReadDir(saveDir)
			if err != nil {
				logger.Error("Could not read save directory", "path", saveDir, "error", err)
				continue
			}

			if IsDirectorySavePlatform(fsSlug) {
				// Directory-based saves (e.g., PPSSPP): group all directories that
				// share the same Game ID and title into a single LocalSave, so that
				// DATA00/DATA01/INSDIR etc. are synced together as one zip.
				type pspGroup struct {
					title string
					dirs  []string
				}
				groups := make(map[string]*pspGroup)

				for _, entry := range entries {
					if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
						continue
					}
					gameID := extractPSPGameID(entry.Name())
					dirPath := filepath.Join(saveDir, entry.Name())

					if _, ok := groups[gameID]; !ok {
						groups[gameID] = &pspGroup{}
					}
					groups[gameID].dirs = append(groups[gameID].dirs, dirPath)

					if groups[gameID].title == "" {
						if title, ok := ReadPSPSaveTitle(dirPath); ok {
							groups[gameID].title = title
						}
					}
				}

				for gameID, group := range groups {
					// Normalize the Game ID to match pspdb keys (no hyphens or spaces)
					cleanGameID := strings.NewReplacer("-", "", " ", "").Replace(gameID)

					// Prefer the canonical title from pspdb, fall back to PARAM.SFO
					title, inDB := pspdb.Titles[cleanGameID]
					if !inDB {
						if group.title != "" {
							title = group.title
							logger.Debug("PSP game ID not in pspdb, using PARAM.SFO title", "gameID", gameID, "title", title)
						} else {
							logger.Debug("No title found for PSP game ID, skipping", "gameID", gameID, "fsSlug", rommFSSlug)
							continue
						}
					}

					rom, err := cm.GetRomByNameLookup(rommFSSlug, title)
					if err != nil {
						logger.Debug("No cache match for PSP save group", "gameID", gameID, "title", title, "inDB", inDB, "fsSlug", rommFSSlug)
						continue
					}

					sort.Strings(group.dirs)

					logger.Debug("Matched PSP save group to ROM", "gameID", gameID, "title", group.title, "dirCount", len(group.dirs), "romID", rom.ID, "romName", rom.Name)

					saves = append(saves, LocalSave{
						RomID:           rom.ID,
						RomName:         rom.Name,
						FSSlug:          rommFSSlug,
						FileName:        gameID + ".zip",
						FilePath:        group.dirs[0],
						EmulatorDir:     emuDir,
						IsDirectorySave: true,
						GameID:          gameID,
						RelatedDirs:     group.dirs,
					})
				}
			} else {
				// File-based saves: scan for individual save files
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
	}

	logger.Debug("Completed save scan", "matched", len(saves))
	return saves
}

// buildClientSaveStates converts scanned local saves into the negotiate payload,
// computing a content hash per save (composite for directory saves, MD5 for files)
// and the slot from the user's per-ROM preference (default "autosave").
// saveKey identifies a local save for save-state record lookups.
type saveKey struct {
	romID    int
	fileName string
}

// loadRecordedSlots reads the persisted save-sync state for a device into a slot
// lookup keyed by (rom_id, file_name).
func loadRecordedSlots(deviceID string) map[saveKey]string {
	cm := cache.GetCacheManager()
	if cm == nil {
		return nil
	}
	states := cm.GetSaveStates(deviceID)
	if len(states) == 0 {
		return nil
	}
	out := make(map[saveKey]string, len(states))
	for _, s := range states {
		out[saveKey{s.RomID, s.FileName}] = s.Slot
	}
	return out
}

// recordSaveState upserts the current synced state for a local save after a
// successful upload or download, so subsequent syncs report the same slot/identity.
func recordSaveState(deviceID string, romID int, fileName, slot string, saveID int, contentHash string) {
	cm := cache.GetCacheManager()
	if cm == nil || fileName == "" {
		return
	}
	if err := cm.UpsertSaveState(deviceID, cache.SaveSyncState{
		RomID:       romID,
		FileName:    fileName,
		Slot:        slot,
		SaveID:      saveID,
		ContentHash: contentHash,
	}); err != nil {
		gaba.GetLogger().Warn("Failed to record save state", "romID", romID, "file", fileName, "error", err)
	}
}

// resolveReportedSlot determines which slot a local save is reported under during
// negotiate. Precedence: explicit user preference > recorded last-synced slot >
// "autosave" default. This gives downloaded saves a stable slot identity so they are
// not spuriously re-uploaded to a different slot on the next sync.
func resolveReportedSlot(ls LocalSave, config *internal.Config, recordedSlots map[saveKey]string) string {
	slot := "autosave"
	if rec, ok := recordedSlots[saveKey{ls.RomID, ls.FileName}]; ok && rec != "" {
		slot = rec
	}
	if config != nil {
		if pref, ok := config.SlotPreferenceExplicit(ls.RomID); ok {
			slot = pref
		}
	}
	return slot
}

func buildClientSaveStates(localSaves []LocalSave, config *internal.Config, recordedSlots map[saveKey]string) []romm.ClientSaveState {
	logger := gaba.GetLogger()
	states := make([]romm.ClientSaveState, 0, len(localSaves))

	for _, ls := range localSaves {
		slot := resolveReportedSlot(ls, config, recordedSlots)

		emulator := filepath.Base(ls.EmulatorDir)
		if emulator == "." || emulator == "" {
			emulator = ""
		} else if emulator == "SAVEDATA" {
			emulator = "PPSSPP"
		}

		var updatedAt time.Time
		var size int64

		if ls.IsDirectorySave {
			dirs := ls.RelatedDirs
			if len(dirs) == 0 {
				dirs = []string{ls.FilePath}
			}
			updatedAt, size = dirNewestMtimeAndSize(dirs)
		} else {
			info, err := os.Stat(ls.FilePath)
			if err != nil {
				logger.Warn("Cannot stat local save, skipping from negotiate", "path", ls.FilePath, "error", err)
				continue
			}
			updatedAt = info.ModTime().Truncate(time.Second)
			size = info.Size()
		}

		state := romm.ClientSaveState{
			RomID:         ls.RomID,
			FileName:      ls.FileName,
			Slot:          slot,
			Emulator:      emulator,
			UpdatedAt:     updatedAt,
			FileSizeBytes: size,
		}
		hash, err := saveContentHash(ls)
		if err != nil {
			logger.Warn("Failed to hash local save; skipping from negotiate", "romID", ls.RomID, "path", ls.FilePath, "error", err)
			continue
		}
		state.ContentHash = hash

		states = append(states, state)
	}

	return states
}

// saveContentHash returns the server-compatible content hash for a local save.
func saveContentHash(ls LocalSave) (string, error) {
	if ls.IsDirectorySave {
		dirs := ls.RelatedDirs
		if len(dirs) == 0 {
			dirs = []string{ls.FilePath}
		}
		return fileutil.ComputeDirsCompositeHash(dirs)
	}
	return fileutil.ComputeMD5(ls.FilePath)
}

// dirNewestMtimeAndSize returns the newest file mtime (sec-truncated) and total
// byte size across the given directories.
func dirNewestMtimeAndSize(dirs []string) (time.Time, int64) {
	var newest time.Time
	var total int64
	for _, dir := range dirs {
		filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				gaba.GetLogger().Warn("Walk error scanning directory save", "dir", dir, "error", err)
				return nil
			}
			if info.IsDir() {
				return nil
			}
			total += info.Size()
			if mt := info.ModTime(); mt.After(newest) {
				newest = mt
			}
			return nil
		})
	}
	return newest.Truncate(time.Second), total
}

// SelectSaveForSlot picks the latest save in preferredSlot, falling back to the
// most recently updated save across all slots. Used by the multi-slot download UI.
func SelectSaveForSlot(saves []romm.Save, preferredSlot string) *romm.Save {
	if len(saves) == 0 {
		return nil
	}
	var best *romm.Save
	for i := range saves {
		slot := "autosave"
		if saves[i].Slot != nil {
			slot = *saves[i].Slot
		}
		if slot != preferredSlot {
			continue
		}
		if best == nil || saves[i].UpdatedAt.After(best.UpdatedAt) {
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

func ExecuteActions(client *romm.Client, config *internal.Config, deviceID string, items []SyncItem, progressFn func(current, total int)) SyncReport {
	logger := gaba.GetLogger()
	report := SyncReport{}

	actionable := 0
	for _, item := range items {
		if item.Action != ActionSkip && item.Action != ActionConflict {
			actionable++
		}
	}

	logger.Debug("Executing sync actions", "total", len(items), "actionable", actionable)

	current := 0
	for i := range items {
		item := &items[i]

		switch item.Action {
		case ActionUpload:
			current++
			if progressFn != nil {
				progressFn(current, actionable)
			}
			switch upload(client, deviceID, item) {
			case uploadOK:
				item.Success = true
				report.Uploaded++
			case uploadConflict:
				item.Action = ActionConflict
				report.Conflicts++
			default:
				report.Errors++
			}

		case ActionDownload:
			current++
			if progressFn != nil {
				progressFn(current, actionable)
			}
			if download(client, config, deviceID, item) {
				item.Success = true
				report.Downloaded++
			} else {
				report.Errors++
			}

		case ActionConflict:
			report.Conflicts++

		default:
			report.Skipped++
		}
	}

	report.Items = items
	logger.Debug("Sync execution complete", "uploaded", report.Uploaded, "downloaded", report.Downloaded, "skipped", report.Skipped, "errors", report.Errors)
	return report
}

type uploadOutcome int

const (
	uploadErr uploadOutcome = iota
	uploadOK
	uploadConflict
)

func upload(client *romm.Client, deviceID string, item *SyncItem) uploadOutcome {
	logger := gaba.GetLogger()
	logger.Debug("Uploading save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "file", item.LocalSave.FilePath)

	slot := "autosave"
	if item.TargetSlot != "" {
		slot = item.TargetSlot
	} else if item.RemoteSave != nil && item.RemoteSave.Slot != nil {
		slot = *item.RemoteSave.Slot
	}

	emulator := filepath.Base(item.LocalSave.EmulatorDir)
	if emulator == "." || emulator == "" {
		emulator = "unknown"
	}
	if emulator == "SAVEDATA" { // PSP folder name varies across CFW
		emulator = "PPSSPP"
	}

	query := romm.UploadSaveQuery{
		RomID:     item.LocalSave.RomID,
		DeviceID:  deviceID,
		Emulator:  emulator,
		Slot:      slot,
		Overwrite: item.ForceOverwrite || item.RemoteSave != nil,
	}
	if slot == "autosave" {
		query.Autocleanup = true
		query.AutocleanupLimit = 10
	}

	uploadPath := item.LocalSave.FilePath
	if item.LocalSave.IsDirectorySave {
		dirs := item.LocalSave.RelatedDirs
		if len(dirs) == 0 {
			dirs = []string{item.LocalSave.FilePath}
		}
		zipPath, zipErr := ZipDirectories(dirs)
		if zipErr != nil {
			logger.Error("Failed to zip directory save", "gameID", item.LocalSave.GameID, "error", zipErr)
			return uploadErr
		}
		defer os.Remove(zipPath)
		uploadPath = zipPath
	}

	uploadedSave, err := client.UploadSaveWithQuery(query, uploadPath)
	if err != nil {
		if errors.Is(err, romm.ErrConflict) {
			logger.Warn("Upload rejected with 409; surfacing as conflict", "romID", item.LocalSave.RomID, "error", err)
			return uploadConflict
		}
		logger.Error("Failed to upload save", "romID", item.LocalSave.RomID, "error", err)
		return uploadErr
	}

	// Match server precision so the next scan doesn't see a spurious change.
	if !item.LocalSave.IsDirectorySave {
		t := uploadedSave.UpdatedAt.Truncate(time.Second)
		if err := os.Chtimes(item.LocalSave.FilePath, t, t); err != nil {
			logger.Warn("Failed to set save mtime after upload", "path", item.LocalSave.FilePath, "error", err)
		}
	}
	// No MarkDeviceSynced: the server upserts last_synced_at automatically on
	// upload because device_id is supplied.

	// Record the synced state so the next scan reports this save under the same slot.
	hash, _ := saveContentHash(item.LocalSave)
	recordSaveState(deviceID, item.LocalSave.RomID, item.LocalSave.FileName, slot, uploadedSave.ID, hash)

	logger.Debug("Upload successful", "romID", item.LocalSave.RomID)
	return uploadOK
}

func download(client *romm.Client, config *internal.Config, deviceID string, item *SyncItem) bool {
	logger := gaba.GetLogger()

	if item.RemoteSave == nil {
		logger.Error("No remote save to download", "romID", item.LocalSave.RomID)
		return false
	}

	logger.Debug("Downloading save", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "saveID", item.RemoteSave.ID)

	if !item.LocalSave.IsDirectorySave && IsDirectorySavePlatform(item.LocalSave.FSSlug) {
		item.LocalSave.IsDirectorySave = true
	}

	if item.LocalSave.FilePath != "" {
		if info, err := os.Stat(item.LocalSave.FilePath); err == nil {
			backupDir := filepath.Join(filepath.Dir(item.LocalSave.FilePath), ".backup")
			ext := filepath.Ext(item.LocalSave.FileName)
			base := strings.TrimSuffix(item.LocalSave.FileName, ext)
			timestamp := info.ModTime().Format("2006-01-02 15-04-05")
			backupPath := filepath.Join(backupDir, fmt.Sprintf("%s [%s]%s", base, timestamp, ext))

			if err := os.MkdirAll(backupDir, 0755); err != nil {
				logger.Error("Failed to create backup directory, aborting download", "path", backupDir, "error", err)
				return false
			}

			var backupErr error
			if item.LocalSave.IsDirectorySave {
				// Zip all related directories into the backup path
				dirs := item.LocalSave.RelatedDirs
				if len(dirs) == 0 {
					dirs = []string{item.LocalSave.FilePath}
				}
				zipPath, zipErr := ZipDirectories(dirs)
				if zipErr != nil {
					backupErr = zipErr
				} else {
					defer os.Remove(zipPath)
					backupErr = fileutil.CopyFile(zipPath, backupPath)
				}
			} else {
				backupErr = fileutil.CopyFile(item.LocalSave.FilePath, backupPath)
			}

			if backupErr != nil {
				logger.Error("Failed to backup save before download, aborting download", "path", item.LocalSave.FilePath, "error", backupErr)
				return false
			}

			logger.Debug("Backed up save before download", "backup", backupPath)
			if config != nil && config.SaveBackupLimit > 0 {
				cleanupBackups(backupDir, base, config.SaveBackupLimit)
			}
		}
	}

	data, err := client.DownloadSaveByID(item.RemoteSave.ID, deviceID, false)
	if err != nil {
		logger.Error("Failed to download save", "romID", item.LocalSave.RomID, "saveID", item.RemoteSave.ID, "error", err)
		return false
	}

	savePath := item.LocalSave.FilePath
	if savePath == "" {
		saveDir := ResolveSaveDirectory(item.LocalSave.FSSlug, config)
		if saveDir != "" {
			fileName := item.RemoteSave.FileName
			if item.LocalSave.RomFileName != "" {
				romNameNoExt := strings.TrimSuffix(item.LocalSave.RomFileName, filepath.Ext(item.LocalSave.RomFileName))
				fileName = romNameNoExt + "." + item.RemoteSave.FileExtension
			}
			savePath = filepath.Join(saveDir, fileName)
		}
	}
	if savePath == "" {
		logger.Error("Could not determine save path", "romID", item.LocalSave.RomID, "fsSlug", item.LocalSave.FSSlug)
		return false
	}

	if item.LocalSave.IsDirectorySave {
		// Write zip to temp, then extract to the save directory
		tmpZip, err := os.CreateTemp("", "grout-save-dl-*.zip")
		if err != nil {
			logger.Error("Failed to create temp file for directory save", "error", err)
			return false
		}
		tmpZipPath := tmpZip.Name()
		defer os.Remove(tmpZipPath)

		if _, err := tmpZip.Write(data); err != nil {
			tmpZip.Close()
			logger.Error("Failed to write downloaded save zip", "error", err)
			return false
		}
		tmpZip.Close()

		// Remove all existing save directories before extracting.
		// For multi-dir PSP saves (DATA00, DATA01, INSDIR…), RelatedDirs holds
		// all of them; fall back to savePath for single-dir or remote-only cases.
		dirsToRemove := item.LocalSave.RelatedDirs
		if len(dirsToRemove) == 0 {
			dirsToRemove = []string{savePath}
		}
		for _, dir := range dirsToRemove {
			os.RemoveAll(dir)
		}

		if err := UnzipToDirectory(tmpZipPath, filepath.Dir(savePath)); err != nil {
			logger.Error("Failed to extract directory save", "path", savePath, "error", err)
			return false
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
			logger.Error("Failed to create save directory", "path", filepath.Dir(savePath), "error", err)
			return false
		}

		if err := os.WriteFile(savePath, data, 0644); err != nil {
			logger.Error("Failed to write save file", "path", savePath, "error", err)
			return false
		}
	}

	t := item.RemoteSave.UpdatedAt.Truncate(time.Second)
	if !item.LocalSave.IsDirectorySave {
		if err := os.Chtimes(savePath, t, t); err != nil {
			logger.Warn("Failed to set save file mtime", "path", savePath, "error", err)
		}
	}

	if err := client.ConfirmSaveDownloaded(item.RemoteSave.ID, deviceID); err != nil {
		logger.Warn("Failed to confirm save download", "saveID", item.RemoteSave.ID, "error", err)
	}

	// Record synced state. A null-slot ("archival") save is recorded as "autosave"
	// so the next sync promotes it to the autosave slot (matching Argosy); a named-slot
	// save keeps its slot so it isn't re-uploaded elsewhere. fileName matches what was
	// written to disk, so the next ScanSaves looks it up correctly.
	recordSlot := "autosave"
	if item.RemoteSave.Slot != nil && *item.RemoteSave.Slot != "" {
		recordSlot = *item.RemoteSave.Slot
	}
	recordFileName := item.LocalSave.FileName
	if recordFileName == "" {
		recordFileName = filepath.Base(savePath)
	}
	hash, _ := saveContentHash(LocalSave{
		FilePath:        savePath,
		IsDirectorySave: item.LocalSave.IsDirectorySave,
		RelatedDirs:     item.LocalSave.RelatedDirs,
	})
	recordSaveState(deviceID, item.LocalSave.RomID, recordFileName, recordSlot, item.RemoteSave.ID, hash)

	logger.Debug("Download successful", "romID", item.LocalSave.RomID, "romName", item.LocalSave.RomName, "path", savePath, "recordedSlot", recordSlot)
	return true
}

func cleanupBackups(backupDir string, baseName string, limit int) {
	if limit <= 0 {
		return
	}

	logger := gaba.GetLogger()
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	// Collect backup files for this game (matching base name prefix)
	type backupFile struct {
		name    string
		modTime int64
	}
	var backups []backupFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), baseName+" [") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{name: e.Name(), modTime: info.ModTime().UnixNano()})
	}

	if len(backups) <= limit {
		return
	}

	// Sort oldest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime < backups[j].modTime
	})

	// Remove oldest until we're at the limit
	for i := 0; i < len(backups)-limit; i++ {
		path := filepath.Join(backupDir, backups[i].name)
		if err := os.Remove(path); err != nil {
			logger.Warn("Failed to remove old backup", "path", path, "error", err)
		} else {
			logger.Debug("Removed old backup", "path", path)
		}
	}
}

// extractPSPGameID extracts the Game ID from a PSP save directory name.
// Two rules are applied in this order:
//  1. If the name contains an underscore, the Game ID is the part before it
//     (e.g. "UCUS98751_DATA00" → "UCUS98751", "UCUS98751_INSDIR" → "UCUS98751").
//  2. Otherwise, try to shrink prefixes from longest to shortest against
//     pspdb.Titles until a known Game ID is found
//     (e.g. "UCUS98653PROFILE00" → "UCUS98653").
//
// Falls back to the full directory name if no match is found.
func extractPSPGameID(dirName string) string {
	if idx := strings.Index(dirName, "_"); idx > 0 {
		return dirName[:idx]
	}
	for l := len(dirName) - 1; l > 0; l-- {
		if _, ok := pspdb.Titles[dirName[:l]]; ok {
			return dirName[:l]
		}
	}
	return dirName
}

func ResolveSaveDirectory(fsSlug string, config *internal.Config) string {
	if config != nil && config.SaveDirectoryMappings != nil {
		if mapped, ok := config.SaveDirectoryMappings[fsSlug]; ok && mapped != "" {
			baseSavePath := cfw.BaseSavePath()
			if baseSavePath != "" {
				return filepath.Join(baseSavePath, mapped)
			}
		}
	}

	effectiveFSSlug := fsSlug
	if config != nil {
		effectiveFSSlug = config.ResolveFSSlug(fsSlug)
	}

	return cfw.GetSaveDirectory(effectiveFSSlug)
}
