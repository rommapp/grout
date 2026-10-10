package gamelist

import (
	"errors"
	"fmt"
	"grout/files"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"grout/library"

	"github.com/beevik/etree"
)

type GameListEntry struct {
	GL   *GameList
	Path string
	// count is how many entries were applied to GL.
	count int
}

// RomGameEntry is one game to write into a gamelist. It carries library.Game
// rather than the wire type so this package performs no text transformation:
// the display name arrives rendered, and identity is the file name.
type RomGameEntry struct {
	Game         library.Game
	Platform     library.Platform
	RomDirectory string
}

func (e RomGameEntry) FileName() string {
	if e.Game.FileName != "" {
		return e.Game.FileName
	}
	if e.Game.Path == "" {
		return ""
	}
	return filepath.Base(e.Game.Path)
}

func (gl *GameList) AddRomGame(entry RomGameEntry) {
	metadata := romMetadata(entry.Game)
	// A fresh download has nothing better to show, so an unknown player count
	// reads as single player.
	if _, known := metadata[PlayersElement]; !known {
		metadata[PlayersElement] = "1"
	}

	element := gl.AddOrUpdateRomEntry(entry.FileName(), metadata)
	setScraperID(element, entry.Game)
}

// RefreshRomGame rewrites the metadata grout manages on a rom's entry, creating
// the entry when there is none.
//
// Only the elements grout writes are touched: what the firmware records on its
// own, such as play count, play time or favourites, is left as it is.
// Values the server has nothing for are skipped rather than blanking what
// a scraper may have filled in.
func (gl *GameList) RefreshRomGame(entry RomGameEntry) {
	metadata := romMetadata(entry.Game)
	for element, value := range metadata {
		if value == "" {
			delete(metadata, element)
		}
	}

	game := gl.findGame(byAnyFileName(entry.FileName(), filepath.Base(entry.Game.Path)))
	if game == nil {
		setScraperID(gl.AddGameEntry(metadata), entry.Game)
		return
	}

	delete(metadata, PathElement)
	for element, value := range metadata {
		setChild(game, element, value)
	}
	setScraperID(game, entry.Game)
}

// romMetadata is every element grout writes for a game, keyed by element name.
func romMetadata(game library.Game) map[string]string {
	gameMetadata := map[string]string{
		NameElement: game.DisplayName,
		DescElement: game.Summary,
		MD5Element:  game.MD5,
	}

	if game.HasRating() {
		gameMetadata[RatingElement] = fmt.Sprintf("%.1f", game.Rating)
	}

	if game.HasReleaseDate() {
		gameMetadata[ReleaseDateElement] = game.ReleaseDate.Format("20060102T150405")
	}

	for element, path := range map[string]string{
		ImageElement:     game.Art.Cover,
		ThumbnailElement: game.Art.Thumbnail,
		MarqueeElement:   game.Art.Marquee,
		VideoElement:     game.Art.Video,
		BezelElement:     game.Art.Bezel,
		ManualElement:    game.Art.Manual,
		BoxbackElement:   game.Art.BoxBack,
		FanartElement:    game.Art.Fanart,
		PathElement:      game.Path,
	} {
		if path != "" {
			gameMetadata[element] = path
		}
	}

	switch {
	case game.MaxPlayers > 1:
		gameMetadata[PlayersElement] = fmt.Sprintf("1-%d", game.MaxPlayers)
	case game.HasMaxPlayers():
		gameMetadata[PlayersElement] = "1"
	}

	for element, values := range map[string][]string{
		RegionElement:    game.Regions,
		LangElement:      game.Languages,
		GenreElement:     game.Genres,
		DeveloperElement: game.Developers,
	} {
		if len(values) > 0 {
			gameMetadata[element] = strings.Join(values, ", ")
		}
	}

	if game.ScreenScraperID > 0 {
		gameMetadata[ScraperIDElement] = strconv.Itoa(game.ScreenScraperID)
	}

	if game.RetroAchievementsID > 0 {
		gameMetadata[CheevosIDElement] = strconv.Itoa(game.RetroAchievementsID)
	}

	if game.RetroAchievementsHash != "" {
		gameMetadata[CheevosHashElement] = game.RetroAchievementsHash
	}

	return gameMetadata
}

// setScraperID mirrors the ScreenScraper id onto the entry's id attribute,
// which needs the element to exist and so follows the upsert.
func setScraperID(element *etree.Element, game library.Game) {
	if element != nil && game.ScreenScraperID > 0 {
		element.CreateAttr("id", strconv.Itoa(game.ScreenScraperID))
	}
}

// AddRomGamesToGamelist adds or updates the entries of freshly downloaded games.
func AddRomGamesToGamelist(entries []RomGameEntry, gamelistFilename FileName) error {
	_, err := applyToGamelists(entries, gamelistFilename, (*GameList).AddRomGame)
	return err
}

// RefreshRomGamesInGamelist brings the entries of games already on the device
// in line with the server, keeping what the frontend recorded about them. See
// RefreshRomGame. It returns how many entries were written.
func RefreshRomGamesInGamelist(entries []RomGameEntry, gamelistFilename FileName) (int, error) {
	return applyToGamelists(entries, gamelistFilename, (*GameList).RefreshRomGame)
}

// applyToGamelists loads each gamelist file the entries belong to once,
// applies apply to every entry, then saves each file.
//
// Entries are grouped by file rather than by platform: several platforms can
// share a ROM directory, such as nes and famicom, and loading the file once per
// platform would have the last save discard the others' entries.
//
// A file that cannot be loaded or saved does not stop the others. It returns
// how many entries landed in a saved file, with every failure joined.
func applyToGamelists(entries []RomGameEntry, gamelistFilename FileName, apply func(*GameList, RomGameEntry)) (int, error) {
	logger := slog.Default()

	gamelists := make(map[string]*GameListEntry)
	// Files that could not be loaded, so each is tried only once.
	skipped := make(map[string]bool)
	var errs []error

	for _, game := range entries {
		gamelistPath := filepath.Join(game.RomDirectory, string(gamelistFilename))
		if skipped[gamelistPath] {
			continue
		}

		glEntry, exists := gamelists[gamelistPath]
		if !exists {
			gl, err := loadGamelist(gamelistPath)
			if err != nil {
				// Saving over a file that could not be read would wipe every
				// entry in it.
				logger.Error("Unable to load gamelist file, skipping its entries", "error", err, "path", gamelistPath)
				errs = append(errs, err)
				skipped[gamelistPath] = true
				continue
			}
			glEntry = &GameListEntry{Path: gamelistPath, GL: gl}
			gamelists[gamelistPath] = glEntry
		}

		apply(glEntry.GL, game)
		glEntry.count++
	}

	written := 0
	for _, glEntry := range gamelists {
		if err := glEntry.GL.Save(glEntry.Path); err != nil {
			logger.Error("Unable to save gamelist file", "error", err, "path", glEntry.Path)
			errs = append(errs, fmt.Errorf("saving %s: %w", glEntry.Path, err))
			continue
		}
		logger.Debug("Successfully saved gamelist file", "path", glEntry.Path)
		written += glEntry.count
	}

	return written, errors.Join(errs...)
}

// loadGamelist reads the gamelist at path, or starts an empty one when there is
// no file yet.
func loadGamelist(path string) (*GameList, error) {
	gl := New()
	if !files.FileExists(path) {
		return gl, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(data) > 0 {
		if err := gl.Parse(data); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	return gl, nil
}
