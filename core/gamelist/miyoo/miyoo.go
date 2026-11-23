package miyoo

import (
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist/entry"
	"github.com/rommapp/grout/core/gamelist/xmlmerge"
)

const FileName = "miyoogamelist.xml"

func Owned() []string {
	return []string{"name", "path", "image"}
}

func Merge(existing []byte, entries []entry.Entry) ([]byte, bool, error) {
	games := make([]xmlmerge.Game, 0, len(entries))
	for _, e := range entries {
		games = append(games, xmlmerge.Game{
			Path: e.Path,
			Elements: []xmlmerge.Element{
				{Name: "name", Value: e.Name},
				{Name: "image", Value: e.Art[firmware.ArtCover]},
			},
		})
	}
	return xmlmerge.Merge(existing, Owned(), games)
}
