package download

import (
	"os"
	"strings"
	"testing"

	"grout/library"
	"grout/romm"
	"grout/settings"
)

func TestMain(m *testing.M) {
	// Planning resolves the firmware, which decides where artwork goes and what
	// it is called. Pin one so these tests do not depend on the developer's
	// environment.
	if os.Getenv("CFW") == "" {
		os.Setenv("CFW", "ROCKNIX")
	}
	os.Exit(m.Run())
}

func testHost() settings.Host { return settings.Host{RootURI: "http://example.invalid"} }
func testPlatform() romm.Platform {
	return romm.Platform{ID: 1, FSSlug: "nds", Name: "Nintendo DS"}
}

// Regression test for issue #223: a cached row written without a files array
// must be skipped, not panic on Files[0].
func TestBuildPlan_EmptyFilesIsSkippedNotFatal(t *testing.T) {
	games := []romm.Rom{{
		ID:          60,
		Name:        "Professor Layton and the Curious Village",
		FsName:      "Professor Layton and the Curious Village (USA).nds",
		FsNameNoExt: "Professor Layton and the Curious Village (USA)",
		Files:       nil,
	}}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildPlan panicked on an empty Files slice: %v", r)
		}
	}()

	plan, skipped := BuildPlan(settings.Config{}, testHost(), testPlatform(), games, 0)

	if len(plan.Roms) != 0 || len(plan.Art) != 0 || len(plan.Entries) != 0 {
		t.Errorf("expected an empty plan, got %d roms, %d art, %d entries",
			len(plan.Roms), len(plan.Art), len(plan.Entries))
	}
	// The skip must be reported, or the user is told nothing about a game that
	// silently never arrives.
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skip, got %d", len(skipped))
	}
	if skipped[0].Game.ID != 60 || skipped[0].Reason == "" {
		t.Errorf("skip = %+v, want game 60 with a reason", skipped[0])
	}
}

func TestBuildPlan_SingleFile(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		Files: []romm.RomFile{{ID: 100, FileName: "test.nds"}},
	}}

	plan, skipped := BuildPlan(settings.Config{}, testHost(), testPlatform(), games, 0)

	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", skipped)
	}
	if len(plan.Roms) != 1 || len(plan.Entries) != 1 {
		t.Fatalf("got %d roms and %d entries, want 1 each", len(plan.Roms), len(plan.Entries))
	}
	if plan.Roms[0].URL == "" {
		t.Error("expected a download URL")
	}
	if !strings.HasSuffix(plan.Roms[0].Location, "test.nds") {
		t.Errorf("location = %q, want it to end in the file name", plan.Roms[0].Location)
	}
}

// A game with several versions downloads the one the user chose, not the first.
func TestBuildPlan_SelectedFileWins(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		Files: []romm.RomFile{
			{ID: 100, FileName: "test (USA).nds"},
			{ID: 200, FileName: "test (Europe).nds"},
		},
	}}

	plan, _ := BuildPlan(settings.Config{}, testHost(), testPlatform(), games, 200)
	if len(plan.Roms) != 1 {
		t.Fatalf("got %d roms, want 1", len(plan.Roms))
	}
	if !strings.Contains(plan.Roms[0].Location, "Europe") {
		t.Errorf("location = %q, want the selected file", plan.Roms[0].Location)
	}
	if !strings.Contains(plan.Roms[0].URL, "file_ids=200") {
		t.Errorf("url = %q, want it to request the selected file", plan.Roms[0].URL)
	}
}

// An unknown selection falls back to the first file rather than downloading
// nothing.
func TestBuildPlan_UnknownSelectedFileFallsBackToTheFirst(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		Files: []romm.RomFile{{ID: 100, FileName: "first.nds"}, {ID: 200, FileName: "second.nds"}},
	}}

	plan, _ := BuildPlan(settings.Config{}, testHost(), testPlatform(), games, 999)
	if len(plan.Roms) != 1 || !strings.Contains(plan.Roms[0].Location, "first.nds") {
		t.Errorf("roms = %+v, want a fallback to the first file", plan.Roms)
	}
}

// A multi-disc game is fetched as one archive into a temp directory, because it
// has to be expanded before it is usable.
func TestBuildPlan_MultiDiscGoesToTemp(t *testing.T) {
	games := []romm.Rom{{
		ID: 7, Name: "Final Fantasy VII", FsName: "ff7.zip", FsNameNoExt: "ff7",
		HasMultipleFiles: true,
	}}

	plan, skipped := BuildPlan(settings.Config{}, testHost(), testPlatform(), games, 0)
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", skipped)
	}
	if len(plan.Roms) != 1 {
		t.Fatalf("got %d roms, want 1", len(plan.Roms))
	}
	if !strings.Contains(plan.Roms[0].Location, "grout_multirom_7.zip") {
		t.Errorf("location = %q, want a temp archive named for the game id", plan.Roms[0].Location)
	}
}

// Art is only planned when the user asked for it.
func TestBuildPlan_NoArtWhenDisabled(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		PathCoverLarge: "/covers/test.png",
		Files:          []romm.RomFile{{ID: 100, FileName: "test.nds"}},
	}}

	plan, _ := BuildPlan(settings.Config{DownloadArt: false}, testHost(), testPlatform(), games, 0)
	if len(plan.Art) != 0 {
		t.Errorf("got %d art items with DownloadArt off, want 0", len(plan.Art))
	}
}

// A game the server has no cover for gets no art, even with art enabled.
func TestBuildPlan_NoArtWhenTheGameHasNone(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		Files: []romm.RomFile{{ID: 100, FileName: "test.nds"}},
	}}

	config := settings.Config{DownloadArt: true, ArtKind: library.ArtKindDefault}
	plan, _ := BuildPlan(config, testHost(), testPlatform(), games, 0)
	if len(plan.Art) != 0 {
		t.Errorf("got %d art items for a game with no cover, want 0", len(plan.Art))
	}
}

func TestBuildPlan_CoverArtIsPlannedAndRecorded(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		PathCoverLarge: "/covers/test.png",
		Files:          []romm.RomFile{{ID: 100, FileName: "test.nds"}},
	}}

	config := settings.Config{DownloadArt: true, ArtKind: library.ArtKindDefault}
	plan, _ := BuildPlan(config, testHost(), testPlatform(), games, 0)

	if len(plan.Art) == 0 {
		t.Fatal("expected cover art to be planned")
	}
	if !plan.Art[0].IsImage {
		t.Error("cover art should be marked as an image so it is normalised")
	}
	// The entry has to point at the file, or the frontend shows no cover.
	if plan.Entries[0].Game.Art.Cover == "" {
		t.Error("expected the entry to record where the cover was written")
	}
	if plan.Entries[0].Game.Art.Cover != plan.Art[0].Location {
		t.Errorf("entry records %q but art is written to %q",
			plan.Entries[0].Game.Art.Cover, plan.Art[0].Location)
	}
}

// A collection can span platforms, so a game's own platform wins over the
// screen's when the screen has none.
func TestBuildPlan_GameOwnPlatformWinsInACollection(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.gba", FsNameNoExt: "test",
		PlatformID: 9, PlatformFSSlug: "gba", PlatformDisplayName: "Game Boy Advance",
		Files: []romm.RomFile{{ID: 100, FileName: "test.gba"}},
	}}

	// A zero-ID platform is what a collection screen passes.
	plan, _ := BuildPlan(settings.Config{}, testHost(), romm.Platform{}, games, 0)
	if len(plan.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(plan.Entries))
	}
	if plan.Entries[0].Platform.FSSlug != "gba" {
		t.Errorf("platform = %q, want the game's own gba", plan.Entries[0].Platform.FSSlug)
	}
}

// Videos and manuals are written as they arrive; marking them as images would
// send them through PNG normalisation.
func TestBuildPlan_NonImageArtIsNotMarkedAsImage(t *testing.T) {
	for _, spec := range artSpecs {
		if spec.fixedExt == "" {
			continue
		}
		if spec.fixedExt != ".mp4" && spec.fixedExt != ".pdf" {
			t.Errorf("slot %v has unexpected fixed extension %q", spec.slot, spec.fixedExt)
		}
	}
}

// Two art kinds writing to the same place would have one overwrite the other.
func TestBuildPlan_ArtLocationsAreDistinct(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game", FsName: "test.nds", FsNameNoExt: "test",
		PathCoverLarge: "/covers/test.png", URLCover: "http://example.invalid/c.png",
		Files: []romm.RomFile{{ID: 100, FileName: "test.nds"}},
	}}

	config := settings.Config{
		DownloadArt:                  true,
		ArtKind:                      library.ArtKindDefault,
		DownloadArtScreenshotPreview: true,
	}
	plan, _ := BuildPlan(config, testHost(), testPlatform(), games, 0)

	seen := map[string]bool{}
	for _, item := range plan.Art {
		if seen[item.Location] {
			t.Errorf("two art items write to %q", item.Location)
		}
		seen[item.Location] = true
	}
}

// The gamelist carries the region by default, and leaves it out when asked.
func TestBuildPlan_GamelistNameRegion(t *testing.T) {
	games := []romm.Rom{{
		ID: 42, Name: "Test Game (USA)", FsName: "test.nds", FsNameNoExt: "test",
		Files:   []romm.RomFile{{ID: 100, FileName: "test.nds"}},
		Regions: []string{"USA"},
	}}

	for _, tc := range []struct {
		omit bool
		want string
	}{
		{omit: false, want: "Test Game (USA)"},
		{omit: true, want: "Test Game"},
	} {
		config := settings.Config{GamelistOmitsRegion: tc.omit}
		plan, _ := BuildPlan(config, testHost(), testPlatform(), games, 0)

		if len(plan.Entries) != 1 {
			t.Fatalf("omit=%v: expected 1 entry, got %d", tc.omit, len(plan.Entries))
		}
		if got := plan.Entries[0].Game.DisplayName; got != tc.want {
			t.Errorf("omit=%v: gamelist name = %q, want %q", tc.omit, got, tc.want)
		}
	}
}
