package ui

import (
	"errors"
	"fmt"
	"grout/archive"
	"grout/cfw"
	"grout/cfw/muos"
	"grout/download"
	"grout/files"
	"grout/imaging"
	"grout/romm"
	"grout/settings"
	_ "image/gif"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
)

type DownloadInput struct {
	Config         settings.Config
	Host           settings.Host
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

func NewDownloadScreen() *DownloadScreen {
	return &DownloadScreen{}
}

func (s *DownloadScreen) Execute(config settings.Config, host settings.Host, platform romm.Platform, selectedGames []romm.Rom, allGames []romm.Rom, searchFilter string, selectedFileID int) DownloadOutput {
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

	plan, skipped := download.BuildPlan(input.Config, input.Host, input.Platform, input.SelectedGames, input.SelectedFileID)
	for _, skip := range skipped {
		logger.Warn("Skipping ROM", "game", skip.Game.Name, "id", skip.Game.ID,
			"fs_name", skip.Game.FsName, "reason", skip.Reason)
	}

	downloads := romDownloads(plan, input.Config.DownloadTimeout.Duration())
	artDownloads, gamelistEntries := plan.Art, plan.Entries

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
				files.DeleteFile(d.Location)
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
				files.DeleteFile(g.Location)
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

		tmpZipPath := filepath.Join(files.TempDir(), fmt.Sprintf("grout_multirom_%d.zip", g.ID))
		romDirectory := cfw.PlatformRomDirectory(input.Config, gamePlatform.FSSlug)
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

				if err := archive.Unzip(tmpZipPath, extractDir, progress); err != nil {
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
					romDirectory := cfw.PlatformRomDirectory(input.Config, gamePlatform.FSSlug)
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
								archiveFiles, extractErr = archive.SevenZipFileNames(archivePath)
								if extractErr == nil {
									extractErr = archive.Un7zip(archivePath, romDirectory, progress)
								}
							} else {
								archiveFiles, extractErr = archive.ZipFileNames(archivePath)
								if extractErr == nil {
									extractErr = archive.Unzip(archivePath, romDirectory, progress)
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

func (s *DownloadScreen) downloadArt(artDownloads []download.Item, downloadedGames []romm.Rom, progress *atomic.Float64, host settings.Host) {
	logger := gaba.GetLogger()

	// One fetcher for the whole run so connections are reused across what can
	// be hundreds of art downloads.
	fetcher := romm.NewArtFetcher(host, romm.DefaultClientTimeout)
	fetcher.Process = imaging.ProcessArtImage

	downloaded := make(map[string]bool, len(downloadedGames))
	for _, g := range downloadedGames {
		downloaded[g.Name] = true
	}

	wanted := make([]download.Item, 0, len(artDownloads))
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

// romDownloads adapts the plan's rom items to what the download widget takes.
// The plan says what to fetch; only this layer knows the widget's shape.
func romDownloads(plan download.Plan, timeout time.Duration) []gaba.Download {
	out := make([]gaba.Download, 0, len(plan.Roms))
	for _, item := range plan.Roms {
		out = append(out, gaba.Download{
			URL:         item.URL,
			Location:    item.Location,
			DisplayName: item.GameName,
			Timeout:     timeout,
		})
	}
	return out
}
