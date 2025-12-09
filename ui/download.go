package ui

import (
	"encoding/base64"
	"fmt"
	"grout/constants"
	"grout/models"
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

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

type DownloadInput struct {
	Config        models.Config
	Host          models.Host
	Platform      romm.Platform
	SelectedGames []romm.Rom
	AllGames      []romm.Rom
	SearchFilter  string
}

type DownloadOutput struct {
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

func (s *DownloadScreen) Execute(config models.Config, host models.Host, platform romm.Platform, selectedGames []romm.Rom, allGames []romm.Rom, searchFilter string) DownloadOutput {
	result, err := s.Draw(DownloadInput{
		Config:        config,
		Host:          host,
		Platform:      platform,
		SelectedGames: selectedGames,
		AllGames:      allGames,
		SearchFilter:  searchFilter,
	})

	if err != nil {
		gaba.GetLogger().Error("Download failed", "error", err)
		return DownloadOutput{
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

func (s *DownloadScreen) Draw(input DownloadInput) (ScreenResult[DownloadOutput], error) {
	logger := gaba.GetLogger()

	output := DownloadOutput{
		Platform:     input.Platform,
		AllGames:     input.AllGames,
		SearchFilter: input.SearchFilter,
	}

	downloads, artDownloads := s.buildDownloads(input.Config, input.Host, input.Platform, input.SelectedGames)

	headers := make(map[string]string)
	auth := input.Host.Username + ":" + input.Host.Password
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	headers["Authorization"] = authHeader

	logger.Debug("RomM Auth Header", "header", authHeader)

	slices.SortFunc(downloads, func(a, b gaba.Download) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	logger.Debug("Starting ROM download", "downloads", downloads)

	res, err := gaba.DownloadManager(downloads, headers, gaba.DownloadManagerOptions{
		AutoContinue: input.Config.DownloadArt,
	})
	if err != nil {
		logger.Error("Error downloading", "error", err)
		return WithCode(output, gaba.ExitCodeError), err
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
		return WithCode(output, gaba.ExitCodeError), nil
	}

	// Process multi-file ROM downloads: extract zips and clean up temp files
	for _, g := range input.SelectedGames {
		if !g.Multi {
			continue
		}

		// Check if this multi-file ROM was successfully downloaded
		completed := slices.ContainsFunc(res.Completed, func(d gaba.Download) bool {
			return d.DisplayName == g.Name
		})
		if !completed {
			continue
		}

		// Get the platform for this game
		gamePlatform := input.Platform
		if input.Platform.ID == 0 && g.PlatformID != 0 {
			gamePlatform = romm.Platform{
				ID:   g.PlatformID,
				Slug: g.PlatformSlug,
				Name: g.PlatformDisplayName,
			}
		}

		// Extract the multi-file ROM with a progress message
		tmpZipPath := filepath.Join(utils.TempDir(), fmt.Sprintf("grout_multirom_%d.zip", g.ID))
		romDirectory := utils.GetPlatformRomDirectory(input.Config, gamePlatform)
		extractDir := filepath.Join(romDirectory, g.Name)

		_, err := gaba.ProcessMessage(
			fmt.Sprintf("Extracting %s...", g.Name),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				logger.Debug("Extracting multi-file ROM", "game", g.Name, "dest", extractDir)

				// Extract the zip directly from disk (low memory usage)
				if err := utils.Unzip(tmpZipPath, extractDir); err != nil {
					logger.Error("Failed to extract multi-file ROM", "game", g.Name, "error", err)
					// Clean up the temp zip file even on error
					os.Remove(tmpZipPath)
					return nil, err
				}

				// muOS requires special folder structure for multi-file ROMs
				if utils.GetCFW() == constants.MuOS {
					if err := utils.OrganizeMultiFileRomForMuOS(extractDir, romDirectory, g.Name); err != nil {
						logger.Error("Failed to organize multi-file ROM for muOS", "game", g.Name, "error", err)
						// Clean up on error
						os.Remove(tmpZipPath)
						os.RemoveAll(extractDir)
						return nil, err
					}
				}

				// Clean up the temp zip file
				if err := os.Remove(tmpZipPath); err != nil {
					logger.Warn("Failed to remove temp zip file", "path", tmpZipPath, "error", err)
				}

				logger.Debug("Successfully extracted multi-file ROM", "game", g.Name, "dest", extractDir)
				return nil, nil
			},
		)

		if err != nil {
			continue
		}
	}

	// Process single-file ROM downloads: extract zips if enabled
	if input.Config.UnzipDownloads {
		for _, g := range input.SelectedGames {
			if g.Multi {
				continue
			}

			// Check if this single-file ROM was successfully downloaded
			completed := slices.ContainsFunc(res.Completed, func(d gaba.Download) bool {
				return d.DisplayName == g.Name
			})
			if !completed {
				continue
			}

			// Get the platform for this game
			gamePlatform := input.Platform
			if input.Platform.ID == 0 && g.PlatformID != 0 {
				gamePlatform = romm.Platform{
					ID:   g.PlatformID,
					Slug: g.PlatformSlug,
					Name: g.PlatformDisplayName,
				}
			}

			// Check if the downloaded file is a zip
			if len(g.Files) > 0 && strings.ToLower(filepath.Ext(g.Files[0].FileName)) == ".zip" {
				romDirectory := utils.GetPlatformRomDirectory(input.Config, gamePlatform)
				zipPath := filepath.Join(romDirectory, g.Files[0].FileName)

				_, err := gaba.ProcessMessage(
					fmt.Sprintf("Extracting %s...", g.Name),
					gaba.ProcessMessageOptions{ShowThemeBackground: true},
					func() (interface{}, error) {
						logger.Debug("Extracting single-file ROM", "game", g.Name, "file", zipPath)

						// Extract the zip to the platform directory
						if err := utils.Unzip(zipPath, romDirectory); err != nil {
							logger.Error("Failed to extract single-file ROM", "game", g.Name, "error", err)
							return nil, err
						}

						// Remove the zip file after successful extraction
						if err := os.Remove(zipPath); err != nil {
							logger.Warn("Failed to remove zip file after extraction", "path", zipPath, "error", err)
						} else {
							logger.Debug("Removed zip file after extraction", "path", zipPath)
						}

						logger.Debug("Successfully extracted single-file ROM", "game", g.Name)
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
			logger.Debug("Game marked as downloaded", "game", g.Name)
		}
	}

	logger.Debug("Download complete", "successful", len(downloadedGames), "attempted", len(input.SelectedGames))

	// Download art silently in the background for successfully downloaded games
	if len(artDownloads) > 0 && len(downloadedGames) > 0 {
		go s.downloadArtInBackground(artDownloads, downloadedGames, headers)
	}

	output.DownloadedGames = downloadedGames
	return Success(output), nil
}

func (s *DownloadScreen) buildDownloads(config models.Config, host models.Host, platform romm.Platform, games []romm.Rom) ([]gaba.Download, []artDownload) {
	downloads := make([]gaba.Download, 0, len(games))
	artDownloads := make([]artDownload, 0, len(games))

	for _, g := range games {
		// For collections, use each game's platform info; for platforms, use the passed platform
		gamePlatform := platform
		if platform.ID == 0 && g.PlatformID != 0 {
			// Construct platform from game's platform info (happens when viewing collections)
			gamePlatform = romm.Platform{
				ID:   g.PlatformID,
				Slug: g.PlatformSlug,
				Name: g.PlatformDisplayName,
			}
		}

		romDirectory := utils.GetPlatformRomDirectory(config, gamePlatform)
		downloadLocation := ""

		sourceURL := ""

		if g.Multi {
			// For multi-file ROMs, download as zip to temp location
			// The zip will be extracted to a folder named after the game
			tmpDir := utils.TempDir()
			downloadLocation = filepath.Join(tmpDir, fmt.Sprintf("grout_multirom_%d.zip", g.ID))
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", g.Name)
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

		// Add art download if enabled and art is available
		if config.DownloadArt && (g.PathCoverSmall != "" || g.URLCover != "") {
			artDir := utils.GetArtDirectory(config, gamePlatform)
			artFileName := g.FsNameNoExt + ".png"
			artLocation := filepath.Join(artDir, artFileName)

			var coverPath string
			if g.PathCoverLarge != "" {
				coverPath = g.PathCoverSmall
			} else if g.URLCover != "" {
				coverPath = g.URLCover
			}

			// Construct and properly encode the URL to handle query params with spaces
			// RomM sometimes returns URLs with unencoded spaces in timestamps
			baseURL := host.URL() + coverPath
			// Replace spaces with %20 to ensure proper URL encoding
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

func (s *DownloadScreen) downloadArtInBackground(artDownloads []artDownload, downloadedGames []romm.Rom, headers map[string]string) {
	logger := gaba.GetLogger()

	// Create a map of downloaded game names for quick lookup
	downloadedGameNames := make(map[string]bool)
	for _, g := range downloadedGames {
		downloadedGameNames[g.Name] = true
	}

	successCount := 0
	failCount := 0

	for _, art := range artDownloads {
		// Only download art for games that were successfully downloaded
		if !downloadedGameNames[art.GameName] {
			continue
		}

		// Create art directory if it doesn't exist
		artDir := filepath.Dir(art.Location)
		if err := os.MkdirAll(artDir, 0755); err != nil {
			logger.Warn("Failed to create art directory", "dir", artDir, "game", art.GameName, "error", err)
			failCount++
			continue
		}

		// Download the art file
		req, err := http.NewRequest("GET", art.URL, nil)
		if err != nil {
			logger.Warn("Failed to create art request", "game", art.GameName, "error", err)
			failCount++
			continue
		}

		// Add authentication headers
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("Failed to download art", "game", art.GameName, "url", art.URL, "error", err)
			failCount++
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Warn("Art download failed with bad status", "game", art.GameName, "url", art.URL, "status", resp.Status)
			failCount++
			continue
		}

		outFile, err := os.Create(art.Location)
		if err != nil {
			logger.Warn("Failed to create art file", "game", art.GameName, "location", art.Location, "error", err)
			failCount++
			continue
		}

		_, err = io.Copy(outFile, resp.Body)
		outFile.Close()

		if err != nil {
			logger.Warn("Failed to write art file", "game", art.GameName, "location", art.Location, "error", err)
			os.Remove(art.Location) // Clean up partial file
			failCount++
			continue
		}

		logger.Debug("Art downloaded successfully", "game", art.GameName, "location", art.Location)

		// Process the art image (convert to PNG if needed and resize)
		if err := utils.ProcessArtImage(art.Location); err != nil {
			logger.Warn("Failed to process art image", "game", art.GameName, "location", art.Location, "error", err)
			os.Remove(art.Location) // Clean up if processing failed
			failCount++
			continue
		}

		logger.Debug("Art processed successfully", "game", art.GameName)
		successCount++
	}

	if successCount > 0 || failCount > 0 {
		logger.Debug("Background art download complete", "successful", successCount, "failed", failCount)
	}
}
