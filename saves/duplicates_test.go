package saves

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The case from the device: the same game's save in gpSP's folder and in mGBA's.
func TestAGameWithASaveInTwoFoldersIsFound(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Pokemon Monomyth v1_0.srm", "latest from RomM", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Pokemon Monomyth v1_0.srm", "old", yesterday)
	other := copyIn(t, root, 12, "gpSP", "Other.srm", "x", yesterday)

	got := FindDuplicateSaves([]LocalSave{gpsp, mgba, other})

	if len(got) != 1 || got[0].RomID != 4679 || len(got[0].Copies) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestSeveralSavesInOneFolderAreNotTheSameProblem(t *testing.T) {
	root := t.TempDir()
	a := copyIn(t, root, 1, "gpSP", "Game.srm", "a", yesterday)
	b := copyIn(t, root, 1, "gpSP", "Game.sav", "b", yesterday)

	if got := FindDuplicateSaves([]LocalSave{a, b}); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestCollapsingKeepsTheCopyInTheFolderTheGamesCoreReads(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "a", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "b", yesterday)
	other := copyIn(t, root, 12, "gpSP", "Other.srm", "x", yesterday)

	kept, dups := collapseDuplicates([]LocalSave{gpsp, mgba, other}, func(int) string { return "mGBA" })

	if len(dups) != 1 {
		t.Fatalf("dups = %+v", dups)
	}
	if len(kept) != 2 || kept[0].FilePath != mgba.FilePath || kept[1].FilePath != other.FilePath {
		t.Errorf("kept %+v", kept)
	}
}

func TestCollapsingWithoutKnowingTheCoreStillLeavesOneCopy(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "a", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "b", yesterday)

	kept, _ := collapseDuplicates([]LocalSave{gpsp, mgba}, func(int) string { return "" })

	if len(kept) != 1 {
		t.Errorf("kept %d copies", len(kept))
	}
}

func TestNothingIsDoneForAGameWhoseSaveIsInDoubt(t *testing.T) {
	items := []SyncItem{
		{LocalSave: LocalSave{RomID: 4679}, Action: ActionDownload},
		{LocalSave: LocalSave{RomID: 4679}, Action: ActionUpload},
		{LocalSave: LocalSave{RomID: 12}, Action: ActionDownload},
	}

	got := withoutDuplicated(items, []DuplicateSaves{{RomID: 4679}})

	if len(got) != 1 || got[0].LocalSave.RomID != 12 {
		t.Errorf("got %+v", got)
	}
}

func TestKeepingTheCopyFromAnotherFolderPutsItWhereTheCoreReadsAndBacksUpTheRest(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "latest from RomM", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "old", yesterday.Add(-48*time.Hour))
	dup := DuplicateSaves{RomID: 4679, Copies: []LocalSave{gpsp, mgba}}

	if err := KeepSave(dup, 0, filepath.Join(root, "mGBA")); err != nil {
		t.Fatal(err)
	}

	if data, _ := os.ReadFile(filepath.Join(root, "mGBA", "Game.srm")); string(data) != "latest from RomM" {
		t.Errorf("mGBA's folder holds %q", data)
	}
	if _, err := os.Stat(gpsp.FilePath); !os.IsNotExist(err) {
		t.Error("the other copy is still where Grout would find it")
	}
	oldBackup := filepath.Join(root, "mGBA", ".backup", "Game [2026-11-07 20-15-30].srm")
	if data, _ := os.ReadFile(oldBackup); string(data) != "old" {
		t.Errorf("the save that was replaced is not kept at %s", oldBackup)
	}
	keptBackup := filepath.Join(root, "gpSP", ".backup", "Game [2026-11-09 20-15-30].srm")
	if data, _ := os.ReadFile(keptBackup); string(data) != "latest from RomM" {
		t.Errorf("the original of the chosen copy is not kept at %s", keptBackup)
	}
	if info, _ := os.Stat(filepath.Join(root, "mGBA", "Game.srm")); !info.ModTime().Equal(yesterday) {
		t.Errorf("the chosen save lost its time: %v", info.ModTime())
	}
	if remaining := FindDuplicateSaves(LocalSavesIn(t, root)); len(remaining) != 0 {
		t.Errorf("still duplicated: %+v", remaining)
	}
}

func TestKeepingTheCopyAlreadyInPlaceOnlyPutsTheOthersAway(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "stale", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "current", yesterday)
	dup := DuplicateSaves{RomID: 4679, Copies: []LocalSave{gpsp, mgba}}

	if err := KeepSave(dup, 1, filepath.Join(root, "mGBA")); err != nil {
		t.Fatal(err)
	}

	if data, _ := os.ReadFile(mgba.FilePath); string(data) != "current" {
		t.Errorf("the chosen save was changed to %q", data)
	}
	if _, err := os.Stat(gpsp.FilePath); !os.IsNotExist(err) {
		t.Error("the other copy was left in place")
	}
	if _, err := os.Stat(filepath.Join(root, "gpSP", ".backup", "Game [2026-11-09 20-15-30].srm")); err != nil {
		t.Error("the other copy was not kept in a backup")
	}
}

func TestBackupsOfSavesWithTheSameTimeDoNotOverwriteEachOther(t *testing.T) {
	root := t.TempDir()
	first := copyIn(t, root, 1, "gpSP", "Game.srm", "first", yesterday)
	if _, err := putAway(first.FilePath, "Game.srm"); err != nil {
		t.Fatal(err)
	}
	second := copyIn(t, root, 1, "gpSP", "Game.srm", "second", yesterday)
	if _, err := putAway(second.FilePath, "Game.srm"); err != nil {
		t.Fatal(err)
	}

	backups, _ := filepath.Glob(filepath.Join(root, "gpSP", ".backup", "*"))
	if len(backups) != 2 {
		t.Errorf("%d backups kept, want 2: %v", len(backups), backups)
	}
}

func TestKeepSaveRefusesACopyThatIsNotThere(t *testing.T) {
	if err := KeepSave(DuplicateSaves{Copies: []LocalSave{{FilePath: "/nope/Game.srm"}}}, 3, t.TempDir()); err == nil {
		t.Error("accepted a copy that does not exist")
	}
}

// LocalSavesIn lists the save files left in the live folders of a test card.
func LocalSavesIn(t *testing.T, root string) []LocalSave {
	t.Helper()
	var found []LocalSave
	for _, folder := range []string{"gpSP", "mGBA"} {
		entries, _ := os.ReadDir(filepath.Join(root, folder))
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			found = append(found, LocalSave{RomID: 4679, FileName: e.Name(), FilePath: filepath.Join(root, folder, e.Name())})
		}
	}
	return found
}

func TestExtraCopiesIdenticalToTheLiveOneAreTidiedAway(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 291, "gpSP", "Advance Wars.srm", "same save", yesterday)
	mgba := copyIn(t, root, 291, "mGBA", "Advance Wars.srm", "same save", yesterday)
	other := copyIn(t, root, 12, "gpSP", "Other.srm", "x", yesterday)

	kept, tidied := tidyIdenticalCopies([]LocalSave{gpsp, mgba, other}, func(int) string { return "gpSP" })

	if tidied != 1 {
		t.Fatalf("tidied %d copies, want 1", tidied)
	}
	if len(kept) != 2 || kept[0].FilePath != gpsp.FilePath || kept[1].FilePath != other.FilePath {
		t.Errorf("kept %+v", kept)
	}
	if _, err := os.Stat(mgba.FilePath); !os.IsNotExist(err) {
		t.Error("the extra copy is still where Grout would find it")
	}
	if data, _ := os.ReadFile(filepath.Join(root, "mGBA", ".backup", "Advance Wars [2026-11-09 20-15-30].srm")); string(data) != "same save" {
		t.Error("the extra copy was not kept in a backup")
	}
	if data, _ := os.ReadFile(gpsp.FilePath); string(data) != "same save" {
		t.Error("the live copy was touched")
	}
}

func TestCopiesThatDifferAreLeftForThePlayer(t *testing.T) {
	root := t.TempDir()
	gpsp := copyIn(t, root, 4679, "gpSP", "Game.srm", "latest from RomM", yesterday)
	mgba := copyIn(t, root, 4679, "mGBA", "Game.srm", "an older one", yesterday)

	kept, tidied := tidyIdenticalCopies([]LocalSave{gpsp, mgba}, func(int) string { return "mGBA" })

	if tidied != 0 || len(kept) != 2 {
		t.Errorf("tidied %d, kept %d: a game whose saves differ must be left alone", tidied, len(kept))
	}
	for _, path := range []string{gpsp.FilePath, mgba.FilePath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was moved", path)
		}
	}
}

func TestNothingIsTidiedWhenTheLiveFolderIsNotKnown(t *testing.T) {
	root := t.TempDir()
	a := copyIn(t, root, 1, "gpSP", "Game.srm", "same", yesterday)
	b := copyIn(t, root, 1, "mGBA", "Game.srm", "same", yesterday)

	if _, tidied := tidyIdenticalCopies([]LocalSave{a, b}, func(int) string { return "" }); tidied != 0 {
		t.Errorf("tidied %d copies without knowing which one is in use", tidied)
	}
	if _, tidied := tidyIdenticalCopies([]LocalSave{a, b}, func(int) string { return "Snes9x" }); tidied != 0 {
		t.Errorf("tidied %d copies although none is in the live folder", tidied)
	}
}

func TestThreeCopiesAreOnlyTidiedIfAllThreeAreTheSame(t *testing.T) {
	root := t.TempDir()
	a := copyIn(t, root, 373, "gpSP", "Metroid Fusion.srm", "same", yesterday)
	b := copyIn(t, root, 373, "mGBA", "Metroid Fusion.srm", "same", yesterday)
	c := copyIn(t, root, 373, "VBA", "Metroid Fusion.srm", "DIFFERENT", yesterday)

	if _, tidied := tidyIdenticalCopies([]LocalSave{a, b, c}, func(int) string { return "gpSP" }); tidied != 0 {
		t.Errorf("tidied %d copies although one of them differs", tidied)
	}
	if _, err := os.Stat(b.FilePath); err != nil {
		t.Error("an identical copy was moved although another copy differs")
	}
}
