package ui

import (
	"os"
	"path/filepath"
	"testing"

	"grout/romm"
)

type fakeResolver struct{ dir string }

func (f fakeResolver) GetPlatformRomDirectory(romm.Platform) string { return f.dir }

func TestMissingGames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "have.gba"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "multi-have.m3u"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	games := []romm.Rom{
		{Name: "Have", PlatformFSSlug: "gba", Files: []romm.RomFile{{FileName: "have.gba"}}},
		{Name: "Missing", PlatformFSSlug: "gba", Files: []romm.RomFile{{FileName: "missing.gba"}}},
		{Name: "MultiHave", PlatformFSSlug: "gba", HasMultipleFiles: true, FsNameNoExt: "multi-have"},
		{Name: "MultiMissing", PlatformFSSlug: "gba", HasMultipleFiles: true, FsNameNoExt: "multi-missing"},
		{Name: "NoSlug"},
	}

	missing := missingGames(games, fakeResolver{dir: dir})

	got := make(map[string]bool, len(missing))
	for _, g := range missing {
		got[g.Name] = true
	}

	want := []string{"Missing", "MultiMissing", "NoSlug"}
	if len(missing) != len(want) {
		t.Fatalf("expected %d missing (%v), got %d (%v)", len(want), want, len(missing), got)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("expected %q to be reported missing", name)
		}
	}
}
