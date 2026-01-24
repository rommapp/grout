package gamelist

import (
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
	ThumbnailElement   = "thumbnail"
	LangElement        = "lang"
	RegionElement      = "region"
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

func (gl *GameList) Contains(element, value string) bool {
	root := gl.document.SelectElement(GameListElement)
	games := root.SelectElements(GameElement)
	for _, game := range games {
		element := game.FindElement(element)
		if element != nil && element.Text() == value {
			return true
		}
	}
	return false
}

func (gl *GameList) Save(filepath string) error {
	gl.document.Indent(4)
	if err := gl.document.WriteToFile(filepath); err != nil {
		return err
	}
	return nil
}

func (gl *GameList) AddGameEntry(info map[string]string) {
	root := gl.document.SelectElement(GameListElement)
	newGame := root.CreateElement(GameElement)

	for key, value := range info {
		newGame.CreateElement(key).SetText(value)
	}
}

func (gl *GameList) AdddOrUpdateEntry(name string, info map[string]string) {
	root := gl.document.SelectElement(GameListElement)
	games := root.SelectElements(GameElement)
	for _, game := range games {
		nameElement := game.FindElement(NameElement)
		if nameElement != nil && nameElement.Text() == name {
			// Update existing entry
			for key, value := range info {
				if element := game.FindElement(key); element != nil {
					element.SetText(value)
				} else {
					game.CreateElement(key).SetText(value)
				}
			}
			return
		}
	}
	// Add new entry
	gl.AddGameEntry(info)
}
