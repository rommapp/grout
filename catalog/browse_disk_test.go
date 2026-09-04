package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"grout/cfw"
	"grout/romm"
	"grout/settings"
)

// romsOnCard lays out a muOS rom directory holding one complete single-file
// game and one multi-disc game with only its first disc, and returns the two
// roms that describe them.
func romsOnCard(t *testing.T) (complete, partial romm.Rom) {
	t.Helper()

	base := t.TempDir()
	t.Setenv("BASE_PATH", base)
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	dir := filepath.Join(base, "ROMS", "snes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Mario.zip", "FF7 (Disc 1).chd"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	complete = romm.Rom{
		ID: 1, Name: "Mario", PlatformFSSlug: "snes", FsNameNoExt: "Mario",
		Files: []romm.RomFile{{FileName: "Mario.zip"}},
	}
	partial = romm.Rom{
		ID: 2, Name: "FF7", PlatformFSSlug: "snes", FsNameNoExt: "FF7",
		HasNestedSingleFile: true,
		Files: []romm.RomFile{
			{FileName: "FF7 (Disc 1).chd"},
			{FileName: "FF7 (Disc 2).chd"},
		},
	}
	return complete, partial
}

func browseOnCard(t *testing.T, mode settings.DownloadedGamesMode, games ...romm.Rom) GameList {
	t.Helper()
	return Browse(BrowseRequest{Games: games, Config: settings.Config{DownloadedGames: mode}})
}

func entryFor(list GameList, id int) (GameEntry, bool) {
	for _, entry := range list.Entries {
		if entry.Game.ID == id {
			return entry, true
		}
	}
	return GameEntry{}, false
}

// With the markers switched off nothing reads the download state, so it must
// not be looked up: every answer is a stat on a slow card, once per game.
func TestBrowse_DoNothingSkipsTheCard(t *testing.T) {
	complete, partial := romsOnCard(t)

	list := browseOnCard(t, settings.DownloadedGamesModeDoNothing, complete, partial)

	if len(list.Entries) != 2 {
		t.Fatalf("entries = %d, want both games left alone", len(list.Entries))
	}
	for _, entry := range list.Entries {
		if entry.Downloaded != NotDownloaded {
			t.Errorf("%s reported %v, but nothing asked for a download state", entry.Name, entry.Downloaded)
		}
	}

	// The multi-file marker says something else entirely and is not gated on
	// the download setting.
	if entry, _ := entryFor(list, partial.ID); !entry.MultipleFiles {
		t.Error("the multi-file flag must survive with the download markers off")
	}
}

func TestBrowse_MarkReportsWhatIsOnTheCard(t *testing.T) {
	complete, partial := romsOnCard(t)

	list := browseOnCard(t, settings.DownloadedGamesModeMark, complete, partial)

	if entry, ok := entryFor(list, complete.ID); !ok || entry.Downloaded != FullyDownloaded {
		t.Errorf("Mario = %v, want FullyDownloaded", entry.Downloaded)
	}
	// Only disc 1 is there, which has to read differently from having it all.
	if entry, ok := entryFor(list, partial.ID); !ok || entry.Downloaded != PartlyDownloaded {
		t.Errorf("FF7 = %v, want PartlyDownloaded with one disc of two", entry.Downloaded)
	}
}

// Filter hides what the user already has. A game whose later discs are still
// missing is not one of those, and hiding it would leave no way to finish it.
func TestBrowse_FilterKeepsPartlyDownloadedGames(t *testing.T) {
	complete, partial := romsOnCard(t)

	list := browseOnCard(t, settings.DownloadedGamesModeFilter, complete, partial)

	if _, ok := entryFor(list, complete.ID); ok {
		t.Error("a game that is entirely on the card must be hidden")
	}
	if _, ok := entryFor(list, partial.ID); !ok {
		t.Error("a game still missing a disc must stay listed so it can be finished")
	}
}
