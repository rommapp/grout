package cache

import (
	"testing"

	"grout/romm"
)

func genreValues(t *testing.T, cm *Manager, platformID int) []string {
	t.Helper()
	got, err := cm.GetDistinctValuesWithFilter("genres", "game_genres", "genre_id", platformID, GameFilter{})
	if err != nil {
		t.Fatalf("GetDistinctValuesWithFilter(genres): %v", err)
	}
	return got
}

// An incremental library refresh (RefreshPlatformGamesWithProgress) only re-fetches games
// changed since the last sync, then hands that partial set to SavePlatformGames. Games that
// weren't part of the incremental set must keep their metadata-junction rows, or the Filters
// screen loses every value and pressing Y opens an empty (silently-cancelled) screen.
func TestSavePlatformGames_IncrementalSavePreservesUntouchedJunctions(t *testing.T) {
	cm := newTestManager(t)

	full := []romm.Rom{
		{ID: 1, PlatformID: 5, PlatformFSSlug: "gba", Name: "Alpha",
			Metadatum: romm.RomMetadata{Genres: []string{"Action"}}},
		{ID: 2, PlatformID: 5, PlatformFSSlug: "gba", Name: "Beta",
			Metadatum: romm.RomMetadata{Genres: []string{"Puzzle"}}},
	}
	if err := cm.SavePlatformGames(5, full); err != nil {
		t.Fatalf("full save: %v", err)
	}
	if got := genreValues(t, cm, 5); len(got) != 2 {
		t.Fatalf("after full save, want [Action Puzzle], got %v", got)
	}

	// Incremental refresh re-fetched only game 1 (game 2 unchanged upstream).
	if err := cm.SavePlatformGames(5, []romm.Rom{full[0]}); err != nil {
		t.Fatalf("incremental save: %v", err)
	}

	// Game 2's genre must survive: it wasn't in the incremental set.
	if got := genreValues(t, cm, 5); len(got) != 2 {
		t.Errorf("after incremental save, want both genres [Action Puzzle], got %v", got)
	}
}

// The v14 migration must reconstruct the metadata junction tables from data_json, so caches
// that lost their junctions to the pre-fix incremental wipe get their Filters values back
// WITHOUT a library re-download.
func TestBackfillMetadataJunctions_RebuildsFromDataJSON(t *testing.T) {
	cm := newTestManager(t)

	games := []romm.Rom{
		{ID: 1, PlatformID: 5, PlatformFSSlug: "gba", Name: "Alpha",
			Metadatum: romm.RomMetadata{Genres: []string{"Action"}, Companies: []string{"Nintendo"}}},
		{ID: 2, PlatformID: 5, PlatformFSSlug: "gba", Name: "Beta",
			Metadatum: romm.RomMetadata{Genres: []string{"Puzzle"}}},
	}
	if err := cm.SavePlatformGames(5, games); err != nil {
		t.Fatalf("save games: %v", err)
	}

	// Simulate a cache corrupted by the pre-fix incremental wipe: junctions gone, data_json
	// intact.
	for _, table := range junctionTables {
		if _, err := cm.db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("wipe %s: %v", table, err)
		}
	}
	if got := genreValues(t, cm, 5); len(got) != 0 {
		t.Fatalf("precondition: junctions should be wiped, got %v", got)
	}

	if err := backfillMetadataJunctions(cm.db); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if got := genreValues(t, cm, 5); len(got) != 2 {
		t.Errorf("after backfill, want [Action Puzzle], got %v", got)
	}
	companies, err := cm.GetDistinctValuesWithFilter("companies", "game_companies", "company_id", 5, GameFilter{})
	if err != nil {
		t.Fatalf("companies: %v", err)
	}
	if len(companies) != 1 || companies[0] != "Nintendo" {
		t.Errorf("after backfill, want companies [Nintendo], got %v", companies)
	}
}
