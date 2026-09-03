package ui

import (
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/internal/imageutil"
	"grout/library"
	"grout/romm"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	SyncMissingOnlyOption = "missing_only"
	SyncAllOption         = "all"
)

type ArtworkSyncInput struct {
	Config         internal.Config
	Host           romm.Host
	DownloadedOnly bool
}

type ArtworkSyncOutput struct{}

type ArtworkSyncScreen struct{}

func NewArtworkSyncScreen() *ArtworkSyncScreen {
	return &ArtworkSyncScreen{}
}

func (s *ArtworkSyncScreen) Execute(input ArtworkSyncInput) ArtworkSyncOutput {
	s.draw(input)
	return ArtworkSyncOutput{}
}

func (s *ArtworkSyncScreen) draw(input ArtworkSyncInput) {
	logger := gaba.GetLogger()

	var platforms []romm.Platform
	var err error

	if cm := cache.GetCacheManager(); cm != nil {
		platforms, err = cm.GetPlatforms()
	}
	if len(platforms) == 0 {
		client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout.Duration())
		platforms, err = client.GetPlatforms()
		if err != nil {
			logger.Error("Failed to fetch platforms", "error", err)
			gaba.ConfirmationMessage(
				fmt.Sprintf("Failed to fetch platforms: %v", err),
				ContinueFooter(),
				gaba.MessageOptions{},
			)
			return
		}
	}
	romm.DisambiguatePlatformNames(platforms)

	var mappedPlatforms []romm.Platform
	for _, p := range platforms {
		if _, exists := input.Config.DirectoryMappings[p.FSSlug]; exists {
			mappedPlatforms = append(mappedPlatforms, p)
		}
	}

	if len(mappedPlatforms) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "artwork_sync_no_platforms", Other: "No platforms with directory mappings found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	artForceRes, err := gaba.SelectionMessage(
		i18n.Localize(&goi18n.Message{ID: "artwork_sync_preload_choice", Other: "Do you want to preload all or missing artwork ?"}, nil),
		[]gaba.SelectionOption{
			{DisplayName: i18n.Localize(&goi18n.Message{ID: "artwork_sync_preload_missing", Other: "Missing Only"}, nil), Value: SyncMissingOnlyOption},
			{DisplayName: i18n.Localize(&goi18n.Message{ID: "artwork_sync_preload_all", Other: "All"}, nil), Value: SyncAllOption},
		},
		[]gaba.FooterHelpItem{
			FooterContinue(),
			FooterCancel(),
		},
		gaba.SelectionMessageSettings{},
	)

	if err != nil {
		return
	}

	// Scan all platforms and collect artwork per platform
	type platformArtwork struct {
		platform romm.Platform
		roms     []romm.Rom
	}

	var platformResults []platformArtwork
	platformCount := len(mappedPlatforms)
	cm := cache.GetCacheManager()

	// ProcessMessage runs the closure synchronously on the calling goroutine,
	// so mutating platformResults from within the closure is safe.
	for i, platform := range mappedPlatforms {
		p := platform
		gaba.ProcessMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "artwork_sync_scanning", Other: "Scanning platform %d/%d: %s..."}, nil), i+1, platformCount, p.Name),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				var roms []romm.Rom
				var err error

				if cm != nil {
					roms, err = cm.GetPlatformGames(p.ID)
					if err != nil || len(roms) == 0 {
						if err := cm.RefreshPlatformGames(p); err != nil {
							logger.Error("Failed to refresh platform games", "platform", p.Name, "error", err)
							return nil, nil
						}
						roms, err = cm.GetPlatformGames(p.ID)
						if err != nil {
							logger.Error("Failed to get platform games from cache", "platform", p.Name, "error", err)
							return nil, nil
						}
					}
				} else {
					logger.Error("Cache manager not available", "platform", p.Name)
					return nil, nil
				}

				if input.DownloadedOnly {
					var downloaded []romm.Rom
					for _, r := range roms {
						if isRomDownloaded(input.Config, r) {
							downloaded = append(downloaded, r)
						}
					}
					roms = downloaded

					if artForceRes.SelectedValue == SyncMissingOnlyOption {
						roms = filterMissingCFWArt(roms, p, input.Config, input.Host)
					}
				} else if artForceRes.SelectedValue == SyncMissingOnlyOption {
					roms = cache.GetMissingArtwork(roms)
				}

				if len(roms) > 0 {
					platformResults = append(platformResults, platformArtwork{platform: p, roms: roms})
				}
				return nil, nil
			},
		)
	}

	if len(platformResults) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "artwork_sync_up_to_date", Other: "All artwork is already cached!"}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	// Show platform list in multi-select mode for user to pick which to download
	var menuItems []gaba.MenuItem
	for _, pr := range platformResults {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:     fmt.Sprintf("%s (%d)", pr.platform.Name, len(pr.roms)),
			Selected: true,
			Metadata: pr,
		})
	}

	options := gaba.DefaultListOptions(
		i18n.Localize(&goi18n.Message{ID: "artwork_sync_select_platforms", Other: "Select Platforms"}, nil),
		menuItems,
	)
	options.UseSmallTitle = true
	options.InitialMultiSelectMode = true
	options.FooterHelpItems = []gaba.FooterHelpItem{
		FooterBack(),
		{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_download", Other: "Download"}, nil), IsConfirmButton: true},
	}
	options.StatusBar = StatusBar()

	sel, err := gaba.List(options)
	if err != nil || sel.Action != gaba.ListActionSelected || len(sel.Selected) == 0 {
		return
	}

	// Collect artwork from selected platforms
	var selectedResults []platformRoms
	for _, idx := range sel.Selected {
		pr := sel.Items[idx].Metadata.(platformArtwork)
		selectedResults = append(selectedResults, platformRoms{platform: pr.platform, roms: pr.roms})
	}

	var downloads []gaba.Download

	if input.DownloadedOnly {
		downloads = buildCFWArtDownloads(selectedResults, input.Config, input.Host)
	} else {
		for _, sr := range selectedResults {
			for _, rom := range sr.roms {
				downloadURL := cache.GetArtworkCoverPath(rom, input.Config.ArtKind, input.Host)
				if downloadURL == "" {
					continue
				}

				cachePath := cache.GetArtworkCachePath(rom.PlatformFSSlug, rom.ID)
				cache.EnsureArtworkCacheDir(rom.PlatformFSSlug)

				downloads = append(downloads, gaba.Download{
					URL:         downloadURL,
					Location:    cachePath,
					DisplayName: rom.Name,
				})
			}
		}
	}

	if len(downloads) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "artwork_sync_up_to_date", Other: "All artwork is already cached!"}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	headers := make(map[string]string)
	headers["Authorization"] = input.Host.AuthHeader()

	res, err := gaba.DownloadManager(downloads, headers, gaba.DownloadManagerOptions{
		AutoContinueOnComplete: true,
		SkipSSLVerification:    input.Host.InsecureSkipVerify,
	})
	if err != nil {
		logger.Error("Artwork download failed", "error", err)
		gaba.ConfirmationMessage(
			fmt.Sprintf("Download failed: %v", err),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	var successCount int32
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4)

	for _, download := range res.Completed {
		wg.Add(1)
		go func(dl gaba.Download) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := imageutil.ProcessArtImage(dl.Location); err != nil {
				logger.Warn("Failed to process artwork", "path", dl.Location, "error", err)
				return
			}
			atomic.AddInt32(&successCount, 1)
		}(download)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "artwork_sync_processing", Other: "Processing artwork..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			<-done
			return nil, nil
		},
	)

	finalCount := int(atomic.LoadInt32(&successCount))
	logger.Info("Artwork sync complete", "success", finalCount, "failed", len(res.Failed))

	for _, failed := range res.Failed {
		path := failed.Download.URL
		if u, err := url.Parse(failed.Download.URL); err == nil {
			path = u.Path
		}
		logger.Error("Failed to download artwork",
			"path", path,
			"name", failed.Download.DisplayName,
			"timeout", failed.Download.Timeout,
			"error", failed.Error,
		)
	}

	if finalCount > 0 {
		gaba.ConfirmationMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "artwork_sync_complete", Other: "Successfully downloaded %d artwork images."}, nil), finalCount),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	} else if len(res.Failed) > 0 {
		gaba.ConfirmationMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "artwork_sync_failed", Other: "Failed to download %d artwork images."}, nil), len(res.Failed)),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	}
}

// filterMissingCFWArt returns only roms that are missing art in the CFW art directory.
func filterMissingCFWArt(roms []romm.Rom, platform romm.Platform, config internal.Config, host romm.Host) []romm.Rom {
	activeCFW := cfw.GetCFW()

	var missing []romm.Rom
	for _, rom := range roms {
		if !cache.HasArtworkURL(rom) {
			continue
		}
		artDir := config.ArtDirectory(platform, cfw.ArtCover)
		artPath := filepath.Join(artDir, cfw.ArtFileName(activeCFW, cfw.ArtCover, romArtFileName(rom), rom.FsNameNoExt))
		if !fileutil.FileExists(artPath) {
			missing = append(missing, rom)
			continue
		}
		// Also check preview and splash if configured
		if config.DownloadArtScreenshotPreview {
			previewDir := config.ArtDirectory(platform, cfw.ArtScreenshotPreview)
			if previewDir != "" && rom.GetScreenshotURL(host) != "" {
				if !fileutil.FileExists(filepath.Join(previewDir, cfw.ArtFileName(activeCFW, cfw.ArtScreenshotPreview, romArtFileName(rom), rom.FsNameNoExt))) {
					missing = append(missing, rom)
					continue
				}
			}
		}
		if config.DownloadSplashArt != library.ArtKindNone {
			splashDir := config.ArtDirectory(platform, cfw.ArtThumbnail)
			if splashDir != "" && rom.GetSplashArtURL(config.DownloadSplashArt, host) != "" {
				if !fileutil.FileExists(filepath.Join(splashDir, cfw.ArtFileName(activeCFW, cfw.ArtThumbnail, romArtFileName(rom), rom.FsNameNoExt))) {
					missing = append(missing, rom)
					continue
				}
			}
		}
	}
	return missing
}

type platformRoms struct {
	platform romm.Platform
	roms     []romm.Rom
}

// buildCFWArtDownloads builds download entries targeting CFW art directories.
func buildCFWArtDownloads(results []platformRoms, config internal.Config, host romm.Host) []gaba.Download {
	var downloads []gaba.Download
	activeCFW := cfw.GetCFW()

	for _, sr := range results {
		for _, rom := range sr.roms {
			artFileName := cfw.ArtFileName(activeCFW, cfw.ArtCover, romArtFileName(rom), rom.FsNameNoExt)

			// Cover art
			coverURL := rom.GetArtworkURL(config.ArtKind, host)
			if coverURL != "" {
				artDir := config.ArtDirectory(sr.platform, cfw.ArtCover)
				artLocation := filepath.Join(artDir, artFileName)
				downloads = append(downloads, gaba.Download{
					URL:         coverURL,
					Location:    artLocation,
					DisplayName: rom.Name,
				})
			}

			// Screenshot preview
			if config.DownloadArtScreenshotPreview {
				previewDir := config.ArtDirectory(sr.platform, cfw.ArtScreenshotPreview)
				if previewDir != "" {
					if screenshotURL := rom.GetScreenshotURL(host); screenshotURL != "" {
						downloads = append(downloads, gaba.Download{
							URL:         screenshotURL,
							Location:    filepath.Join(previewDir, artFileName),
							DisplayName: rom.Name,
						})
					}
				}
			}

			// Splash art
			if config.DownloadSplashArt != library.ArtKindNone {
				splashDir := config.ArtDirectory(sr.platform, cfw.ArtThumbnail)
				if splashDir != "" {
					if splashURL := rom.GetSplashArtURL(config.DownloadSplashArt, host); splashURL != "" {
						downloads = append(downloads, gaba.Download{
							URL:         splashURL,
							Location:    filepath.Join(splashDir, cfw.ArtFileName(activeCFW, cfw.ArtThumbnail, romArtFileName(rom), rom.FsNameNoExt)),
							DisplayName: rom.Name,
						})
					}
				}
			}
		}
	}

	return downloads
}
