package ui

import (
	"crypto/tls"
	"errors"
	"fmt"
	"grout/cfw"
	"grout/cfw/minui"
	"grout/cfw/muos"
	"grout/internal"
	"grout/internal/artutil"
	"grout/internal/fileutil"
	"grout/internal/gamelist"
	"grout/internal/imageutil"
	"grout/romm"
	_ "image/gif"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
)

type DownloadInput struct {
	Config         internal.Config
	Host           romm.Host
	Platform       romm.Platform
	SelectedGames  []romm.Rom
	AllGames       []romm.Rom
	SearchFilter   string
	SelectedFileID int
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
	IsImage  bool
}

func NewDownloadScreen() *DownloadScreen {
	return &DownloadScreen{}
}

func (s *DownloadScreen) Execute(config internal.Config, host romm.Host, platform romm.Platform, selectedGames []romm.Rom, allGames []romm.Rom, searchFilter string, selectedFileID int) DownloadOutput {
	result, err := s.draw(DownloadInput{
		Config:         config,
		Host:           host,
		Platform:       platform,
		SelectedGames:  selectedGames,
		AllGames:       allGames,
		SelectedFileID: selectedFileID,
		SearchFilter:   searchFilter,
	})

	if err != nil {
		gaba.GetLogger().Error("Download failed", "error", err)
		return DownloadOutput{
			AllGames:     allGames,
			Platform:     platform,
			SearchFilter: searchFilter,
		}
	}

	if len(result.DownloadedGames) > 0 {
		gaba.GetLogger().Debug("Successfully downloaded games", "count", len(result.DownloadedGames))
	}

	return result
}

func (s *DownloadScreen) draw(input DownloadInput) (DownloadOutput, error) {
	logger := gaba.GetLogger()

	output := DownloadOutput{
		Platform:     input.Platform,
		AllGames:     input.AllGames,
		SearchFilter: input.SearchFilter,
	}

	downloads, artDownloads, gamelistEntries := s.buildDownloads(input.Config, input.Host, input.Platform, input.SelectedGames, input.SelectedFileID)

	headers := make(map[string]string)
	headers["Authorization"] = input.Host.AuthHeader()

	slices.SortFunc(downloads, func(a, b gaba.Download) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	logger.Debug("Starting ROM download", "downloads", downloads)

	res, err := gaba.DownloadManager(downloads, headers, gaba.DownloadManagerOptions{
		AutoContinueOnComplete: input.Config.DownloadArt,
		SkipSSLVerification:    input.Host.InsecureSkipVerify,
	})
	if err != nil {
		logger.Error("Error downloading", "error", err)

		// Clean up any partial downloads when cancelled
		if errors.Is(err, gaba.ErrCancelled) {
			for _, d := range downloads {
				fileutil.DeleteFile(d.Location)
			}
		}

		return output, err
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
				fileutil.DeleteFile(g.Location)
			}
		}
	}

	if len(res.Completed) == 0 {
		return output, nil
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
				ID:     g.PlatformID,
				FSSlug: g.PlatformFSSlug,
				Name:   g.PlatformDisplayName,
			}
		}

		tmpZipPath := filepath.Join(fileutil.TempDir(), fmt.Sprintf("grout_multirom_%d.zip", g.ID))
		romDirectory := input.Config.GetPlatformRomDirectory(gamePlatform)
		extractDir := filepath.Join(romDirectory, g.FsNameNoExt)

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

				if err := fileutil.Unzip(tmpZipPath, extractDir, progress); err != nil {
					logger.Error("Failed to extract multi-file ROM", "game", g.DisplayName, "error", err)
					os.Remove(tmpZipPath)
					return nil, err
				}

				if cfw.GetCFW() == cfw.MuOS {
					if err := muos.OrganizeMultiFileRom(extractDir, romDirectory, g.FsNameNoExt); err != nil {
						logger.Error("Failed to organize multi-file ROM for muOS", "game", g.FsNameNoExt, "error", err)
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
					ID:     g.PlatformID,
					FSSlug: g.PlatformFSSlug,
					Name:   g.PlatformDisplayName,
				}
			}

			if len(g.Files) > 0 {
				ext := strings.ToLower(filepath.Ext(g.Files[0].FileName))
				if ext == ".zip" || ext == ".7z" {
					romDirectory := input.Config.GetPlatformRomDirectory(gamePlatform)
					var archivePath string
					var extractDir string
					var cleanName string
					var gameFolder string

					if input.Config.SubfolderPerGame && cfw.GetCFW() == cfw.MinUI {
						tag := extractPlatformTag(romDirectory)
						cleanName = stripParentheses(g.FsNameNoExt)
						gameFolder = fmt.Sprintf("%s (%s)", cleanName, tag)
						extractDir = filepath.Join(minui.GetRomDirectory(), gameFolder)
						archivePath = filepath.Join(extractDir, g.Files[0].FileName)
					} else {
						extractDir = romDirectory
						archivePath = filepath.Join(romDirectory, g.Files[0].FileName)
					}

					progress := &atomic.Float64{}
					_, err := gaba.ProcessMessage(
						i18n.Localize(&goi18n.Message{ID: "download_extracting", Other: "Extracting {{.Name}}..."}, map[string]interface{}{"Name": g.Name}),
						gaba.ProcessMessageOptions{
							ShowThemeBackground: true,
							ShowProgressBar:     true,
							Progress:            progress,
						},
						func() (interface{}, error) {
							logger.Debug("Extracting single-file ROM", "game", g.Name, "file", archivePath)

							var archiveFiles []string
							var extractErr error
							if ext == ".7z" {
								archiveFiles, extractErr = fileutil.SevenZipFileNames(archivePath)
								if extractErr == nil {
									extractErr = fileutil.Un7zip(archivePath, extractDir, progress)
								}
							} else {
								archiveFiles, extractErr = fileutil.ZipFileNames(archivePath)
								if extractErr == nil {
									extractErr = fileutil.Unzip(archivePath, extractDir, progress)
								}
							}

							if extractErr != nil {
								logger.Error("Failed to extract single-file ROM", "game", g.Name, "error", extractErr)
								return nil, extractErr
							}

							if err := os.Remove(archivePath); err != nil {
								logger.Warn("Failed to remove archive file after extraction", "path", archivePath, "error", err)
							}

							if len(archiveFiles) > 0 {
								gamePath := archiveFiles[0]
								if len(archiveFiles) > 1 {
									for _, f := range archiveFiles {
										if strings.ToLower(filepath.Ext(f)) == ".m3u" {
											gamePath = f
											break
										}
									}
								}

								// Write m3u and rename extracted file if SubfolderPerGame
								if input.Config.SubfolderPerGame && cfw.GetCFW() == cfw.MinUI {
									realExt := filepath.Ext(gamePath)
									cleanFileName := cleanName + realExt
									oldPath := filepath.Join(extractDir, gamePath)
									newPath := filepath.Join(extractDir, cleanFileName)
									if err := os.Rename(oldPath, newPath); err != nil {
										logger.Warn("Failed to rename extracted ROM", "from", oldPath, "to", newPath, "error", err)
										cleanFileName = gamePath // fallback to original name
									}
									m3uPath := filepath.Join(extractDir, gameFolder+".m3u")
									if err := os.WriteFile(m3uPath, []byte(cleanFileName), 0644); err != nil {
										logger.Warn("Failed to write m3u file", "path", m3uPath, "error", err)
									}
									for i, entry := range gamelistEntries {
										if entry.Game.ID == g.ID {
											gamelistEntries[i].GamePath = newPath
											break
										}
									}
								} else {
									for i, entry := range gamelistEntries {
										if entry.Game.ID == g.ID {
											gamelistEntries[i].GamePath = filepath.Join(romDirectory, gamePath)
											break
										}
									}
								}
							}

							return nil, nil
						},
					)

					if err != nil {
						logger.Warn("Failed to extract ROM, keeping archive file", "game", g.Name)
						continue
					}
				}
			}
		}
	}

	// Write m3u for SubfolderPerGame when not unzipping
	if input.Config.SubfolderPerGame && !input.Config.UnzipDownloads && cfw.GetCFW() == cfw.MinUI {
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

			if len(g.Files) == 0 {
				continue
			}

			ext := strings.ToLower(filepath.Ext(g.Files[0].FileName))

			gamePlatform := input.Platform
			if input.Platform.ID == 0 && g.PlatformID != 0 {
				gamePlatform = romm.Platform{
					ID:     g.PlatformID,
					FSSlug: g.PlatformFSSlug,
					Name:   g.PlatformDisplayName,
				}
			}

			romDirectory := input.Config.GetPlatformRomDirectory(gamePlatform)
			tag := extractPlatformTag(romDirectory)
			cleanName := stripParentheses(g.FsNameNoExt)
			gameFolder := fmt.Sprintf("%s (%s)", cleanName, tag)
			subdir := filepath.Join(minui.GetRomDirectory(), gameFolder)

			if err := os.MkdirAll(subdir, 0755); err != nil {
				logger.Warn("Failed to create game subfolder for m3u", "dir", subdir, "error", err)
				continue
			}

			romFileName := cleanName + ext
			m3uPath := filepath.Join(subdir, gameFolder+".m3u")
			if err := os.WriteFile(m3uPath, []byte(romFileName), 0644); err != nil {
				logger.Warn("Failed to write m3u file", "path", m3uPath, "error", err)
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
				s.downloadArt(artDownloads, downloadedGames, headers, progress, input.Host.InsecureSkipVerify)
				return nil, nil
			},
		)

		if err != nil {
			logger.Warn("Art download process encountered an error", "error", err)
		}
	}

	cfw.FillGamesMetadata(gamelistEntries)

	output.DownloadedGames = downloadedGames
	return output, nil
}

func (s *DownloadScreen) buildDownloads(config internal.Config, host romm.Host, platform romm.Platform, games []romm.Rom, selectedFileID int) ([]gaba.Download, []artDownload, []gamelist.RomGameEntry) {
	downloads := make([]gaba.Download, 0, len(games))
	artDownloads := make([]artDownload, 0, len(games))
	gamesSummaries := make([]gamelist.RomGameEntry, 0, len(games))

	for _, g := range games {
		gamelistRomEntry := gamelist.RomGameEntry{
			Game:     &g,
			Platform: &platform,
		}
		gamePlatform := platform
		if platform.ID == 0 && g.PlatformID != 0 {
			gamePlatform = romm.Platform{
				ID:     g.PlatformID,
				FSSlug: g.PlatformFSSlug,
				Name:   g.PlatformDisplayName,
			}
		}

		romDirectory := config.GetPlatformRomDirectory(gamePlatform)
		gamelistRomEntry.RomDirectory = romDirectory
		downloadLocation := ""

		sourceURL := ""

		if g.HasMultipleFiles {
			tmpDir := fileutil.TempDir()
			downloadLocation = filepath.Join(tmpDir, fmt.Sprintf("grout_multirom_%d.zip", g.ID))
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", g.FsName)
		} else {
			// Find the file to download - use selected file if specified, otherwise first file
			fileToDownload := g.Files[0]
			if selectedFileID > 0 {
				for _, f := range g.Files {
					if f.ID == selectedFileID {
						fileToDownload = f
						break
					}
				}
			}
			if config.SubfolderPerGame && cfw.GetCFW() == cfw.MinUI {
				tag := extractPlatformTag(romDirectory)
				cleanName := stripParentheses(g.FsNameNoExt)
				gameFolder := fmt.Sprintf("%s (%s)", cleanName, tag)
				subdir := filepath.Join(minui.GetRomDirectory(), gameFolder)
				if err := os.MkdirAll(subdir, 0755); err != nil {
					gaba.GetLogger().Warn("Failed to create game subfolder", "dir", subdir, "error", err)
				}
				ext := filepath.Ext(fileToDownload.FileName)
				cleanFileName := cleanName + ext
				downloadLocation = filepath.Join(subdir, cleanFileName)
			} else {
				downloadLocation = filepath.Join(romDirectory, fileToDownload.FileName)
			}
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", fileToDownload.FileName)
			sourceURL += "?" + url.Values{"file_ids": {strconv.Itoa(fileToDownload.ID)}}.Encode()
		}

		gamelistRomEntry.GamePath = downloadLocation

		downloads = append(downloads, gaba.Download{
			URL:         sourceURL,
			Location:    downloadLocation,
			DisplayName: g.Name,
			Timeout:     config.DownloadTimeout.Duration(),
		})

		if config.DownloadArt && (g.PathCoverLarge != "" || g.PathCoverSmall != "" || g.URLCover != "") {
			// Prepare download for cover art
			artDir := config.GetArtDirectory(gamePlatform)
			var artFileName string
			if cfw.GetCFW() == cfw.MinUI && len(g.Files) > 0 {
				artFileName = g.Files[0].FileName + ".png"
			} else {
				artFileName = g.FsNameNoExt + ".png"
			}
			artLocation := filepath.Join(artDir, artFileName)
			coverURL := g.GetArtworkURL(config.ArtKind, host)
			gamelistRomEntry.ArtLocation.ImagePath = artLocation

			artDownloads = append(artDownloads, artDownload{
				URL:      coverURL,
				Location: artLocation,
				GameName: g.Name,
				IsImage:  true,
			})

			// Prepare download for additional art types if enabled
			artPreviewDir := config.GetArtPreviewDirectory(gamePlatform)
			if config.DownloadArtScreenshotPreview && artPreviewDir != "" {
				screenshotPreviewLocation := filepath.Join(artPreviewDir, artFileName)
				if screenshotURL := g.GetScreenshotURL(host); screenshotURL != "" {
					artDownloads = append(artDownloads, artDownload{
						URL:      screenshotURL,
						Location: screenshotPreviewLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}

			artSplashDir := config.GetArtSplashDirectory(gamePlatform)
			if (config.DownloadSplashArt != artutil.ArtKindNone || config.AdditionalDownloads.Thumbnail != artutil.ArtKindNone) && artSplashDir != "" {
				artSplashFileName := g.FsNameNoExt
				isESBased := cfw.GetCFW().IsBasedOnEmulationStation()
				if isESBased {
					artSplashFileName += "-thumb.png"
				} else {
					artSplashFileName += ".png"
				}
				splashArtLocation := filepath.Join(artSplashDir, artSplashFileName)
				kind := config.DownloadSplashArt
				if config.AdditionalDownloads.Thumbnail != artutil.ArtKindNone {
					kind = config.AdditionalDownloads.Thumbnail
				}
				if splashURL := g.GetSplashArtURL(kind, host); splashURL != "" {
					if isESBased {
						gamelistRomEntry.ArtLocation.ThumbnailPath = splashArtLocation
					}
					artDownloads = append(artDownloads, artDownload{
						URL:      splashURL,
						Location: splashArtLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}

			artMarqueeDir := config.GetArtMarqueeDirectory(gamePlatform)
			if config.AdditionalDownloads.Marquee != artutil.ArtKindNone && artMarqueeDir != "" {
				marqueeArtFileName := g.FsNameNoExt
				// if cfw is ES based, use -marquee suffix to avoid conflicts with cover art
				if cfw.GetCFW().IsBasedOnEmulationStation() {
					marqueeArtFileName += "-marquee.png"
				} else {
					marqueeArtFileName += ".png"
				}
				marqueeArtLocation := filepath.Join(artMarqueeDir, marqueeArtFileName)
				marqueeURL := ""
				switch config.AdditionalDownloads.Marquee {
				case artutil.ArtKindMarquee:
					marqueeURL = g.GetMarqueeURL(host)
				case artutil.ArtKindLogo:
					marqueeURL = g.GetLogoURL(host)
				}
				if marqueeURL != "" {
					gamelistRomEntry.ArtLocation.MarqueePath = marqueeArtLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      marqueeURL,
						Location: marqueeArtLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}

			artVideoDir := config.GetArtVideoDirectory(gamePlatform)
			if config.AdditionalDownloads.Video && artVideoDir != "" {
				videoLocation := filepath.Join(artVideoDir, g.FsNameNoExt+".mp4")
				if videoURL := g.GetVideoURL(host); videoURL != "" {
					gamelistRomEntry.ArtLocation.VideoPath = videoLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      videoURL,
						Location: videoLocation,
						GameName: g.Name,
						IsImage:  false,
					})
				}
			}

			artBezelDir := config.GetArtBezelDirectory(gamePlatform)
			if config.AdditionalDownloads.Bezel && artBezelDir != "" {
				bezelArtFileName := g.FsNameNoExt
				// if cfw is ES based, use -bezel suffix to avoid conflicts with cover art
				if cfw.GetCFW().IsBasedOnEmulationStation() {
					bezelArtFileName += "-bezel.png"
				} else {
					bezelArtFileName += ".png"
				}
				bezelArtLocation := filepath.Join(artBezelDir, bezelArtFileName)
				if bezelURL := g.GetBezelURL(host); bezelURL != "" {
					gamelistRomEntry.ArtLocation.BezelPath = bezelArtLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      bezelURL,
						Location: bezelArtLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}

			manualDir := config.GetManualDirectory(gamePlatform)
			if config.AdditionalDownloads.Manual && manualDir != "" {
				manualLocation := filepath.Join(manualDir, g.FsNameNoExt+".pdf")
				if manualURL := g.GetManualURL(host); manualURL != "" {
					gamelistRomEntry.ArtLocation.ManualPath = manualLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      manualURL,
						Location: manualLocation,
						GameName: g.Name,
						IsImage:  false,
					})
				}
			}

			boxbackDir := config.GetBoxbackDirectory(gamePlatform)
			if config.AdditionalDownloads.BoxBack && boxbackDir != "" {
				boxbackArtFileName := g.FsNameNoExt
				// if cfw is ES based, use -boxback suffix to avoid conflicts with cover art
				if cfw.GetCFW().IsBasedOnEmulationStation() {
					boxbackArtFileName += "-boxback.png"
				} else {
					boxbackArtFileName += ".png"
				}
				boxbackArtLocation := filepath.Join(boxbackDir, boxbackArtFileName)
				if boxbackURL := g.GetBoxbackURL(host); boxbackURL != "" {
					gamelistRomEntry.ArtLocation.BoxBackPath = boxbackArtLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      boxbackURL,
						Location: boxbackArtLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}

			fanartDir := config.GetFanartDirectory(gamePlatform)
			if config.AdditionalDownloads.Fanart && fanartDir != "" {
				fanartFileName := g.FsNameNoExt
				// if cfw is ES based, use -fanart suffix to avoid conflicts with cover art
				if cfw.GetCFW().IsBasedOnEmulationStation() {
					fanartFileName += "-fanart.png"
				} else {
					fanartFileName += ".png"
				}
				fanartLocation := filepath.Join(fanartDir, fanartFileName)
				if fanartURL := g.GetFanartURL(host); fanartURL != "" {
					gamelistRomEntry.ArtLocation.FanartPath = fanartLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      fanartURL,
						Location: fanartLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}
		}
		gamesSummaries = append(gamesSummaries, gamelistRomEntry)
	}

	return downloads, artDownloads, gamesSummaries
}

func stripParentheses(name string) string {
	re := regexp.MustCompile(`\s*\([^)]*\)`)
	return strings.TrimSpace(re.ReplaceAllString(name, ""))
}

func extractPlatformTag(romDirectory string) string {
	base := filepath.Base(romDirectory)
	start := strings.LastIndex(base, "(")
	end := strings.LastIndex(base, ")")
	if start >= 0 && end > start {
		return base[start+1 : end]
	}
	return base
}

func (s *DownloadScreen) downloadArt(artDownloads []artDownload, downloadedGames []romm.Rom, headers map[string]string, progress *atomic.Float64, insecureSkipVerify bool) {
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

		client := &http.Client{Timeout: romm.DefaultClientTimeout}
		if insecureSkipVerify {
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
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

		if art.IsImage {
			if err := imageutil.ProcessArtImage(art.Location); err != nil {
				logger.Warn("Failed to process art image", "game", art.GameName, "location", art.Location, "error", err, "url", art.URL)
				os.Remove(art.Location)
				failCount++
				processedCount++
				if totalArt > 0 {
					progress.Store(float64(processedCount) / float64(totalArt))
				}
				continue
			}
		}

		successCount++

		processedCount++
		if totalArt > 0 {
			progress.Store(float64(processedCount) / float64(totalArt))
		}
	}
}
