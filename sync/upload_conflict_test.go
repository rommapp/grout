package sync

import (
	"testing"

	"grout/romm"
)

// When an upload is rejected with 409 ("slot has a newer save since your last sync"),
// grout reconciles by fetching the server's current save for the slot and deciding:
//   - the local save is unmodified since its last sync (localHash == recordedHash) →
//     the server is simply ahead, auto-download it (option b);
//   - the local save diverged → a real conflict to surface (with the server save);
//   - no server save for the slot → can't reconcile, leave it as a bare conflict.
func TestResolveUpload409(t *testing.T) {
	slot := "autosave"
	server := romm.Save{ID: 75, RomID: 6, Slot: &slot}
	serverSaves := []romm.Save{server}

	tests := []struct {
		name         string
		serverSaves  []romm.Save
		localHash    string
		recordedHash string
		wantRes      conflict409Resolution
		wantSaveID   int // 0 means expect nil save
	}{
		{
			name:         "local unmodified since last sync auto-downloads the server save",
			serverSaves:  serverSaves,
			localHash:    "hash-A",
			recordedHash: "hash-A",
			wantRes:      resolve409AsDownload,
			wantSaveID:   75,
		},
		{
			name:         "local diverged from last sync surfaces a resolvable conflict",
			serverSaves:  serverSaves,
			localHash:    "hash-B",
			recordedHash: "hash-A",
			wantRes:      resolve409AsConflict,
			wantSaveID:   75,
		},
		{
			name:         "no recorded hash cannot prove unmodified so treat as conflict",
			serverSaves:  serverSaves,
			localHash:    "hash-A",
			recordedHash: "",
			wantRes:      resolve409AsConflict,
			wantSaveID:   75,
		},
		{
			name:         "no server save for the slot leaves it unresolved",
			serverSaves:  nil,
			localHash:    "hash-A",
			recordedHash: "hash-A",
			wantRes:      resolve409Unresolved,
			wantSaveID:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, save := resolveUpload409(tt.serverSaves, slot, tt.localHash, tt.recordedHash)
			if res != tt.wantRes {
				t.Errorf("resolution = %d, want %d", res, tt.wantRes)
			}
			gotID := 0
			if save != nil {
				gotID = save.ID
			}
			if gotID != tt.wantSaveID {
				t.Errorf("server save ID = %d, want %d", gotID, tt.wantSaveID)
			}
		})
	}
}
