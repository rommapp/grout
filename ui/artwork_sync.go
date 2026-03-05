package ui

import (
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/internal/imageutil"
	"grout/romm"
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
	Config internal.Config
	Host   romm.Host
}

type ArtworkSyncOutput struct{}

type ArtworkSyncScreen struct{}

func NewArtworkSyncScreen() *ArtworkSyncScreen {
	return &ArtworkSyncScreen{}
}

func (s *ArtworkSyncScreen) Execute(config internal.Config, host romm.Host) ArtworkSyncOutput {
	s.draw(ArtworkSyncInput{
		Config: config,
		Host:   host,
	})
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
		client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout)
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

				if artForceRes.SelectedValue == SyncMissingOnlyOption {
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
	var allMissingArtwork []romm.Rom
	for _, idx := range sel.Selected {
		pr := sel.Items[idx].Metadata.(platformArtwork)
		allMissingArtwork = append(allMissingArtwork, pr.roms...)
	}

	var downloads []gaba.Download
	romsByLocation := make(map[string]romm.Rom)

	for _, rom := range allMissingArtwork {
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
		romsByLocation[cachePath] = rom
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
	headers["Authorization"] = input.Host.BasicAuthHeader()

	res, err := gaba.DownloadManager(downloads, headers, gaba.DownloadManagerOptions{
		AutoContinueOnComplete: true,
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
