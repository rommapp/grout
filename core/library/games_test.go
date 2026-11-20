package library

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"grout/core/config"
	"grout/core/romm"
)

func gameRom(id, platform int, name, sortKey string, genres, regions []string) romm.Rom {
	return romm.Rom{
		ID:          id,
		PlatformID:  platform,
		Name:        name,
		NameSortKey: sortKey,
		FSNameNoExt: name,
		Metadatum:   romm.RomMetadata{Genres: genres},
		Regions:     regions,
	}
}

func newGamesLibrary(t *testing.T, roms ...romm.Rom) *Library {
	t.Helper()
	cfg := &config.Config{DirectoryMappings: mappings("gba")}
	l := newPlatformsLibrary(t, cfg, nil,
		romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "GBA", RomCount: 5},
		romm.Platform{ID: 2, Slug: "n64", FSSlug: "n64", Name: "N64", RomCount: 1},
	)
	inTx(t, l.Store, func(tx *sql.Tx) error { return saveGames(tx, roms) })
	return l
}

func standardRoms() []romm.Rom {
	return []romm.Rom{
		gameRom(1, 1, "Pokémon Emerald", "", []string{"RPG"}, []string{"USA"}),
		gameRom(2, 1, "The Legend of Zelda: A Link to the Past", "legend of zelda, the", []string{"Action", "Adventure"}, []string{"USA", "Europe"}),
		gameRom(3, 1, "100% Orange Juice", "", []string{"Action"}, []string{"Japan"}),
		gameRom(4, 1, "1000 Miglia", "", []string{"Racing"}, []string{"Europe"}),
		gameRom(5, 1, "Dragon Quest", "", []string{"RPG"}, []string{"Japan"}),
		gameRom(6, 2, "Disabled Game", "", []string{"RPG"}, []string{"USA"}),
	}
}

func gameIDs(gs []Game) []int {
	var out []int
	for _, g := range gs {
		out = append(out, g.ID)
	}
	return out
}

func mustGames(t *testing.T, l *Library, q Query) []Game {
	t.Helper()
	gs, err := l.Games(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return gs
}

func TestGamesSearchFoldsAccentsAndTokens(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	cases := []struct {
		search string
		want   []int
	}{
		{"pokemon", []int{1}},
		{"zelda link", []int{2}},
		{"link zelda", []int{2}},
		{"100%", []int{3}},
		{"", []int{3, 4, 5, 2, 1}},
	}
	for _, c := range cases {
		got := gameIDs(mustGames(t, l, Query{Search: c.search}))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("search %q: got %v, want %v", c.search, got, c.want)
		}
	}
}

func TestGamesFiltersAndAcrossOrWithin(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	got := gameIDs(mustGames(t, l, Query{Filters: map[Dim][]string{DimGenre: {"RPG", "Action"}}}))
	if !reflect.DeepEqual(got, []int{3, 5, 2, 1}) {
		t.Errorf("union: got %v", got)
	}
	got = gameIDs(mustGames(t, l, Query{Filters: map[Dim][]string{DimGenre: {"RPG", "Action"}, DimRegion: {"USA"}}}))
	if !reflect.DeepEqual(got, []int{2, 1}) {
		t.Errorf("intersect: got %v", got)
	}
}

func TestGamesUnknownDimIsError(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	if _, err := l.Games(context.Background(), Query{Filters: map[Dim][]string{"bogus": {"x"}}}); err == nil {
		t.Error("want error")
	}
	if _, err := l.FilterValues(context.Background(), Query{Filters: map[Dim][]string{"bogus": {"x"}}}, DimGenre); err == nil {
		t.Error("want error for filter values")
	}
	if _, err := l.FilterValues(context.Background(), Query{}, "bogus"); err == nil {
		t.Error("want error for dim")
	}
}

func TestGamesExcludesDisabledPlatforms(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	inTx(t, l.Store, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO collections (kind, remote_id, name) VALUES ('regular', 7, 'Mix')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO collection_games (kind, remote_id, game_id) VALUES ('regular', 7, 1), ('regular', 7, 6)`)
		return err
	})
	key := &CollectionKey{Kind: romm.CollectionRegular, RemoteID: "7"}
	got := gameIDs(mustGames(t, l, Query{Collection: key}))
	if !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("got %v", got)
	}
	got = gameIDs(mustGames(t, l, Query{PlatformID: 2}))
	if len(got) != 0 {
		t.Errorf("disabled platform: got %v", got)
	}
}

func TestGamesOrderUsesSortKey(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	got := gameIDs(mustGames(t, l, Query{PlatformID: 1}))
	if !reflect.DeepEqual(got, []int{3, 4, 5, 2, 1}) {
		t.Errorf("got %v", got)
	}
}

func TestFilterValuesNarrowsByOtherDims(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	q := Query{Filters: map[Dim][]string{DimRegion: {"Japan"}}}
	genres, err := l.FilterValues(context.Background(), q, DimGenre)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(genres, []string{"Action", "RPG"}) {
		t.Errorf("genres: %v", genres)
	}
	regions, err := l.FilterValues(context.Background(), q, DimRegion)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(regions, []string{"Europe", "Japan", "USA"}) {
		t.Errorf("regions: %v", regions)
	}
}

func TestGamesCollectionRemoteIDStringsAndNumbers(t *testing.T) {
	l := newGamesLibrary(t, standardRoms()...)
	inTx(t, l.Store, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			`INSERT INTO collections (kind, remote_id, name) VALUES ('virtual', 'recent', 'Recent'), ('regular', '7', 'Seven')`,
			`INSERT INTO collection_games (kind, remote_id, game_id) VALUES ('virtual', 'recent', 2), ('virtual', 'recent', 3), ('regular', '7', 1)`,
		} {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}
		return nil
	})
	got := gameIDs(mustGames(t, l, Query{Collection: &CollectionKey{Kind: romm.CollectionVirtual, RemoteID: "recent"}}))
	if !reflect.DeepEqual(got, []int{3, 2}) {
		t.Errorf("virtual: got %v", got)
	}
	got = gameIDs(mustGames(t, l, Query{Collection: &CollectionKey{Kind: romm.CollectionRegular, RemoteID: "7"}}))
	if !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("numeric: got %v", got)
	}
}
