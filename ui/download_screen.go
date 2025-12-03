package ui

import (
	"encoding/base64"
	"grout/client"
	"grout/models"
	"grout/state"
	"grout/utils"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
	"qlova.tech/sum"
)

type DownloadScreen struct {
	Platform      models.Platform
	Games         shared.Items
	SelectedGames shared.Items
	SearchFilter  string
}

func InitDownloadScreen(platform models.Platform, games shared.Items, selectedGames shared.Items, searchFilter string) DownloadScreen {
	return DownloadScreen{
		Platform:      platform,
		Games:         games,
		SelectedGames: selectedGames,
		SearchFilter:  searchFilter,
	}
}

func (d DownloadScreen) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.Download
}

func (d DownloadScreen) Draw() (value interface{}, exitCode int, e error) {
	logger := gaba.GetLogger()

	downloads := BuildDownload(d.Platform, d.SelectedGames)

	headers := make(map[string]string)

	auth := d.Platform.Host.Username + ":" + d.Platform.Host.Password
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	headers["Authorization"] = authHeader

	logger.Debug("RomM Auth Header", "header", authHeader)

	slices.SortFunc(downloads, func(a, b gaba.Download) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	logger.Debug("Starting ROM download", "downloads", downloads)

	res, err := gaba.DownloadManager(downloads, headers, state.GetAppState().Config.DownloadArt)
	if err != nil {
		logger.Error("Error downloading", "error", err)
		return nil, -1, err
	}

	if len(res.FailedDownloads) > 0 {
		for _, g := range downloads {
			if slices.Contains(res.FailedDownloads, g) {
				common.DeleteFile(g.Location)
			}
		}
	}

	exitCode = 0

	if len(res.CompletedDownloads) == 0 {
		exitCode = 1
	}

	var downloadedGames []shared.Item

	for _, g := range d.Games {
		if slices.ContainsFunc(res.CompletedDownloads, func(d gaba.Download) bool {
			return d.DisplayName == g.DisplayName
		}) {
			downloadedGames = append(downloadedGames, g)
		}
	}

	return downloadedGames, exitCode, err
}

func BuildDownload(platform models.Platform, games shared.Items) []gaba.Download {
	config := state.GetAppState().Config

	var downloads []gaba.Download
	for _, g := range games {

		filename := g.Filename

		if config.UseTitleAsFilename {
			filename = g.DisplayName + filepath.Ext(g.Filename)
		}

		romDirectory := utils.GetPlatformRomDirectory(platform)
		downloadLocation := filepath.Join(romDirectory, filename)

		root := platform.Host.RootURI

		if platform.Host.Port != 0 {
			root = root + ":" + strconv.Itoa(platform.Host.Port)
		}

		var sourceURL string

		rc := client.NewRomMClient(platform.Host)
		sourceURL, _ = rc.BuildDownloadURL(g.RomID, g.Filename)

		downloads = append(downloads, gaba.Download{
			URL:         sourceURL,
			Location:    downloadLocation,
			DisplayName: g.DisplayName,
		})
	}

	return downloads
}
