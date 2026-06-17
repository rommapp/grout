package sync

import (
	"os"
	"path/filepath"
	"testing"

	"grout/romm"
)

func strPtr(s string) *string { return &s }

// writeLocalSave creates a real save file on disk and returns a LocalSave pointing
// at it, so saveContentHash can hash actual bytes.
func writeLocalSave(t *testing.T, romID int, fileName, content string) LocalSave {
	t.Helper()
	path := filepath.Join(t.TempDir(), fileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write save: %v", err)
	}
	return LocalSave{RomID: romID, FileName: fileName, FilePath: path}
}

func countUploads(items []SyncItem) int {
	n := 0
	for _, it := range items {
		if it.Action == ActionUpload {
			n++
		}
	}
	return n
}

// A negotiate "upload" op for a save whose content is byte-identical to what was
// previously downloaded (recorded hash matches) must be suppressed: the content is
// already on the server (under a null/archival slot negotiate doesn't pair on), so
// re-uploading it is the spurious "uploads everything I just downloaded" churn.
func TestMapOperationsToItems_SuppressesUnchangedPromotionUpload(t *testing.T) {
	ls := writeLocalSave(t, 155, "Kirby's Pinball Land (USA, Europe).srm", "SAVE-CONTENT")
	hash, err := saveContentHash(ls)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	recordedSlots := map[saveKey]string{{155, ls.FileName}: "autosave"}
	recordedHashes := map[saveKey]string{{155, ls.FileName}: hash}

	ops := []romm.SyncOperationSchema{{
		Action:   "upload",
		RomID:    155,
		Slot:     strPtr("autosave"),
		FileName: "Kirby's Pinball Land (USA, Europe) [2026-06-12_01-31-33].srm",
	}}

	items := mapOperationsToItems(ops, []LocalSave{ls}, nil, nil, nil, recordedSlots, recordedHashes)

	if got := countUploads(items); got != 0 {
		t.Errorf("expected upload to be suppressed (content unchanged since download), got %d upload items", got)
	}
}

// Once the user actually changes the save (local hash diverges from the recorded
// download hash), the upload must go through.
func TestMapOperationsToItems_UploadsWhenContentChangedSinceDownload(t *testing.T) {
	ls := writeLocalSave(t, 155, "Kirby's Pinball Land (USA, Europe).srm", "NEW-PROGRESS")

	recordedSlots := map[saveKey]string{{155, ls.FileName}: "autosave"}
	recordedHashes := map[saveKey]string{{155, ls.FileName}: "stale-hash-from-earlier-download"}

	ops := []romm.SyncOperationSchema{{
		Action:   "upload",
		RomID:    155,
		Slot:     strPtr("autosave"),
		FileName: "Kirby's Pinball Land (USA, Europe) [2026-06-12_01-31-33].srm",
	}}

	items := mapOperationsToItems(ops, []LocalSave{ls}, nil, nil, nil, recordedSlots, recordedHashes)

	if got := countUploads(items); got != 1 {
		t.Errorf("expected 1 upload (content changed since download), got %d", got)
	}
}

// A genuinely new local save (no prior download record) must upload normally.
func TestMapOperationsToItems_UploadsNewSaveWithNoDownloadRecord(t *testing.T) {
	ls := writeLocalSave(t, 200, "Tecmo Super Bowl 2025.srm", "FRESH-SAVE")

	recordedSlots := map[saveKey]string{{200, ls.FileName}: "autosave"}
	recordedHashes := map[saveKey]string{} // never downloaded

	ops := []romm.SyncOperationSchema{{
		Action:   "upload",
		RomID:    200,
		Slot:     strPtr("autosave"),
		FileName: "Tecmo Super Bowl 2025.srm",
	}}

	items := mapOperationsToItems(ops, []LocalSave{ls}, nil, nil, nil, recordedSlots, recordedHashes)

	if got := countUploads(items); got != 1 {
		t.Errorf("expected 1 upload (new save, no download record), got %d", got)
	}
}
