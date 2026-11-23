package xmlmerge

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/beevik/etree"
)

type Element struct {
	Name  string
	Value string
}

type Game struct {
	Path     string
	Elements []Element
}

var ErrUnparseable = errors.New("gamelist cannot be parsed")

const (
	rootTag = "gameList"
	gameTag = "game"
	pathTag = "path"
)

func Merge(existing []byte, owned []string, games []Game) (out []byte, changed bool, err error) {
	doc := etree.NewDocument()
	fresh := len(bytes.TrimSpace(existing)) == 0
	if fresh {
		doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
		doc.CreateElement(rootTag)
	} else if err := doc.ReadFromBytes(existing); err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrUnparseable, err)
	}
	root := doc.Root()
	if root == nil || root.Tag != rootTag || root.Space != "" {
		return nil, false, fmt.Errorf("%w: no %s root", ErrUnparseable, rootTag)
	}

	dirty := fresh
	for _, g := range games {
		if mergeGame(root, owned, g) {
			dirty = true
		}
	}
	if !dirty {
		return existing, false, nil
	}

	indent := etree.NewIndentSettings()
	indent.Spaces = 4
	indent.PreserveLeafWhitespace = true
	doc.IndentWithSettings(indent)
	out, err = doc.WriteToBytes()
	if err != nil {
		return nil, false, err
	}
	if !fresh && bytes.Equal(out, existing) {
		return existing, false, nil
	}
	return out, true, nil
}

func mergeGame(root *etree.Element, owned []string, g Game) bool {
	game := findGame(root, g.Path)
	if game == nil {
		game = root.CreateElement(gameTag)
		game.CreateElement(pathTag).SetText(g.Path)
		setElements(game, owned, g.Elements)
		return true
	}
	return setElements(game, owned, g.Elements)
}

func setElements(game *etree.Element, owned []string, elements []Element) bool {
	dirty := false
	for _, e := range elements {
		if e.Value == "" || e.Name == pathTag || !slices.Contains(owned, e.Name) {
			continue
		}
		child := game.SelectElement(e.Name)
		if child == nil {
			game.CreateElement(e.Name).SetText(e.Value)
			dirty = true
			continue
		}
		if child.Text() != e.Value {
			child.SetText(e.Value)
			dirty = true
		}
	}
	return dirty
}

func findGame(root *etree.Element, gamePath string) *etree.Element {
	base, ok := baseName(gamePath)
	if !ok {
		return nil
	}
	exact := path.Clean(strings.TrimSpace(gamePath))
	var byBase *etree.Element
	for _, game := range root.SelectElements(gameTag) {
		p := game.SelectElement(pathTag)
		if p == nil {
			continue
		}
		if path.Clean(strings.TrimSpace(p.Text())) == exact {
			return game
		}
		if b, ok := baseName(p.Text()); ok && b == base && byBase == nil {
			byBase = game
		}
	}
	return byBase
}

func baseName(p string) (string, bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", false
	}
	b := path.Base(p)
	return b, b != "." && b != "/"
}
