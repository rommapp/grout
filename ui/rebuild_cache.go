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

type RebuildCacheScreen struct{}

func NewRebuildCacheScreen() *RebuildCacheScreen {
	return &RebuildCacheScreen{}
}

func (s *RebuildCacheScreen) Draw(input RebuildCacheInput) (RebuildCacheOutput, error) {
	logger := gaba.GetLogger()

	if input.CacheSync != nil {
		input.CacheSync.Stop()
	}

	if err := cache.DeleteCacheFolder(); err != nil {
		logger.Error("Failed to delete cache folder", "error", err)
	}

	if err := cache.InitCacheManager(input.Host, input.Config); err != nil {
		logger.Error("Failed to reinitialize cache manager", "error", err)
		return RebuildCacheOutput{Action: RebuildCacheActionError}, err
	}

	platforms, err := internal.GetMappedPlatforms(input.Host, input.Config.DirectoryMappings, input.Config.ApiTimeout)
	if err != nil {
		logger.Error("Failed to fetch platforms", "error", err)
		return RebuildCacheOutput{Action: RebuildCacheActionError}, err
	}

	platforms = internal.SortPlatformsByOrder(platforms, input.Config.PlatformOrder)

	cm := cache.GetCacheManager()
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

	if input.CacheSync != nil {
		input.CacheSync.SetSynced()
	}

	return RebuildCacheOutput{
		Action:           RebuildCacheActionComplete,
		UpdatedPlatforms: platforms,
	}, nil
}
