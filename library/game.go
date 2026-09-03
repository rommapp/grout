// Package library holds the game and platform types grout works with once a
// response has left the RomM client.
//
// These are not the RomM wire types. Identity is FileName, the rom's name on
// disk, and never depends on how the game is displayed. DisplayName arrives
// already rendered; nothing downstream transforms it further.
package library

import "time"

// Platform is a console as it exists on this device.
type Platform struct {
	// FSSlug is RomM's filesystem slug ("gba"), the key for directory mappings
	// and platform tables.
	FSSlug string
	Name   string
}

// ArtPaths records where artwork was written. An empty field means that kind
// was not downloaded.
type ArtPaths struct {
	Cover     string
	Thumbnail string
	Marquee   string
	Video     string
	Bezel     string
	Manual    string
	BoxBack   string
	Fanart    string
}

// Game is what grout knows about one game, in the form the metadata writers
// need.
type Game struct {
	// FileName is the game's identity: gamelist entries, artwork and saves are
	// all matched on it.
	FileName string
	// BaseName is FileName without its extension.
	BaseName string
	// Path is empty when the rom is not installed.
	Path string

	// DisplayName is already rendered. Never use it as a key.
	DisplayName string
	Summary     string

	Regions    []string
	Languages  []string
	Genres     []string
	Developers []string

	// Zero when unknown.
	ReleaseDate time.Time
	// Rating is 0 when unknown, otherwise 0..1.
	Rating float64
	// At least 1.
	MaxPlayers int

	MD5                   string
	ScreenScraperID       int
	RetroAchievementsID   int
	RetroAchievementsHash string

	Art ArtPaths
}

// HasRating distinguishes unrated from a genuine zero.
func (g Game) HasRating() bool { return g.Rating > 0 }

func (g Game) HasReleaseDate() bool { return !g.ReleaseDate.IsZero() }
