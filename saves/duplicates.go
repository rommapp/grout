package saves

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/files"
	"grout/romm"
	"grout/settings"
)

// DuplicateSaves is a game with a save in more than one emulator's folder, such
// as a GBA game with one in gpSP's and one in mGBA's.
//
// RomM keeps one save per game per slot, and the server pairs what it says with
// what is on the device by game and slot. Two copies on the device share that
// key, so one hides the other and the sync quietly does nothing for the game.
// Which copy is current is the player's to say.
type DuplicateSaves struct {
	RomID   int
	RomName string
	Copies  []LocalSave
	// FSSlug and RomFileName name the game's platform and ROM file, for putting
	// a save in place when the game has no copy on the device to take them from.
	FSSlug      string
	RomFileName string
}

// FolderName is the name of the emulator folder a copy is in.
func FolderName(save LocalSave) string { return filepath.Base(filepath.Dir(save.FilePath)) }

// FindDuplicateSaves finds the games that have saves in more than one folder.
// Saves kept as folders belong to other platforms and are not considered.
func FindDuplicateSaves(local []LocalSave) []DuplicateSaves {
	byRom := map[int][]LocalSave{}
	var order []int
	for _, save := range local {
		if save.IsDirectorySave {
			continue
		}
		if _, seen := byRom[save.RomID]; !seen {
			order = append(order, save.RomID)
		}
		byRom[save.RomID] = append(byRom[save.RomID], save)
	}

	var duplicates []DuplicateSaves
	for _, romID := range order {
		copies := byRom[romID]
		folders := map[string]bool{}
		for _, copy := range copies {
			folders[filepath.Dir(copy.FilePath)] = true
		}
		if len(folders) > 1 {
			duplicates = append(duplicates, DuplicateSaves{RomID: romID, RomName: copies[0].RomName, Copies: copies})
		}
	}
	return duplicates
}

// collapseDuplicates leaves one copy of each duplicated game in the list, the
// one in the folder its core reads if that is known and else the first, so the
// server still sees the game as present. liveFolder says which folder that is.
func collapseDuplicates(local []LocalSave, liveFolder func(romID int) string) ([]LocalSave, []DuplicateSaves) {
	duplicates := FindDuplicateSaves(local)
	if len(duplicates) == 0 {
		return local, nil
	}

	keep := map[int]string{}
	for _, dup := range duplicates {
		keep[dup.RomID] = dup.Copies[0].FilePath
		if live := liveFolder(dup.RomID); live != "" {
			for _, copy := range dup.Copies {
				if strings.EqualFold(FolderName(copy), live) {
					keep[dup.RomID] = copy.FilePath
				}
			}
		}
	}

	collapsed := make([]LocalSave, 0, len(local))
	for _, save := range local {
		if path, duplicated := keep[save.RomID]; duplicated && !save.IsDirectorySave && save.FilePath != path {
			continue
		}
		collapsed = append(collapsed, save)
	}
	return collapsed, duplicates
}

// withoutDuplicated drops the sync items for games whose saves are in doubt: the
// sync must neither send a copy that may be the wrong one nor overwrite either.
func withoutDuplicated(items []SyncItem, duplicates []DuplicateSaves) []SyncItem {
	if len(duplicates) == 0 {
		return items
	}
	doubtful := map[int]bool{}
	for _, dup := range duplicates {
		doubtful[dup.RomID] = true
	}
	kept := make([]SyncItem, 0, len(items))
	for _, item := range items {
		if !doubtful[item.LocalSave.RomID] {
			kept = append(kept, item)
		}
	}
	return kept
}

// ChangedSinceSync reports whether a copy differs from what was last
// downloaded from RomM or uploaded to it, which is the clue to which copy has
// been played since. ok is false when there is no record to compare with.
func ChangedSinceSync(deviceID string, copy LocalSave) (changed, ok bool) {
	recorded := loadRecordedHashes(deviceID)[saveKey{copy.RomID, copy.FileName}]
	if recorded == "" {
		return false, false
	}
	hash, err := saveContentHash(copy)
	if err != nil {
		return false, false
	}
	return hash != recorded, true
}

// KeepSave makes one copy the game's save, in destDir, and puts every other copy
// away in a .backup folder beside it. Nothing is deleted.
//
// The chosen copy is copied and checked before anything else is touched, and if
// a later step fails the earlier ones are undone, so a failure never costs the
// player a save.
func KeepSave(duplicates DuplicateSaves, keep int, destDir string) error {
	if keep < 0 || keep >= len(duplicates.Copies) {
		return fmt.Errorf("no copy %d", keep)
	}
	chosen := duplicates.Copies[keep]
	dest := filepath.Join(destDir, chosen.FileName)

	info, err := os.Stat(chosen.FilePath)
	if err != nil {
		return err
	}

	inPlace := filepath.Clean(dest) == filepath.Clean(chosen.FilePath)
	var displaced, displacedBackup string
	if !inPlace {
		if _, err := os.Stat(dest); err == nil {
			// Whatever is at the destination is another copy, or a stray; it
			// goes to the backup folder first so it can be put back.
			if displacedBackup, err = putAway(dest, filepath.Base(dest)); err != nil {
				return err
			}
			displaced = dest
		}
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
		if err := files.CopyFile(chosen.FilePath, dest); err != nil {
			undoPutAway(displaced, displacedBackup)
			return err
		}
		if copied, err := os.Stat(dest); err != nil || copied.Size() != info.Size() {
			os.Remove(dest)
			undoPutAway(displaced, displacedBackup)
			return fmt.Errorf("the copy at %s does not match the original", dest)
		}
		os.Chtimes(dest, info.ModTime(), info.ModTime())
	}

	var failed []string
	for i, copy := range duplicates.Copies {
		if i == keep && inPlace {
			continue
		}
		if filepath.Clean(copy.FilePath) == filepath.Clean(displaced) {
			continue // already put away above
		}
		if _, err := os.Stat(copy.FilePath); err != nil {
			continue
		}
		if _, err := putAway(copy.FilePath, copy.FileName); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("the save was kept but %d other copies could not be put away: %s", len(failed), strings.Join(failed, "; "))
	}
	return nil
}

// putAway moves a save into the .backup folder beside it, named the way the
// backups made before a download are, and returns where it went.
func putAway(path, fileName string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(path), ".backup")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	stamp := info.ModTime().Format("2006-01-02 15-04-05")

	target := filepath.Join(dir, fmt.Sprintf("%s [%s]%s", base, stamp, ext))
	for n := 2; ; n++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		target = filepath.Join(dir, fmt.Sprintf("%s [%s] (%d)%s", base, stamp, n, ext))
	}
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}

func undoPutAway(original, backup string) {
	if original != "" && backup != "" {
		os.Rename(backup, original)
	}
}

// RemoteSaveFor is RomM's current save for a game, in the slot the player uses
// for it, or nil when RomM has none.
func RemoteSaveFor(client *romm.Client, config *settings.Config, deviceID string, romID int) (*romm.Save, error) {
	remote, err := client.GetSaves(romm.SaveQuery{RomID: romID, DeviceID: deviceID})
	if err != nil {
		return nil, err
	}
	preferred := settings.DefaultSaveSlot
	if config != nil {
		preferred = config.GetSlotPreference(romID)
	}
	return SelectSaveForSlot(remote, preferred), nil
}

// UseRemoteSave makes RomM's save the game's save: it is downloaded into destDir,
// the way a sync would download it, and every local copy is put away in a
// .backup folder. Nothing is deleted.
//
// The download comes first and is the only thing that can fail before anything
// local is touched, and it backs up the copy it replaces, so a failed download
// leaves every local copy as it was.
func UseRemoteSave(client *romm.Client, config *settings.Config, deviceID string, duplicates DuplicateSaves, remote *romm.Save, destDir string) error {
	if remote == nil {
		return fmt.Errorf("no save to use")
	}

	fsSlug, romFileName := duplicates.FSSlug, duplicates.RomFileName
	if len(duplicates.Copies) > 0 {
		if fsSlug == "" {
			fsSlug = duplicates.Copies[0].FSSlug
		}
		if romFileName == "" {
			romFileName = duplicates.Copies[0].RomFileName
		}
	}

	// The name the emulator already uses for this game's save; a copy in the
	// folder being filled is the one the download replaces. A game with no copy
	// at all is named the way a sync would name a new one.
	target := ""
	name := ""
	if len(duplicates.Copies) > 0 {
		name = duplicates.Copies[0].FileName
		for _, copy := range duplicates.Copies {
			if filepath.Clean(filepath.Dir(copy.FilePath)) == filepath.Clean(destDir) {
				name = copy.FileName
			}
		}
		target = filepath.Join(destDir, name)
	}

	item := SyncItem{
		LocalSave: LocalSave{
			RomID:       duplicates.RomID,
			RomName:     duplicates.RomName,
			FSSlug:      fsSlug,
			FileName:    name,
			FilePath:    target,
			RomFileName: romFileName,
			EmulatorDir: destDir,
		},
		RemoteSave: remote,
		Action:     ActionDownload,
		TargetSlot: SlotName(remote.Slot),
	}
	if !download(client, config, deviceID, &item) {
		return fmt.Errorf("could not download RomM's save")
	}

	var failed []string
	for _, copy := range duplicates.Copies {
		if target != "" && filepath.Clean(copy.FilePath) == filepath.Clean(target) {
			continue // replaced by the download, which kept its own backup
		}
		if _, err := os.Stat(copy.FilePath); err != nil {
			continue
		}
		if _, err := putAway(copy.FilePath, copy.FileName); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("RomM's save is in place but %d other copies could not be put away: %s", len(failed), strings.Join(failed, "; "))
	}
	return nil
}
