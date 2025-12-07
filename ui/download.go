package ui

import (
	"encoding/base64"
	"grout/models"
	"grout/utils"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/brandonkowalski/go-romm"
)

// DownloadInput contains data needed to render the download screen
type DownloadInput struct {
	Config        models.Config
	Host          models.Host
	Platform      romm.Platform
	SelectedGames []romm.DetailedRom
	AllGames      []romm.DetailedRom
	SearchFilter  string
}

// DownloadOutput contains the result of the download screen
type DownloadOutput struct {
	DownloadedGames []romm.DetailedRom
	Platform        romm.Platform
	AllGames        []romm.DetailedRom
	SearchFilter    string
}

// DownloadScreen handles downloading selected games
type DownloadScreen struct{}

func NewDownloadScreen() *DownloadScreen {
	return &DownloadScreen{}
}

func (s *DownloadScreen) Draw(input DownloadInput) (gaba.ScreenResult[DownloadOutput], error) {
	logger := gaba.GetLogger()

	output := DownloadOutput{
		Platform:     input.Platform,
		AllGames:     input.AllGames,
		SearchFilter: input.SearchFilter,
	}

	downloads := s.buildDownloads(input.Config, input.Host, input.Platform, input.SelectedGames)

	headers := make(map[string]string)
	auth := input.Host.Username + ":" + input.Host.Password
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	headers["Authorization"] = authHeader

	logger.Debug("RomM Auth Header", "header", authHeader)

	slices.SortFunc(downloads, func(a, b gaba.Download) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	logger.Debug("Starting ROM download", "downloads", downloads)

	res, err := gaba.DownloadManager(downloads, headers, input.Config.DownloadArt)
	if err != nil {
		logger.Error("Error downloading", "error", err)
		return gaba.WithCode(output, gaba.ExitCodeError), err
	}

	if len(res.Failed) > 0 {
		for _, g := range downloads {
			failedMatch := slices.ContainsFunc(res.Failed, func(de gaba.DownloadError) bool {
				return de.Download.DisplayName == g.DisplayName
			})
			if failedMatch {
				utils.DeleteFile(g.Location)
			}
		}
	}

	// No successful downloads
	if len(res.Completed) == 0 {
		return gaba.WithCode(output, gaba.ExitCodeError), nil
	}

	// Build list of successfully downloaded games
	downloadedGames := make([]romm.DetailedRom, 0, len(res.Completed))
	for _, g := range input.SelectedGames {
		if slices.ContainsFunc(res.Completed, func(d gaba.Download) bool {
			return d.DisplayName == g.Name
		}) {
			downloadedGames = append(downloadedGames, g)
		}
	}

	output.DownloadedGames = downloadedGames
	return gaba.Success(output), nil
}

func (s *DownloadScreen) buildDownloads(config models.Config, host models.Host, platform romm.Platform, games []romm.DetailedRom) []gaba.Download {
	downloads := make([]gaba.Download, 0, len(games))

	for _, g := range games {
		romDirectory := utils.GetPlatformRomDirectory(config, platform)
		downloadLocation := ""

		sourceURL := ""

		if g.MultiFile {
			// TODO Fill this shit out
		} else {
			downloadLocation = filepath.Join(romDirectory, g.Files[0].FileName)
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", g.Files[0].FileName)
		}

		downloads = append(downloads, gaba.Download{
			URL:         sourceURL,
			Location:    downloadLocation,
			DisplayName: g.Name,
			Timeout:     config.DownloadTimeout,
		})
	}

	return downloads
}
