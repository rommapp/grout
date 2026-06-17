package sync

import "testing"

// A downloaded save's save-state record is keyed on file_name (PK is
// device_id+rom_id+file_name). The next ScanSaves finds the file under its plain
// on-disk basename, so the record MUST be keyed on that same basename — not the
// server's datetime-tagged op.FileName, which would never be looked up again and
// leaves the save reporting the fallback "autosave" slot (the re-upload churn).
func TestRecordedDownloadFileName(t *testing.T) {
	tests := []struct {
		name     string
		item     SyncItem
		savePath string
		want     string
	}{
		{
			name: "file save uses the on-disk basename, not the tagged server name",
			item: SyncItem{LocalSave: LocalSave{
				FileName:        "Dragon Ball Z - Buu's Fury [2026-03-28_17-15-06].sav",
				IsDirectorySave: false,
			}},
			savePath: "/saves/mGBA/Dragon Ball Z - Buu's Fury.sav",
			want:     "Dragon Ball Z - Buu's Fury.sav",
		},
		{
			name: "file save with an already-plain local name still uses the on-disk basename",
			item: SyncItem{LocalSave: LocalSave{
				FileName:        "Chrono Trigger (USA).srm",
				IsDirectorySave: false,
			}},
			savePath: "/saves/Snes9x/Chrono Trigger (USA).srm",
			want:     "Chrono Trigger (USA).srm",
		},
		{
			// PSP-style directory saves are scanned as gameID + ".zip" (see ScanSaves),
			// not as a file basename, so the record must match that.
			name: "directory save uses gameID.zip to match ScanSaves",
			item: SyncItem{LocalSave: LocalSave{
				IsDirectorySave: true,
				GameID:          "UCUS98751",
				FileName:        "UCUS98751.zip",
			}},
			savePath: "/saves/SAVEDATA/UCUS98751",
			want:     "UCUS98751.zip",
		},
		{
			name: "directory save without a gameID falls back to the local file name",
			item: SyncItem{LocalSave: LocalSave{
				IsDirectorySave: true,
				FileName:        "grout-save-1020011912.zip",
			}},
			savePath: "/saves/SAVEDATA/grout-save-1020011912",
			want:     "grout-save-1020011912.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recordedDownloadFileName(tt.item, tt.savePath); got != tt.want {
				t.Errorf("recordedDownloadFileName() = %q, want %q", got, tt.want)
			}
		})
	}
}
