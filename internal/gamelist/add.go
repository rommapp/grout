package gamelist

import (
	"fmt"
	"grout/internal/fileutil"
	"grout/internal/stringutil"
	"grout/romm"
	"os"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type GameListEntry struct {
	GL   *GameList
	Path string
}

type RomGameEntry struct {
	Game         *romm.Rom
	ArtLocation  string
	GamePath     string
	RomDirectory string
	Platform     *romm.Platform
}

func (gl *GameList) AddRomGame(entry RomGameEntry) {
	gameMetadata := make(map[string]string)
	gameMetadata[NameElement] = stringutil.PrepareRomName(entry.Game.Name, entry.Game.Regions)
	gameMetadata[DescElement] = entry.Game.Summary
	gameMetadata[MD5Element] = entry.Game.Md5Hash
	if entry.Game.Metadatum.AverageRating != 0 {
		gameMetadata[RatingElement] = fmt.Sprintf("%.1f", entry.Game.Metadatum.AverageRating/100)
	}

	if entry.Game.Metadatum.FirstReleaseDate != 0 {
		t := time.Unix(entry.Game.Metadatum.FirstReleaseDate/1000, 0).UTC()
		formatted := t.Format("20060102T150405")
		gameMetadata[ReleaseDateElement] = fmt.Sprintf("%s", formatted)
	}

	if entry.ArtLocation != "" {
		gameMetadata[ImageElement] = entry.ArtLocation
	}

	if entry.GamePath != "" {
		gameMetadata[PathElement] = entry.GamePath
	}

	maxPlayers := entry.Game.MaxPlayerCount()
	if maxPlayers > 1 {
		gameMetadata[PlayersElement] = fmt.Sprintf("1-%d", maxPlayers)
	} else {
		gameMetadata[PlayersElement] = "1"
	}

	if len(entry.Game.Regions) > 0 {
		gameMetadata[RegionElement] = strings.Join(entry.Game.Regions, ", ")
	}

	if len(entry.Game.Languages) > 0 {
		gameMetadata[LangElement] = strings.Join(entry.Game.Languages, ", ")
	}

	if len(entry.Game.Metadatum.Genres) > 0 {
		gameMetadata[GenreElement] = strings.Join(entry.Game.Metadatum.Genres, ", ")
	}

	if len(entry.Game.Metadatum.Companies) > 0 {
		gameMetadata[DeveloperElement] = strings.Join(entry.Game.Metadatum.Companies, ", ")
	} else if entry.Game.ScreenScraperMetadata.Companies != nil && len(entry.Game.ScreenScraperMetadata.Companies) > 0 {
		gameMetadata[DeveloperElement] = strings.Join(entry.Game.ScreenScraperMetadata.Companies, ", ")
	}

	gl.AdddOrUpdateEntry(entry.Game.Name, gameMetadata)
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
					gaba.GetLogger().Debug("Found gamelist file", "path", gamelistPath, "data", string(data))
					if err := gl.Parse(data); err != nil {
						gaba.GetLogger().Error("gamelist not found or can't be parsed, skipping platform", "path", gamelistPath, "error", err)
						continue
					} else {
						gaba.GetLogger().Debug("Successfully parsed gamelist file", "path", gamelistPath, "data", string(data))
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
