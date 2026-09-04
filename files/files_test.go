package files

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSubdirectoryNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"gba", "snes", ".Trashes"} {
		if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := SubdirectoryNames(root)
	if err != nil {
		t.Fatalf("SubdirectoryNames: %v", err)
	}
	slices.Sort(got)

	// Files and the hidden folders these cards collect are not rom folders.
	if want := []string{"gba", "snes"}; !slices.Equal(got, want) {
		t.Errorf("SubdirectoryNames = %v, want %v", got, want)
	}
}

// The rom root lives on a removable card, so a missing one is routine and has
// to come back as an error the caller can show.
func TestSubdirectoryNames_Missing(t *testing.T) {
	if _, err := SubdirectoryNames(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Error("expected an error for a directory that does not exist")
	}
}
