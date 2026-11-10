package saves

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"grout/romm"
)

var (
	romMTime   = time.Date(2026, 11, 10, 7, 55, 11, 0, time.UTC)
	deviceTime = time.Date(2026, 11, 9, 20, 15, 30, 0, time.UTC)
)

func str(s string) *string { return &s }

func hashOf(t *testing.T, content string) (LocalSave, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Game.srm")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	local := LocalSave{RomID: 1, FileName: "Game.srm", FilePath: path}
	hash, err := saveContentHash(local)
	if err != nil {
		t.Fatal(err)
	}
	return local, hash
}

func mustSay(t *testing.T, o Outcome, kind OutcomeKind, phrases ...string) {
	t.Helper()
	if o.Kind != kind {
		t.Errorf("kind = %v, want %v\n%s", o.Kind, kind, o.Text())
	}
	for _, phrase := range phrases {
		if !strings.Contains(o.Text(), phrase) {
			t.Errorf("message does not say %q:\n%s", phrase, o.Text())
		}
	}
}

func TestSayingRomMsSaveWasTakenAndWhatItReplaced(t *testing.T) {
	got := DescribeSync(SyncFacts{
		Report: SyncReport{Downloaded: 1, Items: []SyncItem{{
			Action: ActionDownload, Success: true,
			RemoteSave: &romm.Save{UpdatedAt: romMTime},
		}}},
		LocalWrote:    deviceTime,
		StateSetAside: true,
	})

	mustSay(t, got, OutcomeDownloaded, "Took RomM's save", when(romMTime), "Replaced this device's save", when(deviceTime), "backed up", "resume state")
	if strings.Contains(got.Text(), "up to date") || strings.Contains(got.Text(), "in sync") {
		t.Errorf("a download was described as being in sync:\n%s", got.Text())
	}
}

func TestSayingTheDeviceHadNoSaveBeforeADownload(t *testing.T) {
	got := DescribeSync(SyncFacts{Report: SyncReport{Downloaded: 1, Items: []SyncItem{{
		Action: ActionDownload, Success: true, RemoteSave: &romm.Save{UpdatedAt: romMTime}}}}})

	mustSay(t, got, OutcomeDownloaded, "no save yet")
}

func TestSayingTheDevicesSaveWasSentAndWhatRomMHad(t *testing.T) {
	got := DescribeSync(SyncFacts{
		Report:     SyncReport{Uploaded: 1},
		LocalWrote: deviceTime,
		Ops:        []romm.SyncOperationSchema{{Action: "upload", ServerUpdatedAt: &romMTime}},
	})

	mustSay(t, got, OutcomeUploaded, "Sent this device's save", "This device: "+when(deviceTime), "RomM had: "+when(romMTime))
}

func TestInSyncIsOnlySaidWhenRomMsHashMatches(t *testing.T) {
	local, hash := hashOf(t, "progress")

	same := DescribeSync(SyncFacts{
		Local: []LocalSave{local},
		Ops:   []romm.SyncOperationSchema{{Action: "no_op", ServerUpdatedAt: &romMTime, ServerContentHash: &hash}},
	})
	mustSay(t, same, OutcomeInSync, "Already in sync", "same save", when(romMTime))

	other := "0000"
	differs := DescribeSync(SyncFacts{
		Local: []LocalSave{local},
		Ops:   []romm.SyncOperationSchema{{Action: "no_op", ServerContentHash: &other}},
	})
	mustSay(t, differs, OutcomeLeftAlone, "saves differ")
	if strings.Contains(differs.Text(), "in sync") {
		t.Errorf("said in sync although the hashes differ:\n%s", differs.Text())
	}
}

// What the player reported: nothing was done, and "up to date" was said.
func TestNothingDoneBecauseRomMsNewestSaveIsInAnotherSlotIsNotUpToDate(t *testing.T) {
	got := DescribeSync(SyncFacts{
		Ops: []romm.SyncOperationSchema{{Action: "download", Slot: str("boss"), ServerUpdatedAt: &romMTime}},
		Dropped: []DroppedOp{{
			Op:        romm.SyncOperationSchema{Action: "download", Slot: str("boss"), ServerUpdatedAt: &romMTime},
			Reason:    DropOtherSlot,
			LocalSlot: "autosave",
		}},
	})

	mustSay(t, got, OutcomeOtherSlot, "Nothing changed", "slot 'boss'", when(romMTime), "uses 'autosave'", "Save Slot", "Use RomM's Save")
	if strings.Contains(got.Text(), "up to date") {
		t.Errorf("called an unsynced game up to date:\n%s", got.Text())
	}
}

func TestAnUploadLeftBecauseTheSaveIsWhatWasLastDownloadedSaysSoAndHowToForceIt(t *testing.T) {
	got := DescribeSync(SyncFacts{Dropped: []DroppedOp{{Reason: DropUnchangedSinceDownload}}})

	mustSay(t, got, OutcomeLeftAlone, "asked for an upload", "last downloaded", "Upload Local")
}

func TestAConflictOrSlotChoiceIsSaidToNeedThePlayer(t *testing.T) {
	mustSay(t, DescribeSync(SyncFacts{Held: 1}), OutcomeNeedsYou, "needs you", "Grout > Sync")
}

func TestAFailureIsSaidToBeAFailureWhateverElseHappened(t *testing.T) {
	got := DescribeSync(SyncFacts{Report: SyncReport{Errors: 2, Uploaded: 1}})

	mustSay(t, got, OutcomeFailed, "failed", "2 step(s)", "logs")
}

func TestNoInstructionsIsNotAClaimThatTheSavesMatch(t *testing.T) {
	local, _ := hashOf(t, "x")

	mustSay(t, DescribeSync(SyncFacts{Local: []LocalSave{local}}), OutcomeNoAnswer, "no instructions")
	mustSay(t, DescribeSync(SyncFacts{}), OutcomeNothingYet, "No save for this game yet")
}

func TestNoteOnlyOtherSlotsNameTheSlot(t *testing.T) {
	if got := slotNote(nil); got != "" {
		t.Errorf("default slot noted: %q", got)
	}
	if got := slotNote(str("boss")); got != ", slot 'boss'" {
		t.Errorf("named slot: %q", got)
	}
}
