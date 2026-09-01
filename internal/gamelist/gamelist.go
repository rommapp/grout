package gamelist

import (
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

const (
	NameElement        = "name"
	DescElement        = "desc"
	ImageElement       = "image"
	PlayersElement     = "players"
	GenreElement       = "genre"
	PathElement        = "path"
	GameListElement    = "gameList"
	GameElement        = "game"
	ReleaseDateElement = "releasedate"
	DeveloperElement   = "developer"
	PublisherElement   = "publisher"
	RatingElement      = "rating"
	MD5Element         = "md5"
	VideoElement       = "video"
	MarqueeElement     = "marquee"
	BezelElement       = "bezel"
	ManualElement      = "manual"
	FanartElement      = "fanart"
	BoxbackElement     = "boxback"
	ThumbnailElement   = "thumbnail"
	LangElement        = "lang"
	RegionElement      = "region"
	CheevosHashElement = "cheevosHash"
	CheevosIDElement   = "cheevosId"
	ScraperIDElement   = "scraperId"
)

type FileName string

const (
	GameListFileName      FileName = "gamelist.xml"
	MiyooGameListFileName FileName = "miyoogamelist.xml"
)

type GameList struct {
	document *etree.Document
}

func New() *GameList {
	return &GameList{
		document: emptyGameList(),
	}
}

func emptyGameList() *etree.Document {
	document := etree.NewDocument()
	document.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
	document.CreateElement(GameListElement)
	return document
}

func (gl *GameList) Parse(b []byte) error {
	document := etree.NewDocument()
	if err := document.ReadFromBytes(b); err != nil {
		return err
	}

	gl.document = document
	return nil
}

// root returns the <gameList> element, or nil if this document does not have
// one. A gamelist file written by another tool may have any root element, so
// callers must tolerate its absence rather than dereferencing blindly.
func (gl *GameList) root() *etree.Element {
	if gl.document == nil {
		return nil
	}
	return gl.document.SelectElement(GameListElement)
}

// gameMatcher reports whether a <game> element is the one being looked for.
type gameMatcher func(*etree.Element) bool

func (gl *GameList) findGame(match gameMatcher) *etree.Element {
	root := gl.root()
	if root == nil {
		return nil
	}
	for _, game := range root.SelectElements(GameElement) {
		if match(game) {
			return game
		}
	}
	return nil
}

func byName(name string) gameMatcher {
	return func(game *etree.Element) bool {
		nameElement := game.FindElement(NameElement)
		return nameElement != nil && nameElement.Text() == name
	}
}

// byFileName matches an entry on the rom's file name, taken from <path>.
//
// This is the stable identity of a rom entry. <name> is a display string: it
// carries region suffixes, is rewritten by nameCleaner, and varies with user
// settings and locale, so keying on it appends a duplicate entry whenever the
// displayed name changes. Comparing base names also lets grout adopt entries
// written by EmulationStation, which stores "./Game.gba" where grout stores an
// absolute path.
func byFileName(fileName string) gameMatcher {
	return func(game *etree.Element) bool {
		pathElement := game.FindElement(PathElement)
		if pathElement == nil {
			return false
		}
		return filepath.Base(strings.TrimSpace(pathElement.Text())) == fileName
	}
}

func (gl *GameList) Contains(element, value string) bool {
	return gl.findGame(func(game *etree.Element) bool {
		e := game.FindElement(element)
		return e != nil && e.Text() == value
	}) != nil
}

func (gl *GameList) GetGameElementByName(name string) *etree.Element {
	return gl.findGame(byName(name))
}

// GetGameElementByFileName finds a rom entry by its file name on disk.
func (gl *GameList) GetGameElementByFileName(fileName string) *etree.Element {
	if fileName == "" {
		return nil
	}
	return gl.findGame(byFileName(fileName))
}

func (gl *GameList) GameContainsElements(name string, elements []string) bool {
	e := gl.GetGameElementByName(name)
	if e == nil {
		return false
	}
	for _, element := range elements {
		if e.FindElement(element) == nil {
			return false
		}
	}
	return true
}

func (gl *GameList) Save(filepath string) error {
	gl.document.Indent(4)
	if err := gl.document.WriteToFile(filepath); err != nil {
		return err
	}
	return nil
}

func (gl *GameList) AddGameEntry(info map[string]string) *etree.Element {
	root := gl.root()
	if root == nil {
		return nil
	}
	newGame := root.CreateElement(GameElement)

	for key, value := range info {
		newGame.CreateElement(key).SetText(value)
	}
	return newGame
}

// upsert updates the entry matched by match, creating it if absent. It returns
// the element so callers can set attributes on it — which must happen after
// the element exists, not before.
func (gl *GameList) upsert(match gameMatcher, info map[string]string) *etree.Element {
	game := gl.findGame(match)
	if game == nil {
		return gl.AddGameEntry(info)
	}

	for key, value := range info {
		if element := game.FindElement(key); element != nil {
			element.SetText(value)
		} else {
			game.CreateElement(key).SetText(value)
		}
	}
	return game
}

// AddOrUpdateEntry upserts an entry keyed by its <name>. This is for entries
// that are not roms — the Grout launcher shortcut — where the name is a fixed
// literal rather than a display string. Rom entries must use
// AddOrUpdateRomEntry, which keys on the file name.
func (gl *GameList) AddOrUpdateEntry(name string, info map[string]string) *etree.Element {
	return gl.upsert(byName(name), info)
}

// AddOrUpdateRomEntry upserts a rom entry keyed by its file name on disk.
func (gl *GameList) AddOrUpdateRomEntry(fileName string, info map[string]string) *etree.Element {
	if fileName == "" {
		return gl.AddGameEntry(info)
	}
	return gl.upsert(byFileName(fileName), info)
}
