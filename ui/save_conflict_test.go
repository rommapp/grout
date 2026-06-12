package ui

import (
	"testing"

	"grout/sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

func TestApplyResolutions_SkipDefaultLeavesConflictUntouched(t *testing.T) {
	// buildMenuItems localizes labels; init an empty bundle so Localize falls back to
	// the Other strings instead of panicking on a nil localizer.
	if err := i18n.InitI18NFromBytes(nil); err != nil {
		t.Fatal(err)
	}

	s := NewSaveConflictScreen()
	conflicts := []sync.SyncItem{
		{LocalSave: sync.LocalSave{RomID: 1, RomName: "A"}, Action: sync.ActionConflict},
		{LocalSave: sync.LocalSave{RomID: 2, RomName: "B"}, Action: sync.ActionConflict},
		{LocalSave: sync.LocalSave{RomID: 3, RomName: "C"}, Action: sync.ActionConflict},
	}
	menu := s.buildMenuItems(conflicts)
	// Default selection (index 0) must be "skip" for every row.
	for i, m := range menu {
		if m.Options[m.SelectedOption].Value != "skip" {
			t.Fatalf("row %d default = %v, want skip", i, m.Options[m.SelectedOption].Value)
		}
	}

	// Row 0 left on Skip (default), row 1 → Keep Local, row 2 → Keep Remote.
	menu[1].SelectedOption = indexOf(menu[1].Options, "local")
	menu[2].SelectedOption = indexOf(menu[2].Options, "remote")

	s.applyResolutions(conflicts, menu)

	if conflicts[0].Action != sync.ActionConflict || conflicts[0].ForceOverwrite {
		t.Errorf("skipped conflict must stay ActionConflict with no overwrite, got %+v", conflicts[0])
	}
	if conflicts[1].Action != sync.ActionUpload || !conflicts[1].ForceOverwrite {
		t.Errorf("keep-local must be upload+overwrite, got %+v", conflicts[1])
	}
	if conflicts[2].Action != sync.ActionDownload {
		t.Errorf("keep-remote must be download, got %v", conflicts[2].Action)
	}
}

func indexOf(opts []gaba.Option, value string) int {
	for i, o := range opts {
		if o.Value == value {
			return i
		}
	}
	return -1
}
