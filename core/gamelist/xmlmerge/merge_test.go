package xmlmerge

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

var testOwned = []string{"name", "desc", "path", "image"}

func parse(t *testing.T, b []byte) *etree.Element {
	t.Helper()
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(b); err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, b)
	}
	root := doc.SelectElement("gameList")
	if root == nil {
		t.Fatalf("output has no gameList root:\n%s", b)
	}
	return root
}

func childText(t *testing.T, game *etree.Element, name string) string {
	t.Helper()
	e := game.SelectElement(name)
	if e == nil {
		t.Fatalf("game has no <%s>", name)
	}
	return e.Text()
}

func childNames(game *etree.Element) []string {
	var names []string
	for _, c := range game.ChildElements() {
		names = append(names, c.Tag)
	}
	return names
}

func TestMergeNewDocument(t *testing.T) {
	for _, existing := range [][]byte{nil, []byte(""), []byte("  \n\t ")} {
		out, changed, err := Merge(existing, testOwned, []Game{{
			Path:     "./Game.chd",
			Elements: []Element{{Name: "name", Value: "Game"}, {Name: "image", Value: "./media/Game.png"}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("new document reported unchanged")
		}
		if !bytes.HasPrefix(out, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
			t.Fatalf("missing xml declaration:\n%s", out)
		}
		if !strings.Contains(string(out), "\n    <game>\n        <path>") {
			t.Fatalf("not indented with 4 spaces:\n%s", out)
		}
		games := parse(t, out).SelectElements("game")
		if len(games) != 1 {
			t.Fatalf("games = %d, want 1", len(games))
		}
		if got, want := strings.Join(childNames(games[0]), ","), "path,name,image"; got != want {
			t.Fatalf("children = %s, want %s", got, want)
		}
		if childText(t, games[0], "path") != "./Game.chd" || childText(t, games[0], "name") != "Game" {
			t.Fatalf("wrong values:\n%s", out)
		}
	}
}

func TestMergeNewDocumentNoGamesIsChanged(t *testing.T) {
	out, changed, err := Merge(nil, testOwned, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("new empty document reported unchanged")
	}
	parse(t, out)
}

func TestMergeAppendsNewGame(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Other.chd</path>
        <name>Other</name>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, testOwned, []Game{{
		Path:     "./Game.chd",
		Elements: []Element{{Name: "name", Value: "Game"}, {Name: "desc", Value: "About"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("append reported unchanged")
	}
	games := parse(t, out).SelectElements("game")
	if len(games) != 2 {
		t.Fatalf("games = %d, want 2", len(games))
	}
	if childText(t, games[0], "name") != "Other" {
		t.Fatal("existing game moved or changed")
	}
	if got, want := strings.Join(childNames(games[1]), ","), "path,name,desc"; got != want {
		t.Fatalf("children = %s, want %s", got, want)
	}
}

func TestMergeUpdatesMatchedGameByBaseName(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>  ./sub/Game.chd  </path>
        <name>Old</name>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, testOwned, []Game{{
		Path:     "./Game.chd",
		Elements: []Element{{Name: "name", Value: "New"}, {Name: "image", Value: "./media/Game.png"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("update reported unchanged")
	}
	games := parse(t, out).SelectElements("game")
	if len(games) != 1 {
		t.Fatalf("games = %d, want 1", len(games))
	}
	if got := childText(t, games[0], "path"); got != "  ./sub/Game.chd  " {
		t.Fatalf("path rewritten to %q", got)
	}
	if childText(t, games[0], "name") != "New" {
		t.Fatal("name not updated")
	}
	if got, want := strings.Join(childNames(games[0]), ","), "path,name,image"; got != want {
		t.Fatalf("children = %s, want %s", got, want)
	}
}

func TestMergeFirstMatchWins(t *testing.T) {
	existing := []byte(`<gameList><game><path>a/Game.chd</path><name>One</name></game><game><path>b/Game.chd</path><name>Two</name></game></gameList>`)
	out, _, err := Merge(existing, testOwned, []Game{{Path: "Game.chd", Elements: []Element{{Name: "name", Value: "New"}}}})
	if err != nil {
		t.Fatal(err)
	}
	games := parse(t, out).SelectElements("game")
	if childText(t, games[0], "name") != "New" || childText(t, games[1], "name") != "Two" {
		t.Fatalf("wrong game updated:\n%s", out)
	}
}

func TestMergeKeepsUnownedElementsAndAttributes(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Before.chd</path>
        <name>Before</name>
        <favorite>true</favorite>
    </game>
    <game id="4242" source="ScreenScraper">
        <!-- scraped by skraper -->
        <path>./Game.chd</path>
        <name>Old Name</name>
        <favorite>true</favorite>
        <playcount>7</playcount>
        <lastplayed>20240101T120000</lastplayed>
        <thumbnail>./media/thumbs/Game.png</thumbnail>
    </game>
    <game>
        <path>./After.chd</path>
        <name>After</name>
        <playcount>3</playcount>
    </game>
</gameList>
`)
	owned := []string{"name", "path", "image"}
	out, changed, err := Merge(existing, owned, []Game{{
		Path: "./Game.chd",
		Elements: []Element{
			{Name: "name", Value: "New Name"},
			{Name: "image", Value: "./Imgs/Game.png"},
			{Name: "thumbnail", Value: "./Imgs/Other.png"},
			{Name: "favorite", Value: "false"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("update reported unchanged")
	}
	root := parse(t, out)
	games := root.SelectElements("game")
	if len(games) != 3 {
		t.Fatalf("games = %d, want 3", len(games))
	}
	for i, want := range []string{"./Before.chd", "./Game.chd", "./After.chd"} {
		if got := childText(t, games[i], "path"); got != want {
			t.Fatalf("game %d path = %q, want %q", i, got, want)
		}
	}
	if childText(t, games[0], "name") != "Before" || childText(t, games[0], "favorite") != "true" {
		t.Fatal("game before changed")
	}
	if childText(t, games[2], "name") != "After" || childText(t, games[2], "playcount") != "3" {
		t.Fatal("game after changed")
	}
	g := games[1]
	if g.SelectAttrValue("id", "") != "4242" || g.SelectAttrValue("source", "") != "ScreenScraper" {
		t.Fatal("attributes lost")
	}
	for name, want := range map[string]string{
		"name":       "New Name",
		"image":      "./Imgs/Game.png",
		"favorite":   "true",
		"playcount":  "7",
		"lastplayed": "20240101T120000",
		"thumbnail":  "./media/thumbs/Game.png",
	} {
		if got := childText(t, g, name); got != want {
			t.Fatalf("<%s> = %q, want %q", name, got, want)
		}
	}
	var comments int
	for _, tok := range g.Child {
		if c, ok := tok.(*etree.Comment); ok && c.Data == " scraped by skraper " {
			comments++
		}
	}
	if comments != 1 {
		t.Fatalf("comment count = %d, want 1:\n%s", comments, out)
	}
	if got, want := strings.Join(childNames(g), ","), "path,name,favorite,playcount,lastplayed,thumbnail,image"; got != want {
		t.Fatalf("children = %s, want %s", got, want)
	}
}

func TestMergeEmptyValueKeepsOld(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Game.chd</path>
        <name>Game</name>
        <desc>Scraped summary</desc>
        <image>./Imgs/Game.png</image>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, testOwned, []Game{{
		Path:     "./Game.chd",
		Elements: []Element{{Name: "name", Value: "Game"}, {Name: "desc", Value: ""}, {Name: "image", Value: ""}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("empty values reported changed:\n%s", out)
	}
	if !bytes.Equal(out, existing) {
		t.Fatalf("bytes changed:\n%s", out)
	}
	g := parse(t, out).SelectElement("game")
	if childText(t, g, "desc") != "Scraped summary" || childText(t, g, "image") != "./Imgs/Game.png" {
		t.Fatal("old values lost")
	}
}

func TestMergeUnchangedReportsNotChanged(t *testing.T) {
	game := Game{
		Path:     "./Game.chd",
		Elements: []Element{{Name: "name", Value: "Game"}, {Name: "image", Value: "./Imgs/Game.png"}},
	}
	first, changed, err := Merge(nil, testOwned, []Game{game})
	if err != nil || !changed {
		t.Fatalf("first merge: changed=%v err=%v", changed, err)
	}
	second, changed, err := Merge(first, testOwned, []Game{game})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("second merge reported changed:\n%s", second)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("second merge changed bytes")
	}
	if n := len(parse(t, second).SelectElements("game")); n != 1 {
		t.Fatalf("games = %d, want 1", n)
	}
}

func TestMergeUnparseable(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Game.chd</path>
        <name>Gam`)
	out, changed, err := Merge(existing, testOwned, []Game{{Path: "./Game.chd", Elements: []Element{{Name: "name", Value: "Game"}}}})
	if !errors.Is(err, ErrUnparseable) {
		t.Fatalf("err = %v, want ErrUnparseable", err)
	}
	if changed || out != nil {
		t.Fatalf("unparseable merge returned changed=%v out=%q", changed, out)
	}
}

func TestMergeMissingRoot(t *testing.T) {
	for _, existing := range []string{
		`<?xml version="1.0"?><games><game><path>./Game.chd</path></game></games>`,
		`<?xml version="1.0"?><!-- only a comment -->`,
	} {
		_, changed, err := Merge([]byte(existing), testOwned, []Game{{Path: "./Game.chd", Elements: []Element{{Name: "name", Value: "Game"}}}})
		if !errors.Is(err, ErrUnparseable) {
			t.Fatalf("%s: err = %v, want ErrUnparseable", existing, err)
		}
		if changed {
			t.Fatal("changed on error")
		}
	}
}

func TestMergeEscapesAndRoundTrips(t *testing.T) {
	name := `Tom & Jerry <"Ünïcode">`
	desc := "First line & more.\nSecond line <with> \"quotes\"."
	out, _, err := Merge(nil, testOwned, []Game{{
		Path:     "./Tom & Jerry.chd",
		Elements: []Element{{Name: "name", Value: name}, {Name: "desc", Value: desc}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	g := parse(t, out).SelectElement("game")
	if got := childText(t, g, "name"); got != name {
		t.Fatalf("name = %q, want %q", got, name)
	}
	if got := childText(t, g, "desc"); got != desc {
		t.Fatalf("desc = %q, want %q", got, desc)
	}
	if got := childText(t, g, "path"); got != "./Tom & Jerry.chd" {
		t.Fatalf("path = %q", got)
	}
	again, changed, err := Merge(out, testOwned, []Game{{
		Path:     "./Tom & Jerry.chd",
		Elements: []Element{{Name: "name", Value: name}, {Name: "desc", Value: desc}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if changed || !bytes.Equal(again, out) {
		t.Fatalf("round trip changed:\n%s\n---\n%s", out, again)
	}
}

func TestMergeIgnoresUnownedOnNewGame(t *testing.T) {
	out, _, err := Merge(nil, []string{"name", "path"}, []Game{{
		Path:     "./Game.chd",
		Elements: []Element{{Name: "name", Value: "Game"}, {Name: "favorite", Value: "true"}, {Name: "path", Value: "./Other.chd"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	g := parse(t, out).SelectElement("game")
	if got, want := strings.Join(childNames(g), ","), "path,name"; got != want {
		t.Fatalf("children = %s, want %s", got, want)
	}
	if childText(t, g, "path") != "./Game.chd" {
		t.Fatal("path came from Elements")
	}
}

func TestMergePrefersExactPathOverBaseName(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Hacks/Game.sfc</path>
        <name>Hack</name>
    </game>
    <game>
        <path> Game.sfc </path>
        <name>Old</name>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, testOwned, []Game{{
		Path:     "./Game.sfc",
		Elements: []Element{{Name: "name", Value: "New"}, {Name: "image", Value: "./Imgs/Game.png"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("update reported unchanged")
	}
	games := parse(t, out).SelectElements("game")
	if len(games) != 2 {
		t.Fatalf("games = %d, want 2", len(games))
	}
	if got, want := strings.Join(childNames(games[0]), ","), "path,name"; got != want {
		t.Fatalf("hack children = %s, want %s", got, want)
	}
	if childText(t, games[0], "name") != "Hack" || childText(t, games[0], "path") != "./Hacks/Game.sfc" {
		t.Fatalf("hack entry changed:\n%s", out)
	}
	if childText(t, games[1], "name") != "New" || childText(t, games[1], "image") != "./Imgs/Game.png" {
		t.Fatalf("exact entry not updated:\n%s", out)
	}
}

func TestMergeKeepsWhitespaceOnlyLeaves(t *testing.T) {
	existing := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Other.chd</path>
        <name>Other</name>
        <desc> </desc>
        <notes>
</notes>
    </game>
</gameList>
`)
	out, changed, err := Merge(existing, testOwned, []Game{{Path: "./Game.chd", Elements: []Element{{Name: "name", Value: "Game"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("append reported unchanged")
	}
	other := parse(t, out).SelectElements("game")[0]
	if got := childText(t, other, "desc"); got != " " {
		t.Fatalf("desc = %q, want a single space", got)
	}
	if got := childText(t, other, "notes"); got != "\n" {
		t.Fatalf("notes = %q, want a newline", got)
	}
}
