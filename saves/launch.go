package saves

import (
	"errors"
	"path/filepath"
	"time"

	"grout/cfw"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// LaunchPlan is what a launch sync would do for one game.
type LaunchPlan struct {
	Items     []SyncItem
	SessionID int
	RomID     int
	// Held counts the things left for the next manual sync because they need
	// an answer: conflicts, and games with saves in several server slots.
	Held int
	// Ops is what RomM said about the game and Dropped the part of it that was
	// not acted on; Local and LocalWrote are the device's saves for the game
	// and when the newest was written, taken before anything changed.
	Ops        []romm.SyncOperationSchema
	Dropped    []DroppedOp
	Local      []LocalSave
	LocalWrote time.Time
}

// ResolveRomOnLaunch works out what bringing one game's save up to date would
// take, for the moment just before the frontend starts that game. It only
// reads, so a caller may give up on it at any point without leaving anything
// half done.
//
// Nobody is there to answer a prompt, so whatever would need an answer is left
// exactly as it is: a conflict, or a choice between several server slots, is
// dropped from the run and shows up in the next manual sync as it always did.
// Everything else the server asks for, uploads included, can be carried out
// with ExecuteSaveSync.
//
// With uploadOnly it keeps nothing but uploads, for after a game has been
// quit: the save on the device is then the newest there is, so a download
// would only ever be the wrong answer.
func ResolveRomOnLaunch(client *romm.Client, config *settings.Config, deviceID, romPath string, uploadOnly bool) (LaunchPlan, error) {
	// Found before the server is asked anything: a negotiate that is never
	// completed leaves a session open on the server.
	resolved := ResolveLocalRoms(onlyRom(cfw.ScanRoms(*config), romPath))
	romID, ok := FindRomByPath(resolved, romPath)
	if !ok {
		return LaunchPlan{}, ErrRomNotMatched
	}

	// Local, so it is settled before the server is asked anything.
	local := LocalSavesFor(config, romID)
	if cfw.ActiveFirmware().KeepsSavesPerCore() {
		local, _ = tidyIdenticalCopies(local, func(int) string {
			return cfw.ActiveFirmware().SaveFolderForRom(romPath)
		})
	}
	if duplicates := FindDuplicateSaves(local); len(duplicates) > 0 && cfw.ActiveFirmware().KeepsSavesPerCore() {
		folders := make([]string, 0, len(duplicates[0].Copies))
		for _, copy := range duplicates[0].Copies {
			folders = append(folders, FolderName(copy))
		}
		gaba.GetLogger().Warn("Launch sync: the game has saves in more than one folder", "rom", romPath, "folders", folders)
		return LaunchPlan{RomID: romID}, ErrAmbiguousSaves
	}

	result, err := resolveSaveSync(client, config, deviceID, map[int]cfw.LocalRomFile{romID: resolved[romID]})
	if err != nil {
		return LaunchPlan{}, err
	}

	plan := LaunchPlan{SessionID: result.SessionID, RomID: romID, Local: local, LocalWrote: NewestWrite(local)}
	plan.Items, plan.Held = ForLaunch(result.Items, romID)
	for _, op := range result.Operations {
		if op.RomID == romID {
			plan.Ops = append(plan.Ops, op)
		}
	}
	for _, drop := range result.Dropped {
		if drop.Op.RomID == romID {
			plan.Dropped = append(plan.Dropped, drop)
		}
	}
	if uploadOnly {
		plan.Items = UploadsOnly(plan.Items)
	}
	gaba.GetLogger().Debug("Launch sync items", "romID", romID, "count", len(plan.Items), "held", plan.Held)
	return plan, nil
}

// ErrAmbiguousSaves means the game has a save in more than one emulator's folder,
// so there is no telling which is the one to sync.
var ErrAmbiguousSaves = errors.New("saves in more than one folder")

// ErrRomNotMatched means the game is not one RomM knows about, or is not on the
// device where Grout looks for games.
var ErrRomNotMatched = errors.New("not a ROM matched to RomM")

// ForLaunch keeps the items for one ROM that can be carried out unattended,
// and counts the ones that need an answer.
func ForLaunch(items []SyncItem, romID int) (kept []SyncItem, held int) {
	for _, item := range items {
		if item.LocalSave.RomID != romID {
			continue
		}
		if item.Action == ActionConflict || len(item.AvailableSlots) > 1 {
			held++
			continue
		}
		kept = append(kept, item)
	}
	return kept, held
}

// onlyRom narrows a scan of the ROMs on the device to the one being launched.
// Matching a ROM to RomM is a database lookup per file, which adds up over a
// whole library on a handheld, and a launch sync needs only the one.
func onlyRom(scan cfw.LocalRomScan, romPath string) cfw.LocalRomScan {
	want := canonicalPath(romPath)
	narrowed := cfw.LocalRomScan{}
	for fsSlug, roms := range scan {
		for _, rom := range roms {
			if canonicalPath(rom.FilePath) == want {
				narrowed[fsSlug] = append(narrowed[fsSlug], rom)
			}
		}
	}
	return narrowed
}

// UploadsOnly keeps just the uploads.
func UploadsOnly(items []SyncItem) []SyncItem {
	var kept []SyncItem
	for _, item := range items {
		if item.Action == ActionUpload {
			kept = append(kept, item)
		}
	}
	return kept
}

// FindRomByPath finds the ROM a frontend was asked to launch among the ROMs on
// the device. Frontends hand over the path in different spellings (relative,
// through a symlink), so the paths are compared after cleaning and, where the
// file is there, resolving links.
func FindRomByPath(resolved map[int]cfw.LocalRomFile, romPath string) (int, bool) {
	want := canonicalPath(romPath)
	for id, rom := range resolved {
		if canonicalPath(rom.FilePath) == want {
			return id, true
		}
	}
	return 0, false
}

func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}
