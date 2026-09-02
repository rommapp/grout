package gamelist

import (
	"path/filepath"
	"strings"
	"testing"

	"grout/domain/library"
	"grout/internal/stringutil"
)

// rom builds a game the way a caller would: the display name is already
// rendered, and identity is the file name.
func rom(name, fsName string, regions ...string) library.Game {
	return library.Game{
		FileName:    fsName,
		BaseName:    stringutil.StripExtension(fsName),
		DisplayName: stringutil.PrepareRomName(name, regions),
		Regions:     regions,
	}
}

// entry builds a RomGameEntry pointing at a rom installed in romDir.
func entry(g library.Game, romDir string) RomGameEntry {
	g.Path = filepath.Join(romDir, g.FileName)
	return RomGameEntry{
		Game:         g,
		Platform:     library.Platform{FSSlug: "gba"},
		RomDirectory: romDir,
	}
}

func gameElements(t *testing.T, gl *GameList) []string {
	t.Helper()
	root := gl.document.SelectElement(GameListElement)
	if root == nil {
		t.Fatal("no gameList root element")
	}
	var names []string
	for _, g := range root.SelectElements(GameElement) {
		if n := g.FindElement(NameElement); n != nil {
			names = append(names, n.Text())
		} else {
			names = append(names, "<no name>")
		}
	}
	return names
}

// A game whose display name differs from its raw name must not be duplicated
// when written twice. PrepareRomName appends the region, so "Sonic" is stored
// as "Sonic (USA)", so looking the entry back up by the raw name misses it.
func TestAddRomGame_RegionTaggedGameIsNotDuplicated(t *testing.T) {
	gl := New()
	e := entry(rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba", "USA"), "/roms/gba")

	gl.AddRomGame(e)
	gl.AddRomGame(e)

	if got := gameElements(t, gl); len(got) != 1 {
		t.Fatalf("expected 1 <game> element after writing the same rom twice, got %d: %v", len(got), got)
	}
}

// nameCleaner rewrites ":" to " -", so a colon in the name also makes the
// stored name diverge from the raw name.
func TestAddRomGame_ColonInNameIsNotDuplicated(t *testing.T) {
	gl := New()
	e := entry(rom("Metroid: Zero Mission", "Metroid - Zero Mission.gba"), "/roms/gba")

	gl.AddRomGame(e)
	gl.AddRomGame(e)

	if got := gameElements(t, gl); len(got) != 1 {
		t.Fatalf("expected 1 <game> element, got %d: %v", len(got), got)
	}
}

// Identity must not move when the displayed name changes. This is the
// ExcludeRegionFromName case: flipping the setting between writes must update
// the existing entry rather than appending a second one.
func TestAddRomGame_DisplayNameChangeUpdatesInPlace(t *testing.T) {
	gl := New()
	r := rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba", "USA")

	withRegion := entry(r, "/roms/gba")
	gl.AddRomGame(withRegion)

	// Same rom, but now rendered without the region suffix.
	noRegion := entry(rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba"), "/roms/gba")
	gl.AddRomGame(noRegion)

	got := gameElements(t, gl)
	if len(got) != 1 {
		t.Fatalf("expected 1 <game> element, got %d: %v", len(got), got)
	}
	if got[0] != "Sonic the Hedgehog" {
		t.Errorf("expected <name> to be updated to %q, got %q", "Sonic the Hedgehog", got[0])
	}
}

// A gamelist written by EmulationStation or another scraper uses a relative
// "./file.ext" path. Grout writes an absolute path. Both refer to the same rom
// and must resolve to the same entry.
func TestAddRomGame_AdoptsExistingRelativePathEntry(t *testing.T) {
	existing := `<?xml version="1.0" encoding="UTF-8"?>
<gameList>
    <game>
        <path>./Sonic the Hedgehog.gba</path>
        <name>Sonic The Hedgehog</name>
        <desc>An old description</desc>
    </game>
</gameList>`

	gl := New()
	if err := gl.Parse([]byte(existing)); err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba", "USA")
	r.Summary = "A new description"
	gl.AddRomGame(entry(r, "/roms/gba"))

	got := gameElements(t, gl)
	if len(got) != 1 {
		t.Fatalf("expected the existing entry to be updated, got %d entries: %v", len(got), got)
	}

	root := gl.document.SelectElement(GameListElement)
	desc := root.SelectElements(GameElement)[0].FindElement(DescElement)
	if desc == nil || desc.Text() != "A new description" {
		t.Errorf("expected description to be updated in place, got %v", desc)
	}
}

// SetGameID runs before the entry is created, so the id attribute is silently
// dropped for every new game.
func TestAddRomGame_WritesScraperIDAttributeOnNewEntry(t *testing.T) {
	gl := New()
	r := rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba", "USA")
	r.ScreenScraperID = 1234

	gl.AddRomGame(entry(r, "/roms/gba"))

	root := gl.document.SelectElement(GameListElement)
	games := root.SelectElements(GameElement)
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	if attr := games[0].SelectAttr("id"); attr == nil || attr.Value != "1234" {
		t.Errorf("expected id attribute %q on the new entry, got %v", "1234", attr)
	}
}

// Two different roms must remain two entries.
func TestAddRomGame_DistinctRomsStayDistinct(t *testing.T) {
	gl := New()
	gl.AddRomGame(entry(rom("Sonic the Hedgehog", "Sonic the Hedgehog.gba", "USA"), "/roms/gba"))
	gl.AddRomGame(entry(rom("Sonic the Hedgehog", "Sonic the Hedgehog.gbc", "Japan"), "/roms/gba"))

	if got := gameElements(t, gl); len(got) != 2 {
		t.Fatalf("expected 2 <game> elements for two distinct files, got %d: %v", len(got), got)
	}
}

// A well-formed document with an unexpected root must not panic.
func TestGameList_MalformedRootDoesNotPanic(t *testing.T) {
	gl := New()
	if err := gl.Parse([]byte(`<?xml version="1.0"?><notAGameList><game><name>X</name></game></notAGameList>`)); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got := gl.GetGameElementByName("X"); got != nil {
		t.Errorf("expected nil for a document with no gameList root, got %v", got)
	}
	if gl.Contains(NameElement, "X") {
		t.Error("expected Contains to report false for a document with no gameList root")
	}
	if gl.GameContainsElements("X", []string{NameElement}) {
		t.Error("expected GameContainsElements to report false")
	}
	gl.AddGameEntry(map[string]string{NameElement: "Y"})
	gl.AddOrUpdateEntry("Y", map[string]string{NameElement: "Y"})
	gl.AddOrUpdateRomEntry("Y.gba", map[string]string{NameElement: "Y"})
}

// The Grout launcher entry is keyed by name, not by a rom file, and must stay
// idempotent.
func TestAddOrUpdateEntry_GroutEntryIsIdempotent(t *testing.T) {
	gl := New()
	info := map[string]string{
		NameElement: GroutEntryGameListName,
		PathElement: "./Grout.sh",
		DescElement: "Download games wirelessly from your RomM instance",
	}

	gl.AddOrUpdateEntry(GroutEntryGameListName, info)
	gl.AddOrUpdateEntry(GroutEntryGameListName, info)

	got := gameElements(t, gl)
	if len(got) != 1 {
		t.Fatalf("expected 1 Grout entry, got %d: %v", len(got), got)
	}
	if !strings.EqualFold(got[0], GroutEntryGameListName) {
		t.Errorf("expected %q, got %q", GroutEntryGameListName, got[0])
	}
}
