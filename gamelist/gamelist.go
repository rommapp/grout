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

// root returns the <gameList> element, or nil. A file written by another tool
// may have any root, so callers must tolerate its absence.
func (gl *GameList) root() *etree.Element {
	if gl.document == nil {
		return nil
	}
	return gl.document.SelectElement(GameListElement)
}

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

// byFileName matches an entry on the rom's file name from <path>, the stable
// identity of a rom entry. <name> is a display string and changes with regions,
// locale and settings.
//
// Base names are compared so grout adopts entries written by EmulationStation,
// which stores "./Game.gba" where grout stores an absolute path.
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

// upsert updates the entry matched by match, creating it if absent. Returns the
// element so callers can set attributes, which requires it to exist.
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

// AddOrUpdateEntry upserts an entry keyed by <name>, for non-rom entries such
// as the Grout launcher shortcut. Rom entries must use AddOrUpdateRomEntry.
func (gl *GameList) AddOrUpdateEntry(name string, info map[string]string) *etree.Element {
	return gl.upsert(byName(name), info)
}

func (gl *GameList) AddOrUpdateRomEntry(fileName string, info map[string]string) *etree.Element {
	if fileName == "" {
		return gl.AddGameEntry(info)
	}
	return gl.upsert(byFileName(fileName), info)
}
