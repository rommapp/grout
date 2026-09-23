package ui

import (
	"testing"

	"grout/settings"
)

// restoreKidMode puts the session's kid mode back, since it is package state
// the rest of the tests also read.
func restoreKidMode(t *testing.T) {
	t.Helper()
	was := settings.IsKidModeEnabled()
	t.Cleanup(func() { settings.SetKidMode(was) })
}

// Kid mode can be unlocked for one run without changing what is stored. Just
// visiting this screen and saving must not undo that: the parent who unlocked
// it would find the settings button gone again with no sign why.
func TestApplyKidMode_LeavesAnUnlockedSessionAlone(t *testing.T) {
	restoreKidMode(t)

	stored := settings.Config{KidMode: true}
	settings.InitKidMode(&stored)

	// The unlock chord clears the session without touching the config.
	settings.SetKidMode(false)

	applyKidMode(stored, stored)

	if settings.IsKidModeEnabled() {
		t.Error("saving without changing kid mode re-locked the session")
	}
}

func TestApplyKidMode_FollowsAnActualChange(t *testing.T) {
	restoreKidMode(t)

	before := settings.Config{KidMode: true}
	settings.InitKidMode(&before)

	after := settings.Config{KidMode: false}
	applyKidMode(before, after)

	if settings.IsKidModeEnabled() {
		t.Error("turning kid mode off did not unlock the session")
	}

	applyKidMode(after, before)
	if !settings.IsKidModeEnabled() {
		t.Error("turning kid mode on did not lock the session")
	}
}

func TestToolsRows_RoundTrips(t *testing.T) {
	before := settings.Config{KidMode: true}
	after := before

	rows := toolsRows()
	applySettingRows(rows, &after, settingItems(rows, before))

	if after.KidMode != before.KidMode {
		t.Errorf("KidMode %v -> %v just by opening and saving", before.KidMode, after.KidMode)
	}
}

// The row shows what is stored rather than the session's value, or a parent
// who unlocked for this run would see kid mode as already off and have no way
// to actually turn it off.
func TestToolsRows_KidModeShowsTheStoredSetting(t *testing.T) {
	restoreKidMode(t)

	stored := settings.Config{KidMode: true}
	settings.InitKidMode(&stored)
	settings.SetKidMode(false)

	rows := toolsRows()
	items := settingItems(rows, stored)

	for i, row := range rows {
		if row.key != "kid_mode" {
			continue
		}
		if value, _ := items[i].Options[items[i].SelectedOption].Value.(bool); !value {
			t.Error("the row opened on Disabled, but kid mode is stored as on")
		}
		return
	}
	t.Fatal("no kid mode row")
}

func TestToolsRows_EveryRowIsValueOrNavigation(t *testing.T) {
	for _, row := range toolsRows() {
		_, leads := toolsDestinations[row.key]

		if row.leadsSomewhere() && !leads {
			t.Errorf("%q navigates but has no destination, so pressing it does nothing", row.key)
		}
		if !row.leadsSomewhere() && leads {
			t.Errorf("%q holds a value but is also listed as going somewhere", row.key)
		}
	}
}
