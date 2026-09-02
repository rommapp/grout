package ui

import (
	"errors"
	"fmt"
	"grout/cfw"
	"grout/cfw/muos"
	"grout/domain/library"
	"grout/internal"
	"grout/internal/artutil"
	"grout/internal/fileutil"
	"grout/internal/gamelist"
	"grout/internal/imageutil"
	"grout/internal/stringutil"
	"grout/romm"
	_ "image/gif"
	_ "image/jpeg"
	"net/url"
	"os"
	"path/filepath"
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

				// Update the gamelist entry to point to the extracted file
				// instead of the (now deleted) temporary zip.
				newGamePath := resolveExtractedGamePath(romDirectory, extractDir, g.FsNameNoExt)
				for i, entry := range gamelistEntries {
					if entry.Game.FileName == g.FsName {
						gamelistEntries[i].Game.Path = newGamePath
						break
					}
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
					archivePath := filepath.Join(romDirectory, g.Files[0].FileName)

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
									extractErr = fileutil.Un7zip(archivePath, romDirectory, progress)
								}
							} else {
								archiveFiles, extractErr = fileutil.ZipFileNames(archivePath)
								if extractErr == nil {
									extractErr = fileutil.Unzip(archivePath, romDirectory, progress)
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
								for i, entry := range gamelistEntries {
									if entry.Game.FileName == g.FsName {
										gamelistEntries[i].Game.Path = filepath.Join(romDirectory, gamePath)
										break
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
				s.downloadArt(artDownloads, downloadedGames, progress, input.Host)
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

	// Resolved once rather than per game per art kind: GetCFW re-reads the
	// environment on every call.
	activeCFW := cfw.GetCFW()
	isESBased := activeCFW.IsBasedOnEmulationStation()

	for _, g := range games {
		var artPaths library.ArtPaths
		gamePlatform := platform
		if platform.ID == 0 && g.PlatformID != 0 {
			gamePlatform = romm.Platform{
				ID:     g.PlatformID,
				FSSlug: g.PlatformFSSlug,
				Name:   g.PlatformDisplayName,
			}
		}

		romDirectory := config.GetPlatformRomDirectory(gamePlatform)
		downloadLocation := ""

		sourceURL := ""

		if g.HasMultipleFiles {
			tmpDir := fileutil.TempDir()
			downloadLocation = filepath.Join(tmpDir, fmt.Sprintf("grout_multirom_%d.zip", g.ID))
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", g.FsName)
		} else {
			// Skip games with no file metadata to avoid an out-of-range panic.
			// This can happen when the cached row was written without a
			// `files` array (e.g. after an incremental cache update).
			if len(g.Files) == 0 {
				gaba.GetLogger().Warn("Skipping ROM with no file metadata; refresh the library to repopulate it",
					"game", g.Name, "id", g.ID, "fs_name", g.FsName)
				continue
			}
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
			downloadLocation = filepath.Join(romDirectory, fileToDownload.FileName)
			sourceURL, _ = url.JoinPath(host.URL(), "/api/roms/", strconv.Itoa(g.ID), "content", fileToDownload.FileName)
			sourceURL += "?" + url.Values{"file_ids": {strconv.Itoa(fileToDownload.ID)}}.Encode()
		}

		downloads = append(downloads, gaba.Download{
			URL:         sourceURL,
			Location:    downloadLocation,
			DisplayName: g.Name,
			Timeout:     config.DownloadTimeout.Duration(),
		})

		if config.DownloadArt && (g.PathCoverLarge != "" || g.PathCoverSmall != "" || g.URLCover != "") {
			// Prepare download for cover art
			artDir := config.GetArtDirectory(gamePlatform)
			artFileName := cfw.ArtFileName(activeCFW, cfw.ArtCover, romArtFileName(g), g.FsNameNoExt)
			artLocation := filepath.Join(artDir, artFileName)
			coverURL := g.GetArtworkURL(config.ArtKind, host)
			artPaths.Cover = artLocation

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
				artSplashFileName := cfw.ArtFileName(activeCFW, cfw.ArtThumbnail, romArtFileName(g), g.FsNameNoExt)
				splashArtLocation := filepath.Join(artSplashDir, artSplashFileName)
				kind := config.DownloadSplashArt
				if config.AdditionalDownloads.Thumbnail != artutil.ArtKindNone {
					kind = config.AdditionalDownloads.Thumbnail
				}
				if splashURL := g.GetSplashArtURL(kind, host); splashURL != "" {
					if isESBased {
						artPaths.Thumbnail = splashArtLocation
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
				marqueeArtFileName := cfw.ArtFileName(activeCFW, cfw.ArtMarquee, romArtFileName(g), g.FsNameNoExt)
				marqueeArtLocation := filepath.Join(artMarqueeDir, marqueeArtFileName)
				marqueeURL := ""
				switch config.AdditionalDownloads.Marquee {
				case artutil.ArtKindMarquee:
					marqueeURL = g.GetMarqueeURL(host)
				case artutil.ArtKindLogo:
					marqueeURL = g.GetLogoURL(host)
				}
				if marqueeURL != "" {
					artPaths.Marquee = marqueeArtLocation
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
					artPaths.Video = videoLocation
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
				bezelArtLocation := filepath.Join(artBezelDir, artFileName)
				if bezelURL := g.GetBezelURL(host); bezelURL != "" {
					artPaths.Bezel = bezelArtLocation
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
					artPaths.Manual = manualLocation
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
				boxbackArtFileName := cfw.ArtFileName(activeCFW, cfw.ArtBoxback, romArtFileName(g), g.FsNameNoExt)
				boxbackArtLocation := filepath.Join(boxbackDir, boxbackArtFileName)
				if boxbackURL := g.GetBoxbackURL(host); boxbackURL != "" {
					artPaths.BoxBack = boxbackArtLocation
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
				fanartFileName := cfw.ArtFileName(activeCFW, cfw.ArtFanart, romArtFileName(g), g.FsNameNoExt)
				fanartLocation := filepath.Join(fanartDir, fanartFileName)
				if fanartURL := g.GetFanartURL(host); fanartURL != "" {
					artPaths.Fanart = fanartLocation
					artDownloads = append(artDownloads, artDownload{
						URL:      fanartURL,
						Location: fanartLocation,
						GameName: g.Name,
						IsImage:  true,
					})
				}
			}

		}
		gamesSummaries = append(gamesSummaries, gamelist.RomGameEntry{
			Game:         g.ToGame(stringutil.PrepareRomName(g.Name, g.Regions), downloadLocation, artPaths),
			Platform:     gamePlatform.ToPlatform(),
			RomDirectory: romDirectory,
		})
	}

	return downloads, artDownloads, gamesSummaries
}

// resolveExtractedGamePath returns the best path for a multi-file ROM after extraction.
func resolveExtractedGamePath(romDirectory, extractDir, fsNameNoExt string) string {
	logger := gaba.GetLogger()

	m3uPath := filepath.Join(romDirectory, fsNameNoExt+".m3u")
	if _, err := os.Stat(m3uPath); err == nil {
		logger.Debug("Multi-file ROM gamelist path resolved to .m3u in rom directory", "path", m3uPath)
		return m3uPath
	}
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		logger.Debug("Multi-file ROM gamelist path falling back to extract directory (unreadable)", "path", extractDir, "error", err)
		return extractDir
	}
	var firstFile string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) == ".m3u" {
			p := filepath.Join(extractDir, entry.Name())
			logger.Debug("Multi-file ROM gamelist path resolved to .m3u in extract directory", "path", p)
			return p
		}
		if firstFile == "" {
			firstFile = filepath.Join(extractDir, entry.Name())
		}
	}
	if firstFile != "" {
		logger.Debug("Multi-file ROM gamelist path resolved to first file (no .m3u found)", "path", firstFile)
		return firstFile
	}
	logger.Debug("Multi-file ROM gamelist path falling back to extract directory (no files found)", "path", extractDir)
	return extractDir
}

func (s *DownloadScreen) downloadArt(artDownloads []artDownload, downloadedGames []romm.Rom, progress *atomic.Float64, host romm.Host) {
	logger := gaba.GetLogger()

	// One fetcher for the whole run so connections are reused across what can
	// be hundreds of art downloads.
	fetcher := romm.NewArtFetcher(host, romm.DefaultClientTimeout)
	fetcher.Process = imageutil.ProcessArtImage

	downloaded := make(map[string]bool, len(downloadedGames))
	for _, g := range downloadedGames {
		downloaded[g.Name] = true
	}

	wanted := make([]artDownload, 0, len(artDownloads))
	for _, art := range artDownloads {
		if downloaded[art.GameName] {
			wanted = append(wanted, art)
		}
	}
	if len(wanted) == 0 {
		return
	}

	var succeeded, failed int
	for i, art := range wanted {
		save := fetcher.SaveRaw
		if art.IsImage {
			save = fetcher.Save
		}

		if err := save(art.URL, art.Location); err != nil {
			logger.Warn("Failed to download art",
				"game", art.GameName, "url", art.URL, "location", art.Location, "error", err)
			failed++
		} else {
			succeeded++
		}

		progress.Store(float64(i+1) / float64(len(wanted)))
	}

	logger.Debug("Art download complete", "succeeded", succeeded, "failed", failed)
}
