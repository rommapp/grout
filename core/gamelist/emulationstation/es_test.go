package emulationstation

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist/entry"
)

func onlyGame(t *testing.T, out []byte) *etree.Element {
	t.Helper()
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(out); err != nil {
		t.Fatal(err)
	}
	games := doc.FindElements("/gameList/game")
	if len(games) != 1 {
		t.Fatalf("games = %d, want 1:\n%s", len(games), out)
	}
	return games[0]
}

func children(game *etree.Element) ([]string, map[string]string) {
	var names []string
	values := map[string]string{}
	for _, c := range game.ChildElements() {
		names = append(names, c.Tag)
		values[c.Tag] = c.Text()
	}
	return names, values
}

func TestESOwned(t *testing.T) {
	want := []string{"name", "desc", "path", "releasedate", "rating", "players", "genre", "developer", "region", "lang", "md5", "image", "thumbnail", "marquee", "video", "manual", "bezel", "boxback", "fanart"}
	if got := Owned(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Owned = %v, want %v", got, want)
	}
	if FileName != "gamelist.xml" {
		t.Fatalf("FileName = %q", FileName)
	}
}

func TestESElements(t *testing.T) {
	e := entry.Entry{
		Path:        "./Game.chd",
		Name:        "Game",
		Desc:        "About the game",
		ReleaseDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Rating:      82.5,
		HasRating:   true,
		Players:     "1-2",
		Genres:      []string{"Action", "Platform"},
		Developers:  []string{"Dev A", "Dev B"},
		Regions:     []string{"USA", "Europe"},
		Languages:   []string{"en", "fr"},
		MD5:         "d41d8cd98f00b204e9800998ecf8427e",
		Art: map[firmware.ArtKind]string{
			firmware.ArtCover:     "./images/Game.png",
			firmware.ArtThumbnail: "./images/Game-thumb.png",
			firmware.ArtMarquee:   "./images/Game-marquee.png",
			firmware.ArtVideo:     "./videos/Game.mp4",
			firmware.ArtManual:    "./manuals/Game.pdf",
			firmware.ArtBezel:     "./bezels/Game.png",
			firmware.ArtBoxback:   "./images/Game-boxback.png",
			firmware.ArtFanart:    "./images/Game-fanart.png",
			firmware.ArtPreview:   "./previews/Game.png",
			firmware.ArtSplash:    "./splash/Game.png",
		},
	}
	out, changed, err := Merge(nil, []entry.Entry{e})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("new document reported unchanged")
	}
	g := onlyGame(t, out)
	if len(g.Attr) != 0 {
		t.Fatalf("game has attributes %v", g.Attr)
	}
	names, values := children(g)
	want := map[string]string{
		"path":        "./Game.chd",
		"name":        "Game",
		"desc":        "About the game",
		"releasedate": "19900101T000000",
		"rating":      "0.825",
		"players":     "1-2",
		"genre":       "Action, Platform",
		"developer":   "Dev A, Dev B",
		"region":      "USA, Europe",
		"lang":        "en, fr",
		"md5":         "d41d8cd98f00b204e9800998ecf8427e",
		"image":       "./images/Game.png",
		"thumbnail":   "./images/Game-thumb.png",
		"marquee":     "./images/Game-marquee.png",
		"video":       "./videos/Game.mp4",
		"manual":      "./manuals/Game.pdf",
		"bezel":       "./bezels/Game.png",
		"boxback":     "./images/Game-boxback.png",
		"fanart":      "./images/Game-fanart.png",
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %v, want %v", values, want)
	}
	if len(names) != len(Owned()) {
		t.Fatalf("children = %v, want one of each owned element", names)
	}
	if names[0] != "path" {
		t.Fatalf("first child = %s, want path", names[0])
	}
}

func TestESRatingAndDateFormat(t *testing.T) {
	loc := time.FixedZone("plus5", 5*3600)
	out, _, err := Merge(nil, []entry.Entry{{
		Path:        "./Game.chd",
		Name:        "Game",
		ReleaseDate: time.Date(1990, 1, 1, 3, 4, 5, 0, loc),
		Rating:      80,
		HasRating:   true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, values := children(onlyGame(t, out))
	if values["rating"] != "0.8" {
		t.Fatalf("rating = %q, want 0.8", values["rating"])
	}
	if values["releasedate"] != "19891231T220405" {
		t.Fatalf("releasedate = %q, want UTC 19891231T220405", values["releasedate"])
	}
}

func TestESOmitsEmptyFields(t *testing.T) {
	out, changed, err := Merge(nil, []entry.Entry{{
		Path:   "./Game.chd",
		Name:   "Game",
		Rating: 50,
		Art:    map[firmware.ArtKind]string{firmware.ArtCover: "", firmware.ArtVideo: "./videos/Game.mp4"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("new document reported unchanged")
	}
	names, _ := children(onlyGame(t, out))
	if got, want := strings.Join(names, ","), "path,name,video"; got != want {
		t.Fatalf("children = %s, want %s", got, want)
	}
}

func TestESKeepsScraperElementsOnUpdate(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game id="99">
        <path>./Game.chd</path>
        <name>Old</name>
        <playcount>2</playcount>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, []entry.Entry{{Path: "./Game.chd", Name: "New"}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("update reported unchanged")
	}
	g := onlyGame(t, out)
	if g.SelectAttrValue("id", "") != "99" {
		t.Fatal("id attribute lost")
	}
	_, values := children(g)
	if values["name"] != "New" || values["playcount"] != "2" {
		t.Fatalf("values = %v", values)
	}
}
