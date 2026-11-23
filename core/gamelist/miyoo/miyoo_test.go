package miyoo

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist/entry"
)

func games(t *testing.T, out []byte) []*etree.Element {
	t.Helper()
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(out); err != nil {
		t.Fatal(err)
	}
	return doc.FindElements("/gameList/game")
}

func TestMiyooOwned(t *testing.T) {
	if got, want := Owned(), []string{"name", "path", "image"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Owned = %v, want %v", got, want)
	}
	if FileName != "miyoogamelist.xml" {
		t.Fatalf("FileName = %q", FileName)
	}
}

func TestMiyooElements(t *testing.T) {
	out, changed, err := Merge(nil, []entry.Entry{{
		Path:        "./Game.gba",
		Name:        "Game",
		Desc:        "About",
		ReleaseDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Rating:      80,
		HasRating:   true,
		Players:     "1",
		Genres:      []string{"Action"},
		Developers:  []string{"Dev"},
		Regions:     []string{"USA"},
		Languages:   []string{"en"},
		MD5:         "d41d8cd98f00b204e9800998ecf8427e",
		Art: map[firmware.ArtKind]string{
			firmware.ArtCover:     "./Imgs/Game.png",
			firmware.ArtThumbnail: "./Imgs/Game-thumb.png",
			firmware.ArtVideo:     "./videos/Game.mp4",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("new document reported unchanged")
	}
	g := games(t, out)
	if len(g) != 1 {
		t.Fatalf("games = %d, want 1", len(g))
	}
	if len(g[0].Attr) != 0 {
		t.Fatalf("game has attributes %v", g[0].Attr)
	}
	var names []string
	values := map[string]string{}
	for _, c := range g[0].ChildElements() {
		names = append(names, c.Tag)
		values[c.Tag] = c.Text()
	}
	if got, want := strings.Join(names, ","), "path,name,image"; got != want {
		t.Fatalf("children = %s, want %s", got, want)
	}
	if values["path"] != "./Game.gba" || values["name"] != "Game" || values["image"] != "./Imgs/Game.png" {
		t.Fatalf("values = %v", values)
	}
}

func TestMiyooKeepsScraperThumbnail(t *testing.T) {
	existing := []byte(`<?xml version="1.0"?>
<gameList>
    <game>
        <path>./Game.gba</path>
        <name>Old</name>
        <thumbnail>./Imgs/scraped.png</thumbnail>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, []entry.Entry{{
		Path: "./Game.gba",
		Name: "New",
		Art:  map[firmware.ArtKind]string{firmware.ArtThumbnail: "./Imgs/Game-thumb.png"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("update reported unchanged")
	}
	g := games(t, out)[0]
	if g.SelectElement("name").Text() != "New" || g.SelectElement("thumbnail").Text() != "./Imgs/scraped.png" {
		t.Fatalf("unexpected game:\n%s", out)
	}
}
