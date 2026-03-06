package ui

import (
	"grout/cache"
	"grout/internal"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	uatomic "go.uber.org/atomic"
)

type RebuildCacheInput struct {
	Host      romm.Host
	Config    *internal.Config
	CacheSync *cache.BackgroundSync
}

type RebuildCacheOutput struct {
	Action           RebuildCacheAction
	UpdatedPlatforms []romm.Platform
}

type RebuildCacheAction int

const (
	RebuildCacheActionComplete RebuildCacheAction = iota
	RebuildCacheActionError
)

const (
	clearOptionMetadata = iota
	clearOptionArtwork
	clearOptionBoth
)

type RebuildCacheScreen struct{}

func NewRebuildCacheScreen() *RebuildCacheScreen {
	return &RebuildCacheScreen{}
}

func (s *RebuildCacheScreen) Draw(input RebuildCacheInput) (RebuildCacheOutput, error) {
	logger := gaba.GetLogger()
	output := RebuildCacheOutput{Action: RebuildCacheActionComplete}

	result, err := gaba.SelectionMessage(
		i18n.Localize(&goi18n.Message{ID: "cache_clear_prompt", Other: "What would you like to clear?"}, nil),
		[]gaba.SelectionOption{
			{DisplayName: i18n.Localize(&goi18n.Message{ID: "cache_clear_metadata", Other: "Metadata"}, nil), Value: clearOptionMetadata},
			{DisplayName: i18n.Localize(&goi18n.Message{ID: "cache_clear_artwork", Other: "Artwork"}, nil), Value: clearOptionArtwork},
			{DisplayName: i18n.Localize(&goi18n.Message{ID: "cache_clear_both", Other: "All"}, nil), Value: clearOptionBoth},
		},
		[]gaba.FooterHelpItem{
			FooterContinue(),
			FooterCancel(),
		},
		gaba.SelectionMessageSettings{},
	)

	if err != nil {
		return output, nil
	}

	selected := result.SelectedValue.(int)

	if input.CacheSync != nil {
		input.CacheSync.Stop()
	}

	cm := cache.GetCacheManager()
	if cm == nil {
		if err := cache.InitCacheManager(input.Host, input.Config); err != nil {
			logger.Error("Failed to reinitialize cache manager", "error", err)
			return RebuildCacheOutput{Action: RebuildCacheActionError}, err
		}
		cm = cache.GetCacheManager()
	}

	switch selected {
	case clearOptionMetadata:
		if err := cm.ClearMetadata(); err != nil {
			logger.Error("Failed to clear metadata cache", "error", err)
		}
	case clearOptionArtwork:
		cm.ClearArtwork()
	case clearOptionBoth:
		if err := cm.ClearMetadata(); err != nil {
			logger.Error("Failed to clear metadata cache", "error", err)
		}
		cm.ClearArtwork()
	}

	// Only rebuild metadata cache if metadata was cleared
	if selected == clearOptionMetadata || selected == clearOptionBoth {
		platforms, err := internal.GetMappedPlatforms(input.Host, input.Config.DirectoryMappings, input.Config.ApiTimeout)
		if err != nil {
			logger.Error("Failed to fetch platforms", "error", err)
			return RebuildCacheOutput{Action: RebuildCacheActionError}, err
		}

		platforms = internal.SortPlatformsByOrder(platforms, input.Config.PlatformOrder)

		progress := uatomic.NewFloat64(0)
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "cache_building", Other: "Building cache..."}, nil),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (any, error) {
				_, err := cm.PopulateFullCacheWithProgress(platforms, progress)
				return nil, err
			},
		)

		output.UpdatedPlatforms = platforms
	}

	if input.CacheSync != nil {
		input.CacheSync.SetSynced()
	}

	return output, nil
}
