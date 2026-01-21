package knulli

import (
	"grout/internal/fileutil"
	"grout/internal/gamelist"
	"grout/romm"
	"os"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type ArtDownload interface {
	GetLocation() string
	GetGameName() string
	GetURL() string
}

func AddGamesToGamelist(downloadedGames []romm.Rom, artDownloads []ArtDownload) error {
	gamelists := make(map[string]*gamelist.GameList)

	for _, game := range downloadedGames {
		gl, exists := gamelists[game.PlatformFSSlug]
		if !exists {
			gl = gamelist.New()
			gamelists[game.PlatformFSSlug] = gl
			gamelistPath := filepath.Base(game.FsPath)
			if fileutil.FileExists(gamelistPath) {
				data, err := os.ReadFile(gamelistPath)
				if err != nil {
					gaba.GetLogger().Debug("Error reading gamelist.xml file", "error", err)
				}

				if len(data) > 0 {
					gaba.GetLogger().Debug("Found gamelist.xml file", "data", string(data))
					if err := gl.Parse(data); err != nil {
						gaba.GetLogger().Debug("Knulli gamelist.xml not found or can't be parsed, skipping grout entry check", "path", gamelistPath, "error", err)
						// TODO: erreur sentinelle
						return nil
					}
				} else {
					gaba.GetLogger().Debug("gamelist.xml file is empty", "path", gamelistPath)
				}
			}
		}

		gameMetadata := make(map[string]string)
		gameMetadata[gamelist.NameElement] = game.DisplayName
		gameMetadata[gamelist.DescElement] = game.Summary
		// TODO: where is it mapped locally ? GetBaseRomsFolder + romName
		//gameMetadata[gamelist.PathElement] = fmt.Sprintf(game.FsPath)
		//gameMetadata[gamelist.PlayersElement] =
		if len(game.Metadatum.Companies) > 0 {
			// TODO: map each source metadata to find a company filled
			gameMetadata[gamelist.DeveloperElement] = game.Metadatum.Companies[0]
			gameMetadata[gamelist.PublisherElement] = game.Metadatum.Companies[0]
		}

		gl.AdddOrUpdateEntry(game.Name, gameMetadata)
	}

	return nil
}
