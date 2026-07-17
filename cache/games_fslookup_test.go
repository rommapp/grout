package cache

import (
	"database/sql"
	"path/filepath"
	"testing"

	"grout/romm"

	_ "modernc.org/sqlite"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := createTables(db); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &Manager{db: db, initialized: true, stats: &Stats{}}
}

// Issue #242: RomM stores the ROM as a folder (fs_name = clean folder name)
// containing one tagged file. The downloaded ROM and its emulator save are named
// after the file. Resolution must match on the on-disk file basename, not the
// folder-derived fs_name_no_ext.
func TestGetRomByFSLookup_NestedSingleFileMatchesOnDiskName(t *testing.T) {
	cm := newTestManager(t)
	rom := romm.Rom{
		ID:               7972,
		PlatformID:       5,
		PlatformFSSlug:   "gba",
		Name:             "Kingdom Hearts: Chain of Memories",
		FsName:           "Kingdom Hearts - Chain of Memories",
		FsNameNoExt:      "Kingdom Hearts - Chain of Memories",
		HasMultipleFiles: false,
		Files:            []romm.RomFile{{FileName: "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It).gba"}},
	}
	if err := cm.SavePlatformGames(5, []romm.Rom{rom}); err != nil {
		t.Fatalf("save games: %v", err)
	}

	got, err := cm.GetRomByFSLookup("gba", "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It)")
	if err != nil {
		t.Fatalf("expected to resolve by on-disk file name, got error: %v", err)
	}
	if got.ID != 7972 {
		t.Errorf("got rom ID %d, want 7972", got.ID)
	}
}

// Issue #242 (remaining): a RomM game can bundle multiple alternative files (regions/
// revisions) under one entry; the user downloads ONE via the file picker, and its save is
// named after that file. Resolution must match ANY of the game's files, not only Files[0],
// or saves for the non-first version never sync.
func TestGetRomByFSLookup_MultiFileMatchesAnyVersion(t *testing.T) {
	cm := newTestManager(t)
	rom := romm.Rom{
		ID:               7972,
		PlatformID:       5,
		PlatformFSSlug:   "gba",
		Name:             "Kingdom Hearts: Chain of Memories",
		FsName:           "Kingdom Hearts - Chain of Memories",
		FsNameNoExt:      "Kingdom Hearts - Chain of Memories",
		HasMultipleFiles: false,
		Files: []romm.RomFile{
			{FileName: "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It).gba"},
			{FileName: "Kingdom Hearts - Chain of Memories (USA).gba"},
		},
	}
	if err := cm.SavePlatformGames(5, []romm.Rom{rom}); err != nil {
		t.Fatalf("save games: %v", err)
	}

	// The first file resolves (it always did)...
	if got, err := cm.GetRomByFSLookup("gba", "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It)"); err != nil || got.ID != 7972 {
		t.Fatalf("Europe version: got (%d, %v), want (7972, nil)", got.ID, err)
	}
	// ...and so must the non-first file the user actually downloaded.
	got, err := cm.GetRomByFSLookup("gba", "Kingdom Hearts - Chain of Memories (USA)")
	if err != nil {
		t.Fatalf("USA version should resolve, got error: %v", err)
	}
	if got.ID != 7972 {
		t.Errorf("USA version: got rom ID %d, want 7972", got.ID)
	}
}

// The v13 migration must reconstruct game_basenames from the already-cached data_json, so
// existing users get multi-file matching WITHOUT a disruptive library re-download (#242).
func TestBackfillGameBasenames_RebuildsFromDataJSON(t *testing.T) {
	cm := newTestManager(t)
	rom := romm.Rom{
		ID:             7972,
		PlatformID:     5,
		PlatformFSSlug: "gba",
		Name:           "Kingdom Hearts: Chain of Memories",
		FsNameNoExt:    "Kingdom Hearts - Chain of Memories",
		Files: []romm.RomFile{
			{FileName: "Kingdom Hearts - Chain of Memories (Europe) (En,Fr,De,Es,It).gba"},
			{FileName: "Kingdom Hearts - Chain of Memories (USA).gba"},
		},
	}
	if err := cm.SavePlatformGames(5, []romm.Rom{rom}); err != nil {
		t.Fatalf("save games: %v", err)
	}
	// Simulate a pre-v13 database: games cached, but no basename index yet.
	if _, err := cm.db.Exec("DELETE FROM game_basenames"); err != nil {
		t.Fatalf("clear basenames: %v", err)
	}
	if _, err := cm.GetRomByFSLookup("gba", "Kingdom Hearts - Chain of Memories (USA)"); err == nil {
		t.Fatal("precondition: lookup should miss before backfill")
	}

	if err := backfillGameBasenames(cm.db); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	got, err := cm.GetRomByFSLookup("gba", "Kingdom Hearts - Chain of Memories (USA)")
	if err != nil || got.ID != 7972 {
		t.Fatalf("after backfill: got (%d, %v), want (7972, nil)", got.ID, err)
	}
}

// A plain single file (fs_name == the file) must keep resolving as before.
func TestGetRomByFSLookup_SimpleSingleFileStillMatches(t *testing.T) {
	cm := newTestManager(t)
	rom := romm.Rom{
		ID:               20552,
		PlatformID:       5,
		PlatformFSSlug:   "gba",
		Name:             "Metroid: Scrolls 6",
		FsName:           "Metroid_ Scrolls 6 (1.1).gba",
		FsNameNoExt:      "Metroid_ Scrolls 6 (1.1)",
		HasMultipleFiles: false,
		Files:            []romm.RomFile{{FileName: "Metroid_ Scrolls 6 (1.1).gba"}},
	}
	if err := cm.SavePlatformGames(5, []romm.Rom{rom}); err != nil {
		t.Fatalf("save games: %v", err)
	}

	got, err := cm.GetRomByFSLookup("gba", "Metroid_ Scrolls 6 (1.1)")
	if err != nil {
		t.Fatalf("expected to resolve simple single file, got error: %v", err)
	}
	if got.ID != 20552 {
		t.Errorf("got rom ID %d, want 20552", got.ID)
	}
}
