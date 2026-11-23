package library

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rommapp/grout/core/romm"
)

func TestMetaFields(t *testing.T) {
	date := int64(631152000000)
	rating := 82.5
	g := fileGame(1, "Tetris", "Tetris.gba", "Tetris.bin")
	g.Summary = "Line one\nLine two"
	g.Metadatum.Genres = []string{"Puzzle", "Arcade"}
	g.Metadatum.Companies = []string{"Nintendo"}
	g.Metadatum.PlayerCount = "1-2"
	g.Metadatum.FirstReleaseDate = &date
	g.Metadatum.AverageRating = &rating
	g.Regions = []string{"USA", "Japan"}
	g.Languages = []string{"en", "ja"}
	g.Files[0].MD5Hash = "aaaa"
	l, _ := scanLibrary(t, fstest.MapFS{}, g)
	got, err := l.Meta(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := GameMeta{
		GameID:      1,
		PlatformID:  1,
		Name:        "Tetris",
		Summary:     "Line one\nLine two",
		ReleaseDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Rating:      82.5,
		HasRating:   true,
		Players:     "1-2",
		Genres:      []string{"Arcade", "Puzzle"},
		Companies:   []string{"Nintendo"},
		Regions:     []string{"Japan", "USA"},
		Languages:   []string{"en", "ja"},
		MD5ByName:   map[string]string{"Tetris.gba": "aaaa"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("meta = %+v", got)
	}
}

func TestMetaNoRatingNoDate(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{}, fileGame(1, "Tetris", "Tetris.gba"))
	got, err := l.Meta(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ReleaseDate.IsZero() || got.HasRating || got.Rating != 0 {
		t.Fatalf("meta = %+v", got)
	}
	if len(got.Genres) != 0 || len(got.MD5ByName) != 0 {
		t.Fatalf("meta = %+v", got)
	}
}

func TestMetaUnknownGame(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{}, fileGame(1, "Tetris", "Tetris.gba"))
	if _, err := l.Meta(context.Background(), 99); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("err = %v", err)
	}
}

func onCardIDs(got []OnCardGame) []int {
	var ids []int
	for _, g := range got {
		ids = append(ids, g.GameID)
	}
	return ids
}

func TestOnCardIncludesScanMatches(t *testing.T) {
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Tetris.gba": {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Zed.gba":    {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gba"), fileGame(2, "Zed", "Zed.gba"), fileGame(3, "None", "None.gba"))
	ctx := context.Background()
	mustScan(t, l)
	if err := l.RecordFetch(ctx, Download{GameID: 2, PlatformID: 1, LaunchPath: gbaDir + "/Zed.gba", Placed: []string{gbaDir + "/Zed.gba"}}); err != nil {
		t.Fatal(err)
	}
	got, err := l.OnCard(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []OnCardGame{
		{GameID: 1, PlatformID: 1, LaunchPath: gbaDir + "/Tetris.gba"},
		{GameID: 2, PlatformID: 1, LaunchPath: gbaDir + "/Zed.gba"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on card = %+v", got)
	}
}

func TestOnCardPlatformFilter(t *testing.T) {
	n64 := gameRom(2, 2, "Mario", "", nil, nil)
	n64.Files = []romm.RomFile{{ID: 200, FileName: "Mario.z64"}}
	l, h := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Tetris.gba": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gba"), n64)
	ctx := context.Background()
	mustScan(t, l)
	h.Files[romRoot+"Nintendo 64 (N64)/Mario.z64"] = &fstest.MapFile{Data: []byte("x")}
	if err := l.RecordFetch(ctx, Download{GameID: 2, PlatformID: 2, LaunchPath: "/" + romRoot + "Nintendo 64 (N64)/Mario.z64"}); err != nil {
		t.Fatal(err)
	}
	all, err := l.OnCard(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(onCardIDs(all), []int{1, 2}) {
		t.Fatalf("all = %+v", all)
	}
	one, err := l.OnCard(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(onCardIDs(one), []int{2}) || one[0].PlatformID != 2 {
		t.Fatalf("one = %+v", one)
	}
	none, err := l.OnCard(ctx, 77)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("none = %+v err = %v", none, err)
	}
}

func TestOnCardMultiUsesM3U(t *testing.T) {
	g := fileGame(1, "Disc", "d1.chd")
	g.HasMultipleFiles = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Disc.m3u":    {Data: []byte("x")},
		romRoot + "Game Boy Advance (GBA)/Disc/d1.chd": {Data: []byte("x")},
	}, g)
	mustScan(t, l)
	got, err := l.OnCard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []OnCardGame{{GameID: 1, PlatformID: 1, LaunchPath: gbaDir + "/Disc.m3u"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on card = %+v", got)
	}
}

func TestOnCardFolderLaunch(t *testing.T) {
	g := fileGame(1, "Big", "a.chd")
	g.HasMultipleFiles = true
	l, _ := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Big/a.chd": {Data: []byte("x")},
	}, g)
	mustScan(t, l)
	got, err := l.OnCard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []OnCardGame{{GameID: 1, PlatformID: 1, LaunchPath: gbaDir + "/Big", Folder: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on card = %+v", got)
	}
}

func TestOnCardDropsVanishedLaunchPath(t *testing.T) {
	l, h := scanLibrary(t, fstest.MapFS{
		romRoot + "Game Boy Advance (GBA)/Tetris.gba": {Data: []byte("x")},
	}, fileGame(1, "Tetris", "Tetris.gba"))
	mustScan(t, l)
	delete(h.Files, romRoot+"Game Boy Advance (GBA)/Tetris.gba")
	got, err := l.OnCard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("on card = %+v", got)
	}
}
