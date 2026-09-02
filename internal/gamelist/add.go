package gamelist

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"grout/domain/library"
	"grout/internal/fileutil"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type GameListEntry struct {
	GL   *GameList
	Path string
}

// RomGameEntry is one game to write into a gamelist.
//
// It carries library.Game rather than the RomM wire type so that this package
// performs no text transformation of its own: the display name arrives already
// rendered, and identity is the game's file name. When AddRomGame derived the
// name itself, the written <name> and the key used to find the entry again
// disagreed for every region-tagged game.
type RomGameEntry struct {
	Game         library.Game
	Platform     library.Platform
	RomDirectory string
}

// FileName is the stable identity of this entry: the rom's file name on disk.
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
	game := entry.Game

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

	if game.MaxPlayers > 1 {
		gameMetadata[PlayersElement] = fmt.Sprintf("1-%d", game.MaxPlayers)
	} else {
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

	element := gl.AddOrUpdateRomEntry(entry.FileName(), gameMetadata)

	// The id attribute can only be written once the element exists, so this
	// must follow the upsert rather than precede it.
	if element != nil && game.ScreenScraperID > 0 {
		element.CreateAttr("id", strconv.Itoa(game.ScreenScraperID))
	}
}

func AddRomGamesToGamelist(entry []RomGameEntry, gamelistFilename FileName) error {
	gamelists := make(map[string]GameListEntry)
	for _, game := range entry {
		glEntry, exists := gamelists[game.Platform.FSSlug]
		if !exists {
			gl := New()
			gamelistPath := fmt.Sprintf("%s/%s", game.RomDirectory, gamelistFilename)
			if fileutil.FileExists(gamelistPath) {
				data, err := os.ReadFile(gamelistPath)
				if err != nil {
					gaba.GetLogger().Debug("Error reading gamelist file", "error", err, "path", gamelistPath)
				}
				if len(data) > 0 {
					if err := gl.Parse(data); err != nil {
						gaba.GetLogger().Error("gamelist not found or can't be parsed, skipping platform", "path", gamelistPath, "error", err)
						continue
					}
				}
			}
			glEntry = GameListEntry{Path: gamelistPath, GL: gl}
			gamelists[game.Platform.FSSlug] = glEntry
		}

		glEntry.GL.AddRomGame(game)
	}

	for _, glEntry := range gamelists {
		if err := glEntry.GL.Save(glEntry.Path); err != nil {
			gaba.GetLogger().Error("Unable to save gamelist file", "error", err, "path", glEntry.Path)
			return err
		}
		gaba.GetLogger().Debug("Successfully saved gamelist file", "path", glEntry.Path)
	}

	return nil
}
