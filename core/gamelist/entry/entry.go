package entry

import (
	"time"

	"github.com/rommapp/grout/core/firmware"
)

type Entry struct {
	Path        string
	Name        string
	Desc        string
	ReleaseDate time.Time
	Rating      float64
	HasRating   bool
	Players     string
	Genres      []string
	Developers  []string
	Regions     []string
	Languages   []string
	MD5         string
	Art         map[firmware.ArtKind]string
}
