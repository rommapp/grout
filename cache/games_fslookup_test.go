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
