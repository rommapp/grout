package library

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
)

func newCollectionsLibrary(t *testing.T, cfg *config.Config, cols ...romm.Collection) *Library {
	t.Helper()
	l := newPlatformsLibrary(t, cfg, nil,
		romm.Platform{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 2},
		romm.Platform{ID: 2, Slug: "snes", FSSlug: "snes", Name: "SNES", RomCount: 1},
		romm.Platform{ID: 3, Slug: "n64", FSSlug: "n64", Name: "N64", RomCount: 1},
	)
	inTx(t, l.Store, func(tx *sql.Tx) error {
		err := saveGames(tx, []romm.Rom{
			{ID: 1, PlatformID: 1, Name: "A", FSName: "a.gba"},
			{ID: 2, PlatformID: 1, Name: "B", FSName: "b.gba"},
			{ID: 3, PlatformID: 2, Name: "C", FSName: "c.sfc"},
			{ID: 4, PlatformID: 3, Name: "D", FSName: "d.z64"},
		})
		if err != nil {
			return err
		}
		for _, c := range cols {
			if err := saveCollection(tx, c); err != nil {
				return err
			}
		}
		return nil
	})
	return l
}

func TestCollectionsHideDisabledKinds(t *testing.T) {
	cfg := &config.Config{
		DirectoryMappings:      mappings("gba", "snes"),
		ShowRegularCollections: true,
		ShowSmartCollections:   false,
	}
	l := newCollectionsLibrary(t, cfg,
		romm.Collection{Kind: romm.CollectionRegular, RemoteID: "1", Name: "Zed", RomIDs: []int{1, 2}},
		romm.Collection{Kind: romm.CollectionSmart, RemoteID: "2", Name: "Alpha", RomIDs: []int{1}},
	)
	got, err := l.Collections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []CollectionRow{{Key: CollectionKey{Kind: romm.CollectionRegular, RemoteID: "1"}, Name: "Zed", Games: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestCollectionsHideEmptyOnEnabledPlatforms(t *testing.T) {
	cfg := &config.Config{
		DirectoryMappings:      mappings("gba"),
		ShowRegularCollections: true,
	}
	l := newCollectionsLibrary(t, cfg,
		romm.Collection{Kind: romm.CollectionRegular, RemoteID: "1", Name: "Off", RomIDs: []int{3, 4}},
		romm.Collection{Kind: romm.CollectionRegular, RemoteID: "2", Name: "On", RomIDs: []int{1, 3}},
	)
	got, err := l.Collections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "On" || got[0].Games != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestCollectionPlatformsCounts(t *testing.T) {
	cfg := &config.Config{
		DirectoryMappings:      mappings("gba", "snes"),
		ShowRegularCollections: true,
	}
	l := newCollectionsLibrary(t, cfg,
		romm.Collection{Kind: romm.CollectionRegular, RemoteID: "1", Name: "Mix", RomIDs: []int{1, 2, 3, 4}},
	)
	got, err := l.CollectionPlatforms(context.Background(), CollectionKey{Kind: romm.CollectionRegular, RemoteID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].FSSlug != "gba" || got[0].RomCount != 2 || got[1].FSSlug != "snes" || got[1].RomCount != 1 {
		t.Fatalf("got %+v", got)
	}
}
