package saves

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helpers shared by this package's tests, kept in one file so that each test
// file stands on its own.

func saveIn(t *testing.T, root, folder, name, content string) LocalSave {
	t.Helper()
	path := filepath.Join(root, folder, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return LocalSave{RomID: 1, FileName: name, FilePath: path, EmulatorDir: folder}
}

func copyIn(t *testing.T, root string, romID int, folder, name, content string, modified time.Time) LocalSave {
	t.Helper()
	save := saveIn(t, root, folder, name, content)
	save.RomID = romID
	save.RomName = "Pokemon Monomyth"
	if err := os.Chtimes(save.FilePath, modified, modified); err != nil {
		t.Fatal(err)
	}
	return save
}

var yesterday = time.Date(2026, 11, 9, 20, 15, 30, 0, time.Local)

func str(s string) *string { return &s }
