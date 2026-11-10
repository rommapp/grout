package saves

import (
	"errors"
	"path/filepath"

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
	resolved := ResolveLocalRoms(cfw.ScanRoms(*config))
	romID, ok := FindRomByPath(resolved, romPath)
	if !ok {
		return LaunchPlan{}, ErrRomNotMatched
	}

	result, err := resolveSaveSync(client, config, deviceID, map[int]cfw.LocalRomFile{romID: resolved[romID]})
	if err != nil {
		return LaunchPlan{}, err
	}

	plan := LaunchPlan{SessionID: result.SessionID, RomID: romID}
	plan.Items, plan.Held = ForLaunch(result.Items, romID)
	if uploadOnly {
		plan.Items = UploadsOnly(plan.Items)
	}
	gaba.GetLogger().Debug("Launch sync items", "romID", romID, "count", len(plan.Items), "held", plan.Held)
	return plan, nil
}

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
