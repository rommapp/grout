package saves

import (
	"fmt"

	"grout/romm"
	"grout/settings"
)

// NewestRemoteSave is the most recently updated save RomM holds for a game, in
// any slot, or nil when it has none. It is what "RomM's latest" means to a
// player, whichever slot it was saved in.
func NewestRemoteSave(client *romm.Client, deviceID string, romID int) (*romm.Save, error) {
	remote, err := client.GetSaves(romm.SaveQuery{RomID: romID, DeviceID: deviceID})
	if err != nil {
		return nil, err
	}
	var newest *romm.Save
	for i := range remote {
		if newest == nil || remote[i].UpdatedAt.After(newest.UpdatedAt) {
			newest = &remote[i]
		}
	}
	return newest, nil
}

// SlotOf is the slot RomM's save is in.
func SlotOf(save *romm.Save) string { return SlotName(save.Slot) }

// ForceUpload sends the device's save to RomM as the game's current save, in the
// slot the game is synced under, whatever RomM holds and whichever is newer. It
// is the player's decision, so the usual check that RomM has not moved on is
// switched off.
func ForceUpload(client *romm.Client, config *settings.Config, deviceID string, local LocalSave) error {
	item := SyncItem{
		LocalSave:      local,
		Action:         ActionUpload,
		ForceOverwrite: true,
		TargetSlot:     resolveReportedSlot(local, config, loadRecordedSlots(deviceID)),
	}
	switch upload(client, deviceID, &item) {
	case uploadOK:
		return nil
	case uploadConflict, uploadSupersededByDownload:
		return fmt.Errorf("RomM would not take the save over the one it has")
	default:
		return fmt.Errorf("could not upload the save")
	}
}

// FollowSlot makes the game sync under the slot of a save that was just taken
// from RomM. Without it a game that has an explicit slot choice would be paired
// with its old slot on the next sync and the save just taken would look like
// something to change back.
func FollowSlot(config *settings.Config, romID int, taken *romm.Save) bool {
	slot := SlotOf(taken)
	if pref, explicit := config.SlotPreferenceExplicit(romID); explicit && pref != slot {
		config.SetSlotPreference(romID, slot)
		return true
	}
	return false
}
