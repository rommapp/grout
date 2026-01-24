package knulli

import (
	"fmt"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/internal/gamelist"
	"grout/romm"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type ArtDownload interface {
	GetLocation() string
	GetGameName() string
	GetURL() string
}

type GameListEntry struct {
	GL   *gamelist.GameList
	Path string
}

type GameListEntryInput struct {
	ArtLocation    string
	GamePath       string
	PlatformFSSlug string
}

func AddGamesToGamelist(config internal.Config, downloadedGames []romm.Rom, artDownloads []ArtDownload) error {
	gamelists := make(map[string]GameListEntry)

	for _, game := range downloadedGames {
		entry, exists := gamelists[game.PlatformFSSlug]
		if !exists {
			gl := gamelist.New()
			platform := romm.Platform{
				ID:     game.PlatformID,
				FSSlug: game.PlatformFSSlug,
				Name:   game.PlatformDisplayName,
			}

			romDirectory := config.GetPlatformRomDirectory(platform)
			gamelistPath := filepath.Join(romDirectory, "gamelist.xml")
			if fileutil.FileExists(gamelistPath) {
				data, err := os.ReadFile(gamelistPath)
				if err != nil {
					gaba.GetLogger().Debug("Error reading gamelist.xml file", "error", err)
				}
				if len(data) > 0 {
					gaba.GetLogger().Debug("Found gamelist.xml file", "data", string(data))
					if err := gl.Parse(data); err != nil {
						gaba.GetLogger().Error("Knulli gamelist.xml not found or can't be parsed, skipping platform", "path", gamelistPath, "error", err)
						continue
					}
				} else {
					gaba.GetLogger().Debug("gamelist.xml file is empty", "path", gamelistPath)
				}
			}
			gamelists[game.PlatformFSSlug] = GameListEntry{GL: gl, Path: gamelistPath}
		}

		gameMetadata := make(map[string]string)
		gameMetadata[gamelist.NameElement] = game.DisplayName
		gameMetadata[gamelist.DescElement] = game.Summary
		gameMetadata[gamelist.MD5Element] = game.Md5Hash
		if game.Metadatum.AverageRating != 0 {
			gameMetadata[gamelist.RatingElement] = fmt.Sprintf("%.1f", game.Metadatum.AverageRating/100*5)
		}

		if game.Metadatum.FirstReleaseDate != 0 {
			t := time.Unix(game.Metadatum.FirstReleaseDate/1000, 0).UTC()
			formatted := t.Format("20060102T150405")
			gameMetadata[gamelist.ReleaseDateElement] = fmt.Sprintf("%s", formatted)
		}

		// gameMetadata[gamelist.ImageElement] = ???
		// TODO: where is it mapped locally ? GetBaseRomsFolder + romName
		gameMetadata[gamelist.PathElement] = fmt.Sprintf(GetRomDirectory())

		maxPlayers := game.MaxPlayerCount()
		if maxPlayers > 1 {
			gameMetadata[gamelist.PlayersElement] = fmt.Sprintf("1-%d", maxPlayers)
		} else {
			gameMetadata[gamelist.PlayersElement] = "1"
		}

		if len(game.Regions) > 0 {
			gameMetadata[gamelist.RegionElement] = strings.Join(game.Regions, ", ")
		}

		if len(game.Languages) > 0 {
			gameMetadata[gamelist.LangElement] = strings.Join(game.Languages, ", ")
		}

		if len(game.Metadatum.Genres) > 0 {
			gameMetadata[gamelist.GenreElement] = strings.Join(game.Metadatum.Genres, ", ")
		}

		if len(game.Metadatum.Companies) > 0 {
			gameMetadata[gamelist.DeveloperElement] = strings.Join(game.Metadatum.Companies, ", ")
			//gameMetadata[gamelist.PublisherElement] = game.Metadatum.Companies[0]
		}

		entry.GL.AdddOrUpdateEntry(game.Name, gameMetadata)
	}

	for _, entry := range gamelists {
		if err := entry.GL.Save(entry.Path); err != nil {
			gaba.GetLogger().Error("Unable to save gamelist.xml file", "error", err)
			return err
		}
		gaba.GetLogger().Debug("Successfully saved gamelist.xml file", "path", entry.Path)
	}

	return nil
}
