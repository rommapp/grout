package ui

import (
	"errors"
	"fmt"
	"grout/constants"
	"grout/utils"
	_ "image/gif"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
)

type downloadInput struct {
	Config        utils.Config
	Host          romm.Host
	Platform      romm.Platform
	SelectedGames []romm.Rom
	AllGames      []romm.Rom
	SearchFilter  string
}

type downloadOutput struct {
	DownloadedGames []romm.Rom
	Platform        romm.Platform
	AllGames        []romm.Rom
	SearchFilter    string
}

type DownloadScreen struct{}

type artDownload struct {
	URL      string
	Location string
	GameName string
}

func NewDownloadScreen() *DownloadScreen {
	return &DownloadScreen{}
}

func (s *DownloadScreen) Execute(config utils.Config, host romm.Host, platform romm.Platform, selectedGames []romm.Rom, allGames []romm.Rom, searchFilter string) downloadOutput {
	result, err := s.draw(downloadInput{
		Config:        config,
		Host:          host,
		Platform:      platform,
		SelectedGames: selectedGames,
		AllGames:      allGames,
		SearchFilter:  searchFilter,
	})

	if err != nil {
		gaba.GetLogger().Error("Download failed", "error", err)
		return downloadOutput{
			AllGames:     allGames,
			Platform:     platform,
			SearchFilter: searchFilter,
		}
	}

	if result.ExitCode == gaba.ExitCodeSuccess && len(result.Value.DownloadedGames) > 0 {
		gaba.GetLogger().Debug("Successfully downloaded games", "count", len(result.Value.DownloadedGames))
	}

	return result.Value
}

func (s *DownloadScreen) draw(input downloadInput) (ScreenResult[downloadOutput], error) {
	logger := gaba.GetLogger()

	output := downloadOutput{
		Platform:     input.Platform,
		AllGames:     input.AllGames,
		SearchFilter: input.SearchFilter,
	}

	downloads, artDownloads := s.buildDownloads(input.Config, input.Host, input.Platform, input.SelectedGames)

	headers := make(map[string]string)
	headers["Authorization"] = input.Host.BasicAuthHeader()

	slices.SortFunc(downloads, func(a, b gaba.Download) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	logger.Debug("Starting ROM download", "downloads", downloads)

	res, err := gaba.DownloadManager(downloads, headers, gaba.DownloadManagerOptions{
		AutoContinue: input.Config.DownloadArt,
	})
	if err != nil {
		logger.Error("Error downloading", "error", err)

		// Clean up any partial downloads when cancelled
		if errors.Is(err, gaba.ErrCancelled) {
			for _, d := range downloads {
				utils.DeleteFile(d.Location)
			}
		}

		return withCode(output, gaba.ExitCodeError), err
	}

	logger.Debug("Download results", "completed", len(res.Completed), "failed", len(res.Failed))

	if len(res.Failed) > 0 {
		for _, f := range res.Failed {
			logger.Warn("Download failed", "name", f.Download.DisplayName, "url", f.Download.URL, "error", f.Error)
		}

		for _, g := range downloads {
			failedMatch := slices.ContainsFunc(res.Failed, func(de gaba.DownloadError) bool {
				return de.Download.DisplayName == g.DisplayName
			})
			if failedMatch {
				utils.DeleteFile(g.Location)
			}
		}
	}

	if len(res.Completed) == 0 {
		return withCode(output, gaba.ExitCodeError), nil
	}

	for _, g := range input.SelectedGames {
		if !g.HasMultipleFiles {
			continue
		}

		completed := slices.ContainsFunc(res.Completed, func(d gaba.Download) bool {
			return d.DisplayName == g.Name
		})
		if !completed {
			continue
		}

		gamePlatform := input.Platform
		if input.Platform.ID == 0 && g.PlatformID != 0 {
			gamePlatform = romm.Platform{
				ID:   g.PlatformID,
				Slug: g.PlatformSlug,
				Name: g.PlatformDisplayName,
			}
		}

		tmpZipPath := filepath.Join(utils.TempDir(), fmt.Sprintf("grout_multirom_%d.zip", g.ID))
		romDirectory := utils.GetPlatformRomDirectory(input.Config, gamePlatform)
		extractDir := filepath.Join(romDirectory, g.DisplayName)

		progress := &atomic.Float64{}
		_, err := gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "download_extracting", Other: "Extracting {{.Name}}..."}, map[string]interface{}{"Name": g.DisplayName}),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (interface{}, error) {
				logger.Debug("Extracting multi-file ROM", "game", g.DisplayName, "dest", extractDir)

				if err := utils.Unzip(tmpZipPath, extractDir, progress); err != nil {
					logger.Error("Failed to extract multi-file ROM", "game", g.DisplayName, "error", err)
					os.Remove(tmpZipPath)
					return nil, err
				}

				if utils.GetCFW() == constants.MuOS {
					if err := utils.OrganizeMultiFileRomForMuOS(extractDir, romDirectory, g.DisplayName); err != nil {
						logger.Error("Failed to organize multi-file ROM for muOS", "game", g.DisplayName, "error", err)
						os.Remove(tmpZipPath)
						os.RemoveAll(extractDir)
						return nil, err
					}
				}

				if err := os.Remove(tmpZipPath); err != nil {
					logger.Warn("Failed to remove temp zip file", "path", tmpZipPath, "error", err)
				}

				return nil, nil
			},
		)

		if err != nil {
			continue
		}
	}

	if input.Config.UnzipDownloads {
		for _, g := range input.SelectedGames {
			if g.HasMultipleFiles {
				continue
			}

			completed := slices.ContainsFunc(res.Completed, func(d gaba.Download) bool {
				return d.DisplayName == g.Name
			})
			if !completed {
				continue
			}

			gamePlatform := input.Platform
			if input.Platform.ID == 0 && g.PlatformID != 0 {
				gamePlatform = romm.Platform{
					ID:   g.PlatformID,
					Slug: g.PlatformSlug,
					Name: g.PlatformDisplayName,
				}
			}

			if len(g.Files) > 0 && strings.ToLower(filepath.Ext(g.Files[0].FileName)) == ".zip" {
				romDirectory := utils.GetPlatformRomDirectory(input.Config, gamePlatform)
				zipPath := filepath.Join(romDirectory, g.Files[0].FileName)

				progress := &atomic.Float64{}
				_, err := gaba.ProcessMessage(
					i18n.Localize(&goi18n.Message{ID: "download_extracting", Other: "Extracting {{.Name}}..."}, map[string]interface{}{"Name": g.Name}),
					gaba.ProcessMessageOptions{
						ShowThemeBackground: true,
						ShowProgressBar:     true,
						Progress:            progress,
					},
					func() (interface{}, error) {
						logger.Debug("Extracting single-file ROM", "game", g.Name, "file", zipPath)

						if err := utils.Unzip(zipPath, romDirectory, progress); err != nil {
							logger.Error("Failed to extract single-file ROM", "game", g.Name, "error", err)
							return nil, err
						}

						if err := os.Remove(zipPath); err != nil {
							logger.Warn("Failed to remove zip file after extraction", "path", zipPath, "error", err)
						}

						return nil, nil
					},
				)

				if err != nil {
					logger.Warn("Failed to extract ROM, keeping zip file", "game", g.Name)
					continue
				}
			}
		}
	}

	downloadedGames := make([]romm.Rom, 0, len(res.Completed))
	for _, g := range input.SelectedGames {
		if slices.ContainsFunc(res.Completed, func(d gaba.Download) bool {
			return d.DisplayName == g.Name
		}) {
			downloadedGames = append(downloadedGames, g)
		}
	}

	logger.Debug("Download complete", "successful", len(downloadedGames), "attempted", len(input.SelectedGames))

	if len(artDownloads) > 0 && len(downloadedGames) > 0 {
		progress := &atomic.Float64{}
		_, err := gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "download_artwork", Other: "Downloading artwork..."}, nil),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (interface{}, error) {
				s.downloadArt(artDownloads, downloadedGames, headers, progress)
				return nil, nil
			},
		)

		if err != nil {
			logger.Warn("Art download process encountered an error", "error", err)
		}
	}

	output.DownloadedGames = downloadedGames
	return success(output), nil
}

func (s *DownloadScreen) buildDownloads(config utils.Config, host romm.Host, platform romm.Platform, games []romm.Rom) ([]gaba.Download, []artDownload) {
	downloads := make([]gaba.Download, 0, len(games))
	artDownloads := make([]artDownload, 0, len(games))

	for _, g := range games {
		gamePlatform := platform
		if platform.ID == 0 && g.PlatformID != 0 {
			gamePlatform = romm.Platform{
				ID:   g.PlatformID,
				Slug: g.PlatformSlug,
				Name: g.PlatformDisplayName,
			}
		}

		romDirectory := utils.GetPlatformRomDirectory(config, gamePlatform)
		downloadLocation := ""

		sourceURL := ""

		if g.HasMultipleFiles {
			tmpDir := utils.TempDir()
			downloadLocation = filepath.Join(tmpDir, fmt.Sprintf("grout_multirom_%d.zip", g.ID))
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", g.DisplayName)
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

		if config.DownloadArt && (g.PathCoverLarge != "" || g.PathCoverSmall != "" || g.URLCover != "") {
			artDir := utils.GetArtDirectory(config, gamePlatform)
			artFileName := g.FsNameNoExt + ".png"
			artLocation := filepath.Join(artDir, artFileName)

			var coverPath string
			if g.PathCoverSmall != "" {
				coverPath = g.PathCoverSmall
			} else if g.PathCoverLarge != "" {
				coverPath = g.PathCoverLarge
			} else if g.URLCover != "" {
				coverPath = g.URLCover
			}

			baseURL := host.URL() + coverPath
			artURL := strings.ReplaceAll(baseURL, " ", "%20")

			artDownloads = append(artDownloads, artDownload{
				URL:      artURL,
				Location: artLocation,
				GameName: g.Name,
			})
		}
	}

	return downloads, artDownloads
}

func (s *DownloadScreen) downloadArt(artDownloads []artDownload, downloadedGames []romm.Rom, headers map[string]string, progress *atomic.Float64) {
	logger := gaba.GetLogger()

	downloadedGameNames := make(map[string]bool)
	for _, g := range downloadedGames {
		downloadedGameNames[g.Name] = true
	}

	totalArt := 0
	for _, art := range artDownloads {
		if downloadedGameNames[art.GameName] {
			totalArt++
		}
	}

	successCount := 0
	failCount := 0
	processedCount := 0

	for _, art := range artDownloads {
		if !downloadedGameNames[art.GameName] {
			continue
		}

		artDir := filepath.Dir(art.Location)
		if err := os.MkdirAll(artDir, 0755); err != nil {
			logger.Warn("Failed to create art directory", "dir", artDir, "game", art.GameName, "error", err)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		req, err := http.NewRequest("GET", art.URL, nil)
		if err != nil {
			logger.Warn("Failed to create art request", "game", art.GameName, "error", err)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("Failed to download art", "game", art.GameName, "url", art.URL, "error", err)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Warn("Art download failed with bad status", "game", art.GameName, "url", art.URL, "status", resp.Status)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		outFile, err := os.Create(art.Location)
		if err != nil {
			resp.Body.Close()
			logger.Warn("Failed to create art file", "game", art.GameName, "location", art.Location, "error", err)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		_, err = io.Copy(outFile, resp.Body)
		resp.Body.Close()
		outFile.Close()

		if err != nil {
			logger.Warn("Failed to write art file", "game", art.GameName, "location", art.Location, "error", err)
			os.Remove(art.Location)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		if err := utils.ProcessArtImage(art.Location); err != nil {
			logger.Warn("Failed to process art image", "game", art.GameName, "location", art.Location, "error", err)
			os.Remove(art.Location)
			failCount++
			processedCount++
			if totalArt > 0 {
				progress.Store(float64(processedCount) / float64(totalArt))
			}
			continue
		}

		successCount++

		processedCount++
		if totalArt > 0 {
			progress.Store(float64(processedCount) / float64(totalArt))
		}
	}

}
