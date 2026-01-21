package ui

import (
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/internal/imageutil"
	"grout/romm"
	"strings"
	"sync"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
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

	var allMissingArtwork []romm.Rom
	platformCount := len(mappedPlatforms)

	cm := cache.GetCacheManager()
	for i, platform := range mappedPlatforms {
		gaba.ProcessMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "artwork_sync_scanning", Other: "Scanning platform %d/%d: %s..."}, nil), i+1, platformCount, platform.Name),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				var roms []romm.Rom
				var err error

				if cm != nil {
					roms, err = cm.GetPlatformGames(platform.ID)
					if err != nil || len(roms) == 0 {
						if err := cm.RefreshPlatformGames(platform); err != nil {
							logger.Error("Failed to refresh platform games", "platform", platform.Name, "error", err)
							return nil, nil
						}
						roms, err = cm.GetPlatformGames(platform.ID)
						if err != nil {
							logger.Error("Failed to get platform games from cache", "platform", platform.Name, "error", err)
							return nil, nil
						}
					}
				} else {
					logger.Error("Cache manager not available", "platform", platform.Name)
					return nil, nil
				}

				missingArtwork := cache.GetMissingArtwork(roms)
				allMissingArtwork = append(allMissingArtwork, missingArtwork...)
				return nil, nil
			},
		)
	}

	if len(allMissingArtwork) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "artwork_sync_up_to_date", Other: "All artwork is already cached!"}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	var downloads []gaba.Download
	romsByLocation := make(map[string]romm.Rom)

	baseURL := input.Host.URL()
	for _, rom := range allMissingArtwork {
		coverPath := cache.GetArtworkCoverPath(rom)
		if coverPath == "" {
			continue
		}

		downloadURL := strings.ReplaceAll(baseURL+coverPath, " ", "%20")
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

	_, err = gaba.ConfirmationMessage(
		fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "artwork_sync_confirm", Other: "Download artwork for %d games?"}, nil), len(downloads)),
		[]gaba.FooterHelpItem{
			FooterCancel(),
			FooterDownload(),
		},
		gaba.MessageOptions{},
	)

	if err != nil {
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
