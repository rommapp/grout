package emulationstation

import (
	"strconv"
	"strings"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist/entry"
	"github.com/rommapp/grout/core/gamelist/xmlmerge"
)

const FileName = "gamelist.xml"

const dateLayout = "20060102T150405"

var art = []struct {
	name string
	kind firmware.ArtKind
}{
	{"image", firmware.ArtCover},
	{"thumbnail", firmware.ArtThumbnail},
	{"marquee", firmware.ArtMarquee},
	{"video", firmware.ArtVideo},
	{"manual", firmware.ArtManual},
	{"bezel", firmware.ArtBezel},
	{"boxback", firmware.ArtBoxback},
	{"fanart", firmware.ArtFanart},
}

func Owned() []string {
	owned := []string{"name", "desc", "path", "releasedate", "rating", "players", "genre", "developer", "region", "lang", "md5"}
	for _, a := range art {
		owned = append(owned, a.name)
	}
	return owned
}

func Merge(existing []byte, entries []entry.Entry) ([]byte, bool, error) {
	games := make([]xmlmerge.Game, 0, len(entries))
	for _, e := range entries {
		games = append(games, game(e))
	}
	return xmlmerge.Merge(existing, Owned(), games)
}

func game(e entry.Entry) xmlmerge.Game {
	var releaseDate, rating string
	if !e.ReleaseDate.IsZero() {
		releaseDate = e.ReleaseDate.UTC().Format(dateLayout)
	}
	if e.HasRating {
		rating = strconv.FormatFloat(e.Rating/100, 'f', -1, 64)
	}
	elements := []xmlmerge.Element{
		{Name: "name", Value: e.Name},
		{Name: "desc", Value: e.Desc},
		{Name: "releasedate", Value: releaseDate},
		{Name: "rating", Value: rating},
		{Name: "players", Value: e.Players},
		{Name: "genre", Value: strings.Join(e.Genres, ", ")},
		{Name: "developer", Value: strings.Join(e.Developers, ", ")},
		{Name: "region", Value: strings.Join(e.Regions, ", ")},
		{Name: "lang", Value: strings.Join(e.Languages, ", ")},
		{Name: "md5", Value: e.MD5},
	}
	for _, a := range art {
		elements = append(elements, xmlmerge.Element{Name: a.name, Value: e.Art[a.kind]})
	}
	return xmlmerge.Game{Path: e.Path, Elements: elements}
}
