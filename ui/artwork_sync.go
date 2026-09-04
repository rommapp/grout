package ui

import (
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"

	"grout/cache"
	"grout/catalog"
	"grout/download"
	"grout/imaging"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

const (
	SyncMissingOnlyOption = "missing_only"
	SyncAllOption         = "all"
)

type ArtworkSyncInput struct {
	Config settings.Config
	Host   settings.Host
	// DownloadedOnly fills the firmware's own art directories for games already
	// on the card. Without it the run prefetches the covers grout shows in its
	// own lists.
	DownloadedOnly bool
}

type ArtworkSyncOutput struct{}

type ArtworkSyncScreen struct{}

func NewArtworkSyncScreen() *ArtworkSyncScreen {
	return &ArtworkSyncScreen{}
}

// platformRoms is a platform and the games of it that need artwork.
type platformRoms struct {
	platform romm.Platform
	roms     []romm.Rom
}

func (s *ArtworkSyncScreen) Execute(input ArtworkSyncInput) ArtworkSyncOutput {
	s.draw(input)
	return ArtworkSyncOutput{}
}

func (s *ArtworkSyncScreen) draw(input ArtworkSyncInput) {
	platforms, err := catalog.MappedPlatforms(input.Host, input.Config.DirectoryMappings, input.Config.ApiTimeout.Duration())
	if err != nil {
		gaba.GetLogger().Error("Failed to fetch platforms", "error", err)
		s.tell(fmt.Sprintf("Failed to fetch platforms: %v", err))
		return
	}
	if len(platforms) == 0 {
		s.tell(localize("artwork_sync_no_platforms", "No platforms with directory mappings found."))
		return
	}

	missingOnly, ok := s.askScope()
	if !ok {
		return
	}

	found := s.scan(input, platforms, missingOnly)
	if len(found) == 0 {
		s.tell(localize("artwork_sync_up_to_date", "All artwork is already cached!"))
		return
	}

	chosen, ok := s.choosePlatforms(found)
	if !ok {
		return
	}

	downloads := s.downloadsFor(input, chosen)
	if len(downloads) == 0 {
		s.tell(localize("artwork_sync_up_to_date", "All artwork is already cached!"))
		return
	}

	s.fetch(input, downloads)
}

// askScope asks whether to fetch everything or only what is missing. The
// second result is false when the user backs out.
func (s *ArtworkSyncScreen) askScope() (missingOnly bool, ok bool) {
	result, err := gaba.SelectionMessage(
		localize("artwork_sync_preload_choice", "Do you want to preload all or missing artwork ?"),
		[]gaba.SelectionOption{
			{DisplayName: localize("artwork_sync_preload_missing", "Missing Only"), Value: SyncMissingOnlyOption},
			{DisplayName: localize("artwork_sync_preload_all", "All"), Value: SyncAllOption},
		},
		[]gaba.FooterHelpItem{FooterContinue(), FooterCancel()},
		gaba.SelectionMessageSettings{},
	)
	if err != nil {
		return false, false
	}
	return result.SelectedValue == SyncMissingOnlyOption, true
}

// scan works out which games of each platform still want artwork, showing
// progress since it reads every platform's library and, for a device sync,
// stats every art file.
func (s *ArtworkSyncScreen) scan(input ArtworkSyncInput, platforms []romm.Platform, missingOnly bool) []platformRoms {
	logger := gaba.GetLogger()

	var found []platformRoms
	for i, platform := range platforms {
		// ProcessMessage runs the closure on the calling goroutine, so
		// appending from inside it is safe.
		gaba.ProcessMessage(
			fmt.Sprintf(localize("artwork_sync_scanning", "Scanning platform %d/%d: %s..."), i+1, len(platforms), platform.Name),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (any, error) {
				games, err := catalog.Games(catalog.GameSource{Platform: platform})
				if err != nil {
					logger.Error("Failed to read platform games", "platform", platform.Name, "error", err)
					return nil, nil
				}

				if wanting := s.wantingArt(input, platform, games, missingOnly); len(wanting) > 0 {
					found = append(found, platformRoms{platform: platform, roms: wanting})
				}
				return nil, nil
			},
		)
	}
	return found
}

// wantingArt narrows a platform's games to those a run should fetch art for.
func (s *ArtworkSyncScreen) wantingArt(input ArtworkSyncInput, platform romm.Platform, games []romm.Rom, missingOnly bool) []romm.Rom {
	if !input.DownloadedOnly {
		if missingOnly {
			return cache.GetMissingArtwork(games)
		}
		return games
	}

	// Only games actually on the card have anywhere for the firmware to look.
	wanting := make([]romm.Rom, 0, len(games))
	for _, game := range games {
		if !catalog.IsDownloaded(input.Config, game) {
			continue
		}
		if missingOnly && len(download.Missing(s.artFor(input, game, platform))) == 0 {
			continue
		}
		wanting = append(wanting, game)
	}
	return wanting
}

// artFor is the artwork a game should have in the firmware's directories.
//
// The same list decides what is missing and what gets fetched, so the two can
// never disagree about a file's name.
func (s *ArtworkSyncScreen) artFor(input ArtworkSyncInput, game romm.Rom, platform romm.Platform) []download.Item {
	return download.Images(download.ArtFor(input.Config, input.Host, game, platform))
}

// choosePlatforms lets the user drop platforms from the run. Everything starts
// selected, since asking for a sync means wanting all of it by default.
func (s *ArtworkSyncScreen) choosePlatforms(found []platformRoms) ([]platformRoms, bool) {
	items := make([]gaba.MenuItem, 0, len(found))
	for _, entry := range found {
		items = append(items, gaba.MenuItem{
			Text:     fmt.Sprintf("%s (%d)", entry.platform.Name, len(entry.roms)),
			Selected: true,
			Metadata: entry,
		})
	}

	options := gaba.DefaultListOptions(localize("artwork_sync_select_platforms", "Select Platforms"), items)
	options.UseSmallTitle = true
	options.InitialMultiSelectMode = true
	options.StatusBar = StatusBar()
	options.FooterHelpItems = []gaba.FooterHelpItem{
		FooterBack(),
		{ButtonName: icons.Start, HelpText: localize("button_download", "Download"), IsConfirmButton: true},
	}

	result, err := gaba.List(options)
	if err != nil || result.Action != gaba.ListActionSelected || len(result.Selected) == 0 {
		return nil, false
	}

	chosen := make([]platformRoms, 0, len(result.Selected))
	for _, index := range result.Selected {
		chosen = append(chosen, result.Items[index].Metadata.(platformRoms))
	}
	return chosen, true
}

func (s *ArtworkSyncScreen) downloadsFor(input ArtworkSyncInput, chosen []platformRoms) []gaba.Download {
	var downloads []gaba.Download

	for _, entry := range chosen {
		for _, game := range entry.roms {
			if input.DownloadedOnly {
				for _, item := range s.artFor(input, game, entry.platform) {
					downloads = append(downloads, gaba.Download{
						URL: item.URL, Location: item.Location, DisplayName: game.Name,
					})
				}
				continue
			}

			source := cache.GetArtworkCoverPath(game, input.Config.ArtKind, input.Host)
			if source == "" {
				continue
			}
			cache.EnsureArtworkCacheDir(game.PlatformFSSlug)
			downloads = append(downloads, gaba.Download{
				URL:         source,
				Location:    cache.GetArtworkCachePath(game.PlatformFSSlug, game.ID),
				DisplayName: game.Name,
			})
		}
	}

	return downloads
}

func (s *ArtworkSyncScreen) fetch(input ArtworkSyncInput, downloads []gaba.Download) {
	logger := gaba.GetLogger()

	result, err := gaba.DownloadManager(downloads, map[string]string{
		"Authorization": input.Host.AuthHeader(),
	}, gaba.DownloadManagerOptions{
		AutoContinueOnComplete: true,
		SkipSSLVerification:    input.Host.InsecureSkipVerify,
	})
	if err != nil {
		logger.Error("Artwork download failed", "error", err)
		s.tell(fmt.Sprintf("Download failed: %v", err))
		return
	}

	for _, failure := range result.Failed {
		path := failure.Download.URL
		if parsed, err := url.Parse(failure.Download.URL); err == nil {
			path = parsed.Path
		}
		logger.Error("Failed to download artwork", "path", path, "name", failure.Download.DisplayName,
			"timeout", failure.Download.Timeout, "error", failure.Error)
	}

	processed := s.process(result.Completed)
	logger.Info("Artwork sync complete", "success", processed, "failed", len(result.Failed))

	switch {
	case processed > 0:
		s.tell(fmt.Sprintf(localize("artwork_sync_complete", "Successfully downloaded %d artwork images."), processed))
	case len(result.Failed) > 0:
		s.tell(fmt.Sprintf(localize("artwork_sync_failed", "Failed to download %d artwork images."), len(result.Failed)))
	}
}

// process normalises the downloaded images and returns how many survived.
//
// Resizing is CPU bound and these devices have few cores, so a handful run at
// once rather than one per image.
func (s *ArtworkSyncScreen) process(downloaded []gaba.Download) int {
	const workers = 4

	var succeeded int32
	var group sync.WaitGroup
	slots := make(chan struct{}, workers)

	for _, item := range downloaded {
		group.Add(1)
		go func(path string) {
			defer group.Done()
			slots <- struct{}{}
			defer func() { <-slots }()

			if err := imaging.ProcessArtImage(path); err != nil {
				gaba.GetLogger().Warn("Failed to process artwork", "path", path, "error", err)
				return
			}
			atomic.AddInt32(&succeeded, 1)
		}(item.Location)
	}

	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()

	gaba.ProcessMessage(
		localize("artwork_sync_processing", "Processing artwork..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			<-done
			return nil, nil
		},
	)

	return int(atomic.LoadInt32(&succeeded))
}

func (s *ArtworkSyncScreen) tell(message string) {
	gaba.ConfirmationMessage(message, ContinueFooter(), gaba.MessageOptions{})
}
