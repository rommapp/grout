package ui

import (
	"testing"

	"grout/romm"
	"grout/sync"
)

// After execution, an upload that the server rejected with 409 is turned into a
// resolvable conflict (Action=Conflict, RemoteSave populated). Those — and only those
// — must be looped back to the conflict screen in the same run. Pre-existing conflicts
// the user already skipped (wasUpload=false) and conflicts with no server save (not
// resolvable) must be left alone, so the loop terminates.
func TestNewlySurfacedConflicts(t *testing.T) {
	remote := &romm.Save{ID: 75}

	items := []sync.SyncItem{
		{Action: sync.ActionDownload},                     // 0: a download, ignore
		{Action: sync.ActionConflict, RemoteSave: remote}, // 1: upload->409 conflict (resolvable)
		{Action: sync.ActionUpload},                       // 2: upload that succeeded -> still upload
		{Action: sync.ActionConflict, RemoteSave: remote}, // 3: skipped round-1 conflict (wasUpload=false)
		{Action: sync.ActionConflict, RemoteSave: nil},    // 4: upload->409 but no server save (unresolvable)
	}
	wasUpload := []bool{false, true, true, false, true}

	got := newlySurfacedConflicts(items, wasUpload)

	// Only item index 1 qualifies: was an upload, is now a conflict, has a server save.
	want := map[int]int{0: 1}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("conflictIndices[%d] = %d, want %d", k, got[k], v)
		}
	}
}

func TestNewlySurfacedConflicts_NoneWhenAllResolved(t *testing.T) {
	items := []sync.SyncItem{
		{Action: sync.ActionUpload, Success: true},
		{Action: sync.ActionDownload, Success: true},
	}
	wasUpload := []bool{true, false}

	if got := newlySurfacedConflicts(items, wasUpload); len(got) != 0 {
		t.Errorf("expected no loop-back, got %v", got)
	}
}
