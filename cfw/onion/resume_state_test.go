package onion

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("state"), 0644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var stateTime = time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)

func TestSetAsideRenamesTheAutoStateAndItsPreview(t *testing.T) {
	states := t.TempDir()
	auto := filepath.Join(states, "gpSP", "Pokemon Monomyth.state.auto")
	touch(t, auto)
	touch(t, auto+".png")

	moved, err := setAsideResumeState(states, "/mnt/SDCARD/Roms/GBA/Pokemon Monomyth.gba", stateTime)
	if err != nil || moved != 2 {
		t.Fatalf("moved=%d err=%v", moved, err)
	}
	if exists(auto) || exists(auto+".png") {
		t.Error("the resume state is still in place")
	}
	if !exists(auto+"_20261110-093000") || !exists(auto+".png_20261110-093000") {
		t.Error("the state was not kept under its new name")
	}
}

func TestSetAsideLeavesOtherStatesAndOtherGamesAlone(t *testing.T) {
	states := t.TempDir()
	keep := []string{
		filepath.Join(states, "gpSP", "Pokemon Monomyth.state"),
		filepath.Join(states, "gpSP", "Pokemon Monomyth.state1"),
		filepath.Join(states, "gpSP", "Pokemon Monomyth 2.state.auto"),
		filepath.Join(states, "gpSP", "Other.state.auto"),
	}
	for _, p := range keep {
		touch(t, p)
	}

	moved, err := setAsideResumeState(states, "/roms/Pokemon Monomyth.gba", stateTime)
	if err != nil || moved != 0 {
		t.Fatalf("moved=%d err=%v", moved, err)
	}
	for _, p := range keep {
		if !exists(p) {
			t.Errorf("%s was touched", p)
		}
	}
}

func TestSetAsideFindsTheStateUnderAnyCore(t *testing.T) {
	states := t.TempDir()
	a := filepath.Join(states, "mGBA", "Game.state.auto")
	touch(t, a)

	if moved, _ := setAsideResumeState(states, "/roms/Game.zip", stateTime); moved != 1 {
		t.Errorf("moved=%d", moved)
	}
}

func TestSetAsideWithNoStatesFolderIsNotAnError(t *testing.T) {
	moved, err := setAsideResumeState(filepath.Join(t.TempDir(), "nope"), "/roms/Game.gb", stateTime)
	if moved != 0 || err != nil {
		t.Errorf("moved=%d err=%v", moved, err)
	}
}

func TestSetAsideKeepsOnlyTheLastFewCopies(t *testing.T) {
	states := t.TempDir()
	dir := filepath.Join(states, "Gambatte")
	auto := filepath.Join(dir, "Game.state.auto")

	for day := 1; day <= 5; day++ {
		touch(t, auto)
		setAsideResumeState(states, "/roms/Game.gb", time.Date(2026, 11, day, 10, 0, 0, 0, time.UTC))
	}

	matches, _ := filepath.Glob(auto + "_*")
	if len(matches) != keptSetAsideStates {
		t.Fatalf("kept %d copies, want %d: %v", len(matches), keptSetAsideStates, matches)
	}
	if !exists(auto + "_20261105-100000") {
		t.Error("the newest copy was pruned")
	}
	if exists(auto + "_20261101-100000") {
		t.Error("the oldest copy was kept")
	}
}

func TestSetAsideHandlesBracketsInGameNames(t *testing.T) {
	states := t.TempDir()
	auto := filepath.Join(states, "gpSP", "Game [USA].state.auto")
	touch(t, auto)

	moved, err := setAsideResumeState(states, "/roms/Game [USA].gba", stateTime)
	if err != nil || moved != 1 {
		t.Fatalf("moved=%d err=%v", moved, err)
	}
}

// A device whose clock has been set back stamps its newest copy as the oldest.
// That copy is the one that was just made, and pruning must not take it.
func TestPruningNeverRemovesTheCopyJustSetAside(t *testing.T) {
	states := t.TempDir()
	dir := filepath.Join(states, "Gambatte")
	auto := filepath.Join(dir, "Game.state.auto")

	// Three older copies, stamped later than "now" by the device's clock.
	for _, stamp := range []string{"20261201-100000", "20261202-100000", "20261203-100000"} {
		touch(t, auto+"_"+stamp)
	}
	touch(t, auto)

	if _, err := setAsideResumeState(states, "/roms/Game.gb", time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	if !exists(auto + "_20261101-100000") {
		t.Error("the copy just set aside was pruned")
	}
	matches, _ := filepath.Glob(auto + "_*")
	if len(matches) != keptSetAsideStates {
		t.Errorf("%d copies kept, want %d: %v", len(matches), keptSetAsideStates, matches)
	}
}
