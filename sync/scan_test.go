package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupMuOSSaveFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()

	// Emulator save directories (matches muOS save_directories.json structure)
	dirs := []string{
		"file/Gambatte",
		"file/mGBA",
		"file/FCEUmm",
		"file/Snes9x",
		"file/PCSX-ReARMed",
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(base, d), 0755)
	}

	// Save files
	saves := map[string]string{
		"file/Gambatte/Kirby's Pinball Land (USA, Europe).srm": "save-data",
		"file/Gambatte/Tetris Rosy Retrospection.srm":          "save-data",
		"file/Gambatte/Tetris Chromatic.srm":                   "save-data",
		"file/mGBA/Advance Wars.srm":                           "save-data",
		"file/mGBA/Pokemon - Recharged Yellow.srm":             "save-data",
		"file/FCEUmm/Tecmo Super Bowl 2025.srm":                "save-data",
		"file/Snes9x/Super Mario World.srm":                    "save-data",
		"file/PCSX-ReARMed/Bust-A-Move 4 (USA).sav":           "save-data",
	}
	for path, content := range saves {
		os.WriteFile(filepath.Join(base, path), []byte(content), 0644)
	}

	return base
}

func mockRomLookup(knownRoms map[string]map[string]int) RomLookupFunc {
	return func(fsSlug, nameNoExt string) (int, string, bool) {
		if platforms, ok := knownRoms[fsSlug]; ok {
			if id, ok := platforms[nameNoExt]; ok {
				return id, nameNoExt, true
			}
		}
		return 0, "", false
	}
}

func TestScanSavesInDir_FindsAllSaves(t *testing.T) {
	base := setupMuOSSaveFixture(t)

	emulatorMap := map[string][]string{
		"gb":  {"file/Gambatte"},
		"gbc": {"file/Gambatte"},
		"gba": {"file/mGBA"},
		"nes": {"file/FCEUmm"},
	}

	knownRoms := map[string]map[string]int{
		"gb": {
			"Kirby's Pinball Land (USA, Europe)": 1,
			"Tetris Rosy Retrospection":          2,
		},
		"gbc": {
			"Tetris Chromatic": 3,
		},
		"gba": {
			"Advance Wars":                4,
			"Pokemon - Recharged Yellow":   5,
		},
		"nes": {
			"Tecmo Super Bowl 2025": 6,
		},
	}

	saves := scanSavesInDir(base, emulatorMap, mockRomLookup(knownRoms), nil)

	if len(saves) != 6 {
		t.Errorf("expected 6 saves, got %d", len(saves))
		for _, s := range saves {
			t.Logf("  found: %s (romID=%d, slug=%s)", s.FileName, s.RomID, s.FSSlug)
		}
	}
}

func TestScanSavesInDir_SkipsNonExistentDirs(t *testing.T) {
	base := t.TempDir()
	// Don't create any directories

	emulatorMap := map[string][]string{
		"gb": {"file/Gambatte", "file/SameBoy"},
	}

	saves := scanSavesInDir(base, emulatorMap, func(_, _ string) (int, string, bool) {
		t.Error("lookup should not be called for nonexistent dirs")
		return 0, "", false
	}, nil)

	if len(saves) != 0 {
		t.Errorf("expected 0 saves, got %d", len(saves))
	}
}

func TestScanSavesInDir_SkipsHiddenFiles(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "file/Gambatte")
	os.MkdirAll(dir, 0755)

	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("junk"), 0644)
	os.WriteFile(filepath.Join(dir, ".backup_meta"), []byte("junk"), 0644)
	os.WriteFile(filepath.Join(dir, "Real Game.srm"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"gb": {"file/Gambatte"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"gb": {"Real Game": 1},
	})

	saves := scanSavesInDir(base, emulatorMap, lookup, nil)

	if len(saves) != 1 {
		t.Errorf("expected 1 save, got %d", len(saves))
	}
	if len(saves) > 0 && saves[0].FileName != "Real Game.srm" {
		t.Errorf("expected 'Real Game.srm', got %q", saves[0].FileName)
	}
}

func TestScanSavesInDir_SkipsInvalidExtensions(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "file/mGBA")
	os.MkdirAll(dir, 0755)

	os.WriteFile(filepath.Join(dir, "game.srm"), []byte("save"), 0644)
	os.WriteFile(filepath.Join(dir, "game.txt"), []byte("notes"), 0644)
	os.WriteFile(filepath.Join(dir, "game.png"), []byte("image"), 0644)
	os.WriteFile(filepath.Join(dir, "game.sav"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"gba": {"file/mGBA"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"gba": {"game": 1},
	})

	saves := scanSavesInDir(base, emulatorMap, lookup, nil)

	if len(saves) != 2 {
		t.Errorf("expected 2 saves (.srm and .sav), got %d", len(saves))
		for _, s := range saves {
			t.Logf("  found: %s", s.FileName)
		}
	}
}

func TestScanSavesInDir_SkipsDirectories(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "file/Gambatte")
	os.MkdirAll(dir, 0755)
	os.MkdirAll(filepath.Join(dir, ".backup"), 0755)

	os.WriteFile(filepath.Join(dir, "game.srm"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"gb": {"file/Gambatte"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"gb": {"game": 1},
	})

	saves := scanSavesInDir(base, emulatorMap, lookup, nil)

	if len(saves) != 1 {
		t.Errorf("expected 1 save, got %d", len(saves))
	}
}

func TestScanSavesInDir_NoMatchInCache(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "file/Gambatte")
	os.MkdirAll(dir, 0755)

	os.WriteFile(filepath.Join(dir, "Unknown Game.srm"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"gb": {"file/Gambatte"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"gb": {}, // no known roms
	})

	saves := scanSavesInDir(base, emulatorMap, lookup, nil)

	if len(saves) != 0 {
		t.Errorf("expected 0 saves (no cache match), got %d", len(saves))
	}
}

func TestScanSavesInDir_WithSlugResolver(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "file/Snes9x")
	os.MkdirAll(dir, 0755)

	os.WriteFile(filepath.Join(dir, "Super Mario World.srm"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"sfam": {"file/Snes9x"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"snes": {"Super Mario World": 1}, // note: mapped slug, not original
	})

	resolver := func(slug string) string {
		if slug == "sfam" {
			return "snes"
		}
		return slug
	}

	saves := scanSavesInDir(base, emulatorMap, lookup, resolver)

	if len(saves) != 1 {
		t.Errorf("expected 1 save with slug resolution, got %d", len(saves))
	}
	if len(saves) > 0 && saves[0].FSSlug != "snes" {
		t.Errorf("expected resolved slug 'snes', got %q", saves[0].FSSlug)
	}
}

func TestScanSavesInDir_MultipleEmulatorDirs(t *testing.T) {
	base := t.TempDir()

	os.MkdirAll(filepath.Join(base, "file/Gambatte"), 0755)
	os.MkdirAll(filepath.Join(base, "file/SameBoy"), 0755)

	os.WriteFile(filepath.Join(base, "file/Gambatte/game1.srm"), []byte("save"), 0644)
	os.WriteFile(filepath.Join(base, "file/SameBoy/game2.srm"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"gb": {"file/Gambatte", "file/SameBoy"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"gb": {"game1": 1, "game2": 2},
	})

	saves := scanSavesInDir(base, emulatorMap, lookup, nil)

	if len(saves) != 2 {
		t.Errorf("expected 2 saves across emulators, got %d", len(saves))
	}
}

func TestScanSavesInDir_PopulatesLocalSaveFields(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "file/mGBA")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "Advance Wars.srm"), []byte("save"), 0644)

	emulatorMap := map[string][]string{"gba": {"file/mGBA"}}
	lookup := mockRomLookup(map[string]map[string]int{
		"gba": {"Advance Wars": 42},
	})

	saves := scanSavesInDir(base, emulatorMap, lookup, nil)

	if len(saves) != 1 {
		t.Fatalf("expected 1 save, got %d", len(saves))
	}

	s := saves[0]
	if s.RomID != 42 {
		t.Errorf("RomID = %d, want 42", s.RomID)
	}
	if s.RomName != "Advance Wars" {
		t.Errorf("RomName = %q, want 'Advance Wars'", s.RomName)
	}
	if s.FSSlug != "gba" {
		t.Errorf("FSSlug = %q, want 'gba'", s.FSSlug)
	}
	if s.FileName != "Advance Wars.srm" {
		t.Errorf("FileName = %q, want 'Advance Wars.srm'", s.FileName)
	}
	if s.EmulatorDir != "file/mGBA" {
		t.Errorf("EmulatorDir = %q, want 'file/mGBA'", s.EmulatorDir)
	}
	expectedPath := filepath.Join(base, "file/mGBA/Advance Wars.srm")
	if s.FilePath != expectedPath {
		t.Errorf("FilePath = %q, want %q", s.FilePath, expectedPath)
	}
}

func TestCreateBackup_CreatesBackupFile(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "game.srm")
	os.WriteFile(savePath, []byte("original-save"), 0644)

	mtime := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	os.Chtimes(savePath, mtime, mtime)

	backupPath, err := createBackup(savePath, "game.srm")
	if err != nil {
		t.Fatalf("createBackup failed: %v", err)
	}

	if backupPath == "" {
		t.Fatal("expected backup path, got empty string")
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}

	if string(data) != "original-save" {
		t.Errorf("backup content = %q, want 'original-save'", string(data))
	}

	// Verify .backup directory was created
	backupDir := filepath.Join(dir, ".backup")
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error(".backup directory was not created")
	}
}

func TestCreateBackup_NoFileToBackup(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "nonexistent.srm")

	backupPath, err := createBackup(savePath, "nonexistent.srm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backupPath != "" {
		t.Errorf("expected empty path for nonexistent file, got %q", backupPath)
	}
}

func TestCreateBackup_ReadOnlyDirectory(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "game.srm")
	os.WriteFile(savePath, []byte("save"), 0644)

	// Make directory read-only so .backup can't be created
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755)

	_, err := createBackup(savePath, "game.srm")
	if err == nil {
		t.Error("expected error for read-only directory, got nil")
	}
}

func TestCreateBackup_TimestampInFilename(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "game.srm")
	os.WriteFile(savePath, []byte("save"), 0644)

	mtime := time.Date(2026, 3, 28, 14, 30, 45, 0, time.Local)
	os.Chtimes(savePath, mtime, mtime)

	backupPath, err := createBackup(savePath, "game.srm")
	if err != nil {
		t.Fatalf("createBackup failed: %v", err)
	}

	expectedName := "game [2026-03-28 14-30-45].srm"
	if filepath.Base(backupPath) != expectedName {
		t.Errorf("backup filename = %q, want %q", filepath.Base(backupPath), expectedName)
	}
}

func TestValidSaveExtensions(t *testing.T) {
	valid := []string{".srm", ".sav", ".dsv", ".mcr", ".mcd", ".brm", ".eep", ".sra", ".fla", ".mpk", ".nv"}
	for _, ext := range valid {
		if !ValidSaveExtensions[ext] {
			t.Errorf("expected %q to be a valid save extension", ext)
		}
	}

	invalid := []string{".txt", ".png", ".zip", ".gba", ".nes", ".rom", ".bin", ""}
	for _, ext := range invalid {
		if ValidSaveExtensions[ext] {
			t.Errorf("expected %q to NOT be a valid save extension", ext)
		}
	}
}
