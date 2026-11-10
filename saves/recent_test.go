package saves

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var noon = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func newRecent(t *testing.T) *RecentSyncs {
	t.Helper()
	return loadRecentSyncsFrom(filepath.Join(t.TempDir(), "recent_syncs.json"))
}

func TestAGameSyncedMomentsAgoCountsAsRecent(t *testing.T) {
	recent := newRecent(t)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon)

	age, ok := recent.SyncedRecently("/roms/GBA/Monomyth.gba", noon.Add(3*time.Minute))
	if !ok || age != 3*time.Minute {
		t.Errorf("age=%v ok=%v", age, ok)
	}
}

func TestItStopsCountingOnceTheWindowHasPassed(t *testing.T) {
	recent := newRecent(t)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon)

	if _, ok := recent.SyncedRecently("/roms/GBA/Monomyth.gba", noon.Add(RecentSyncWindow)); ok {
		t.Error("still recent exactly at the end of the window")
	}
	if _, ok := recent.SyncedRecently("/roms/GBA/Monomyth.gba", noon.Add(RecentSyncWindow-time.Second)); !ok {
		t.Error("not recent just inside the window")
	}
}

func TestOneGamesSyncSaysNothingAboutAnother(t *testing.T) {
	recent := newRecent(t)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon)

	if _, ok := recent.SyncedRecently("/roms/GBA/Other.gba", noon.Add(time.Minute)); ok {
		t.Error("another game counted as synced")
	}
}

func TestASyncOfEverythingCoversEveryGame(t *testing.T) {
	recent := newRecent(t)
	recent.RecordAll(noon)

	if _, ok := recent.SyncedRecently("/roms/GB/Anything.gb", noon.Add(time.Minute)); !ok {
		t.Error("a game was not covered by the sync of everything")
	}
}

func TestTheNewerOfTheTwoRecordsWins(t *testing.T) {
	recent := newRecent(t)
	recent.RecordAll(noon)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon.Add(30*time.Minute))

	if age, ok := recent.SyncedRecently("/roms/GBA/Monomyth.gba", noon.Add(31*time.Minute)); !ok || age != time.Minute {
		t.Errorf("age=%v ok=%v", age, ok)
	}
}

func TestARecordFromTheFutureIsNotTrusted(t *testing.T) {
	recent := newRecent(t)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon.Add(2*time.Hour))

	if _, ok := recent.SyncedRecently("/roms/GBA/Monomyth.gba", noon); ok {
		t.Error("trusted a sync that has not happened yet by this clock")
	}
}

func TestADifferentSpellingOfThePathIsTheSameGame(t *testing.T) {
	recent := newRecent(t)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon)

	if _, ok := recent.SyncedRecently("/roms/GBA/../GBA/Monomyth.gba", noon.Add(time.Minute)); !ok {
		t.Error("an unclean spelling of the same path was not recognised")
	}
}

func TestTheRecordSurvivesBeingSavedAndLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent_syncs.json")
	recent := loadRecentSyncsFrom(path)
	recent.RecordRom("/roms/GBA/Monomyth.gba", noon)
	if err := recent.Save(noon); err != nil {
		t.Fatal(err)
	}

	again := loadRecentSyncsFrom(path)
	if _, ok := again.SyncedRecently("/roms/GBA/Monomyth.gba", noon.Add(time.Minute)); !ok {
		t.Error("the record was lost")
	}
}

func TestOldRecordsAreDroppedWhenSaving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent_syncs.json")
	recent := loadRecentSyncsFrom(path)
	recent.RecordRom("/roms/old.gb", noon.Add(-48*time.Hour))
	recent.RecordRom("/roms/new.gb", noon.Add(-time.Hour))
	recent.RecordAll(noon.Add(-72 * time.Hour))
	recent.Save(noon)

	again := loadRecentSyncsFrom(path)
	if len(again.data.Roms) != 1 || again.data.All != 0 {
		t.Errorf("kept %+v", again.data)
	}
}

func TestAMissingOrDamagedRecordIsJustEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, ok := loadRecentSyncsFrom(filepath.Join(dir, "missing.json")).SyncedRecently("/r.gb", noon); ok {
		t.Error("a missing record counted a game as synced")
	}

	damaged := filepath.Join(dir, "damaged.json")
	os.WriteFile(damaged, []byte("{not json"), 0644)
	recent := loadRecentSyncsFrom(damaged)
	if _, ok := recent.SyncedRecently("/r.gb", noon); ok {
		t.Error("a damaged record counted a game as synced")
	}
	recent.RecordRom("/r.gb", noon)
	if err := recent.Save(noon); err != nil {
		t.Errorf("could not write over a damaged record: %v", err)
	}
}
