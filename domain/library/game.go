// Package library holds the game and platform types grout works with once a
// response has left the RomM client.
//
// These are deliberately not the RomM wire types. Two differences matter:
//
//   - Identity is FileName, the rom's name on disk. It never depends on how the
//     game is displayed. Keying on a rendered name is what caused grout to
//     append a duplicate gamelist entry for every region-tagged game.
//   - DisplayName is already rendered. Nothing downstream applies further text
//     transformation, so a caller is free to decide how a name should read --
//     with or without the region, translated, or truncated -- without any of
//     that reaching identity.
package library

import "time"

// Platform is a console as it exists on this device.
type Platform struct {
	// FSSlug is RomM's filesystem slug, e.g. "gba". It is the key used for
	// directory mappings and platform tables.
	FSSlug string
	// Name is shown to a person.
	Name string
}

// ArtPaths records where a game's artwork was written on disk. An empty field
// means that kind of art was not downloaded.
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
	// FileName is the rom's file name on disk, including its extension. This
	// is the game's identity: gamelist entries, artwork files and save files
	// are all matched on it.
	FileName string
	// BaseName is FileName without its extension, used where a firmware names
	// a sidecar file after the rom.
	BaseName string
	// Path is the full path to the rom on disk, empty if it is not installed.
	Path string

	// DisplayName is the already-rendered name to show. Never use it as a key.
	DisplayName string
	Summary     string

	Regions    []string
	Languages  []string
	Genres     []string
	Developers []string

	// ReleaseDate is zero when unknown.
	ReleaseDate time.Time
	// Rating is 0 when unknown, otherwise between 0 and 1.
	Rating float64
	// MaxPlayers is at least 1.
	MaxPlayers int

	MD5                   string
	ScreenScraperID       int
	RetroAchievementsID   int
	RetroAchievementsHash string

	Art ArtPaths
}

// HasRating reports whether a rating is known, distinguishing "unrated" from a
// genuine zero.
func (g Game) HasRating() bool { return g.Rating > 0 }

// HasReleaseDate reports whether a release date is known.
func (g Game) HasReleaseDate() bool { return !g.ReleaseDate.IsZero() }
