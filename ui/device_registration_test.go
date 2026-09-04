package ui

import (
	"testing"

	"grout/settings"
)

func TestSaveSyncRows_KeysAreUniqueAndPresent(t *testing.T) {
	seen := make(map[string]bool)
	for _, row := range saveSyncRows(settings.Host{}) {
		if row.key == "" {
			t.Errorf("%q has no key", row.label)
		}
		if seen[row.key] {
			t.Errorf("two rows share the key %q", row.key)
		}
		seen[row.key] = true
	}
}

// A row that navigates needs somewhere to go, and one that holds a value must
// not also be treated as navigation. Device name is neither: the screen
// handles it itself, so it is deliberately absent from the destinations.
func TestSaveSyncRows_ValueAndNavigationRowsAreDistinct(t *testing.T) {
	for _, row := range saveSyncRows(settings.Host{}) {
		_, leads := saveSyncDestinations[row.key]

		if !row.leadsSomewhere() && leads {
			t.Errorf("%q holds a value but is also listed as going somewhere", row.key)
		}
	}
}

// The row shows the name the device is registered under, so the user can see
// what they are about to change.
func TestSaveSyncRows_DeviceNameIsShown(t *testing.T) {
	rows := saveSyncRows(settings.Host{DeviceName: "Living Room RG35XX"})
	items := settingItems(rows, settings.Config{})

	for i, row := range rows {
		if row.key != "device_name" {
			continue
		}
		if got := items[i].Options[0].DisplayName; got != "Living Room RG35XX" {
			t.Errorf("row shows %q, want the registered device name", got)
		}
		return
	}
	t.Fatal("no device name row")
}

// Changing the backup limit has to survive the screen. It used to be written
// into the caller's own config and then compared against that same config to
// decide whether to save, which can never be true, so the setting was lost on
// the next launch.
func TestSaveSyncRows_BackupLimitRoundTrips(t *testing.T) {
	for _, limit := range []int{5, 10, 15, 0} {
		before := settings.Config{SaveBackupLimit: limit}
		after := before

		rows := saveSyncRows(settings.Host{})
		applySettingRows(rows, &after, settingItems(rows, before))

		if after.SaveBackupLimit != limit {
			t.Errorf("limit %d came back as %d", limit, after.SaveBackupLimit)
		}
	}
}

// Zero means no limit, which is also what an unset config holds, so the row
// has to show that rather than fall through to the first entry.
func TestSaveSyncRows_UnsetLimitShowsNoLimit(t *testing.T) {
	rows := saveSyncRows(settings.Host{})
	items := settingItems(rows, settings.Config{})

	for i, row := range rows {
		if row.key != "backup_limit" {
			continue
		}
		if got := items[i].Options[items[i].SelectedOption].Value; got != 0 {
			t.Errorf("an unset limit opened on %v, want No Limit", got)
		}
		return
	}
	t.Fatal("no backup limit row")
}
