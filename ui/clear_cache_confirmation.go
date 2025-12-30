package ui

import (
	"errors"
	"grout/utils"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type ClearCacheOutput struct {
	ClearedCount int
}

type ClearCacheScreen struct{}

func NewClearCacheScreen() *ClearCacheScreen {
	return &ClearCacheScreen{}
}

type cacheItem struct {
	name     string
	hasCache bool
	clear    func() error
}

func (s *ClearCacheScreen) Draw() (ScreenResult[ClearCacheOutput], error) {
	output := ClearCacheOutput{}

	caches := []cacheItem{
		{
			name:     i18n.Localize(&goi18n.Message{ID: "cache_artwork", Other: "Artwork Cache"}, nil),
			hasCache: utils.HasArtworkCache(),
			clear:    utils.ClearArtworkCache,
		},
		{
			name:     i18n.Localize(&goi18n.Message{ID: "cache_games", Other: "Games Cache"}, nil),
			hasCache: utils.HasGamesCache(),
			clear:    utils.ClearGamesCache,
		},
	}

	// Build menu items for caches that exist
	items := make([]gaba.MenuItem, 0)
	availableCaches := make([]cacheItem, 0)
	for _, cache := range caches {
		if cache.hasCache {
			items = append(items, gaba.MenuItem{Text: cache.name})
			availableCaches = append(availableCaches, cache)
		}
	}

	if len(items) == 0 {
		// No caches to clear
		return back(output), nil
	}

	options := gaba.DefaultListOptions(
		i18n.Localize(&goi18n.Message{ID: "clear_cache_title", Other: "Clear Cache"}, nil),
		items,
	)
	options.FooterHelpItems = []gaba.FooterHelpItem{
		FooterCancel(),
		{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_confirm", Other: "Confirm"}, nil), IsConfirmButton: true},
	}
	options.StartInMultiSelectMode = true
	options.StatusBar = utils.StatusBar()
	options.SmallTitle = true

	result, err := gaba.List(options)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		return withCode(output, gaba.ExitCodeError), err
	}

	for _, idx := range result.Selected {
		if idx >= 0 && idx < len(availableCaches) {
			cache := availableCaches[idx]
			if err := cache.clear(); err != nil {
				gaba.GetLogger().Error("Failed to clear cache", "cache", cache.name, "error", err)
			} else {
				output.ClearedCount++
				gaba.GetLogger().Info("Cleared cache", "cache", cache.name)
			}
		}
	}

	if output.ClearedCount > 0 {
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "cache_cleared", Other: "Cache cleared!"}, nil),
			gaba.ProcessMessageOptions{},
			func() (interface{}, error) {
				time.Sleep(time.Second * 1)
				return nil, nil
			},
		)
	}

	return success(output), nil
}
