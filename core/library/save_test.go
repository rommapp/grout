package library

import (
	"database/sql"
	"fmt"
	"testing"

	"grout/core/romm"
	"grout/core/store"
)

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Pokémon: Emerald!":       "pokemon emerald",
		"  The  Legend_of Zelda ": "the legend of zelda",
		"100%":                    "100",
		"":                        "",
	}
	for in, want := range cases {
		if got := fold(in); got != want {
			t.Errorf("fold(%q) = %q, want %q", in, got, want)
		}
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func inTx(t *testing.T, s *store.Store, fn func(tx *sql.Tx) error) {
	t.Helper()
	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, s *store.Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSaveGamesReplacesOnlyThatGamesRows(t *testing.T) {
	s := openTestStore(t)
	g1 := romm.Rom{ID: 1, PlatformID: 1, Name: "One", Metadatum: romm.RomMetadata{Genres: []string{"RPG"}}}
	g2 := romm.Rom{ID: 2, PlatformID: 1, Name: "Two", Metadatum: romm.RomMetadata{Genres: []string{"Action", "Puzzle"}}}
	inTx(t, s, func(tx *sql.Tx) error { return saveGames(tx, []romm.Rom{g1, g2}) })

	g1.Metadatum.Genres = []string{"Sports"}
	inTx(t, s, func(tx *sql.Tx) error { return saveGames(tx, []romm.Rom{g1}) })

	if n := count(t, s, `SELECT COUNT(*) FROM game_meta WHERE game_id = 2 AND dim = 'genre'`); n != 2 {
		t.Fatalf("game 2 genres = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM game_meta WHERE game_id = 1`); n != 1 {
		t.Fatalf("game 1 meta rows = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM game_meta WHERE game_id = 1 AND dim = 'genre' AND value = 'Sports'`); n != 1 {
		t.Fatal("game 1 should have only the new genre")
	}
}

func TestSaveGamesMetaDimsAndNames(t *testing.T) {
	s := openTestStore(t)
	g := romm.Rom{
		ID: 3, PlatformID: 1, Name: "Pokémon Emerald", FSNameNoExt: "Pokemon - Emerald (U)",
		Regions: []string{"USA", ""}, Languages: []string{"en"}, Tags: []string{"hack"},
		Metadatum: romm.RomMetadata{
			Genres: []string{"RPG"}, Franchises: []string{"Pokemon"}, Companies: []string{"GF"},
			GameModes: []string{"Single"}, AgeRatings: []string{"E"},
		},
	}
	inTx(t, s, func(tx *sql.Tx) error { return saveGames(tx, []romm.Rom{g}) })

	for _, dim := range []string{"genre", "franchise", "company", "game_mode", "age_rating", "region", "language", "tag"} {
		if n := count(t, s, `SELECT COUNT(*) FROM game_meta WHERE game_id = 3 AND dim = ?`, dim); n != 1 {
			t.Errorf("dim %s rows = %d", dim, n)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM game_meta WHERE game_id = 3`); n != 8 {
		t.Errorf("meta rows = %d", n)
	}
	var sortKey, search string
	if err := s.DB.QueryRow(`SELECT sort_key, search_name FROM games WHERE id = 3`).Scan(&sortKey, &search); err != nil {
		t.Fatal(err)
	}
	if sortKey != "pokemon emerald" || search != "pokemon emerald pokemon emerald u" {
		t.Fatalf("sort_key = %q search_name = %q", sortKey, search)
	}

	g.NameSortKey = "Emerald, Pokemon"
	inTx(t, s, func(tx *sql.Tx) error { return saveGames(tx, []romm.Rom{g}) })
	if err := s.DB.QueryRow(`SELECT sort_key FROM games WHERE id = 3`).Scan(&sortKey); err != nil {
		t.Fatal(err)
	}
	if sortKey != "emerald pokemon" {
		t.Fatalf("sort_key = %q", sortKey)
	}
}

func TestSaveGamesKindAndPlayable(t *testing.T) {
	s := openTestStore(t)
	multi := romm.Rom{ID: 1, PlatformID: 1, HasMultipleFiles: true, Files: []romm.RomFile{
		{ID: 10, FileName: "disc.cue", Category: ""},
		{ID: 11, FileName: "manual.pdf", Category: "manual"},
	}}
	nested := romm.Rom{ID: 2, PlatformID: 1, HasNestedSingleFile: true}
	single := romm.Rom{ID: 3, PlatformID: 1}
	inTx(t, s, func(tx *sql.Tx) error { return saveGames(tx, []romm.Rom{multi, nested, single}) })

	for id, want := range map[int]string{1: "multi", 2: "nested", 3: "single"} {
		var kind string
		if err := s.DB.QueryRow(`SELECT kind FROM games WHERE id = ?`, id).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		if kind != want {
			t.Errorf("game %d kind = %q, want %q", id, kind, want)
		}
	}
	if n := count(t, s, `SELECT playable FROM game_files WHERE game_id = 1 AND file_id = 11`); n != 0 {
		t.Errorf("manual playable = %d", n)
	}
	if n := count(t, s, `SELECT playable FROM game_files WHERE game_id = 1 AND file_id = 10`); n != 1 {
		t.Errorf("rom playable = %d", n)
	}
}

func TestSaveCollectionReplacesOnlyItsMembers(t *testing.T) {
	s := openTestStore(t)
	a := romm.Collection{Kind: romm.CollectionRegular, RemoteID: "1", Name: "A", RomIDs: []int{1, 2, 3}}
	b := romm.Collection{Kind: romm.CollectionRegular, RemoteID: "2", Name: "B", RomIDs: []int{4, 5}}
	inTx(t, s, func(tx *sql.Tx) error {
		if err := saveCollection(tx, a); err != nil {
			return err
		}
		return saveCollection(tx, b)
	})

	a.RomIDs = []int{9}
	inTx(t, s, func(tx *sql.Tx) error { return saveCollection(tx, a) })

	if n := count(t, s, `SELECT COUNT(*) FROM collection_games WHERE remote_id = 2`); n != 2 {
		t.Fatalf("collection B members = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM collection_games WHERE remote_id = 1`); n != 1 {
		t.Fatalf("collection A members = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM collections`); n != 2 {
		t.Fatalf("collections = %d", n)
	}
}

func TestPurgeCollectionsExceptScopesByKind(t *testing.T) {
	s := openTestStore(t)
	inTx(t, s, func(tx *sql.Tx) error {
		for _, c := range []romm.Collection{
			{Kind: romm.CollectionRegular, RemoteID: "1", RomIDs: []int{1}},
			{Kind: romm.CollectionRegular, RemoteID: "2", RomIDs: []int{1}},
			{Kind: romm.CollectionSmart, RemoteID: "1", RomIDs: []int{1}},
			{Kind: romm.CollectionVirtual, RemoteID: "x", RomIDs: []int{1}},
		} {
			if err := saveCollection(tx, c); err != nil {
				return err
			}
		}
		return nil
	})
	inTx(t, s, func(tx *sql.Tx) error {
		n, err := purgeCollectionsExcept(tx, romm.CollectionRegular, []string{"2"})
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("deleted = %d", n)
		}
		return nil
	})
	if n := count(t, s, `SELECT COUNT(*) FROM collections`); n != 3 {
		t.Fatalf("collections = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM collection_games WHERE kind = 'regular'`); n != 1 {
		t.Fatalf("regular members = %d", n)
	}
	inTx(t, s, func(tx *sql.Tx) error {
		n, err := purgeCollectionsExcept(tx, romm.CollectionVirtual, nil)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("virtual deleted = %d", n)
		}
		return nil
	})
}

func TestPurgeExceptEmptyKeepDeletesAll(t *testing.T) {
	s := openTestStore(t)
	inTx(t, s, func(tx *sql.Tx) error {
		return savePlatforms(tx, []romm.Platform{{ID: 1, Slug: "a"}, {ID: 2, Slug: "b"}, {ID: 3, Slug: "c"}})
	})
	inTx(t, s, func(tx *sql.Tx) error {
		n, err := purgeExcept(tx, "platforms", "id", nil)
		if err != nil {
			return err
		}
		if n != 3 {
			t.Errorf("deleted = %d", n)
		}
		return nil
	})
	if n := count(t, s, `SELECT COUNT(*) FROM platforms`); n != 0 {
		t.Fatalf("platforms left = %d", n)
	}
}

func TestPurgeExceptBatches(t *testing.T) {
	s := openTestStore(t)
	roms := make([]romm.Rom, 1000)
	keep := make([]string, 0, 999)
	for i := range roms {
		roms[i] = romm.Rom{ID: i + 1, PlatformID: 1, Metadatum: romm.RomMetadata{Genres: []string{"g"}}}
		if i > 0 {
			keep = append(keep, fmt.Sprint(i+1))
		}
	}
	inTx(t, s, func(tx *sql.Tx) error { return saveGames(tx, roms) })
	inTx(t, s, func(tx *sql.Tx) error {
		n, err := purgeExcept(tx, "games", "id", keep)
		if err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("deleted = %d", n)
		}
		return nil
	})
	if n := count(t, s, `SELECT COUNT(*) FROM games`); n != 999 {
		t.Fatalf("games left = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM game_meta WHERE game_id = 1`); n != 0 {
		t.Fatalf("cascade left %d meta rows", n)
	}
}
