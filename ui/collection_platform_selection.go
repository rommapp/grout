package ui

import (
	"errors"
	"fmt"
	"grout/romm"
	"grout/utils"
	"slices"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type CollectionPlatformSelectionInput struct {
	Config               *utils.Config
	Host                 romm.Host
	Collection           romm.Collection
	CachedGames          []romm.Rom
	LastSelectedIndex    int
	LastSelectedPosition int
}

type CollectionPlatformSelectionOutput struct {
	SelectedPlatform     romm.Platform
	Collection           romm.Collection
	AllGames             []romm.Rom
	LastSelectedIndex    int
	LastSelectedPosition int
}

type CollectionPlatformSelectionScreen struct{}

func NewCollectionPlatformSelectionScreen() *CollectionPlatformSelectionScreen {
	return &CollectionPlatformSelectionScreen{}
}

func (s *CollectionPlatformSelectionScreen) Draw(input CollectionPlatformSelectionInput) (ScreenResult[CollectionPlatformSelectionOutput], error) {
	logger := gaba.GetLogger()
	output := CollectionPlatformSelectionOutput{
		Collection:           input.Collection,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	var allGames []romm.Rom
	if len(input.CachedGames) > 0 {
		allGames = input.CachedGames
	} else {
		// Build query for cache key and freshness check
		cacheKey := utils.GetCollectionCacheKey(input.Collection)
		opt := romm.GetRomsQuery{
			Limit: 10000,
		}

		// Use appropriate ID based on collection type
		if input.Collection.IsVirtual {
			opt.VirtualCollectionID = input.Collection.VirtualID
		} else if input.Collection.IsSmart {
			opt.SmartCollectionID = input.Collection.ID
		} else {
			opt.CollectionID = input.Collection.ID
		}

		// Check if a prefetch is in progress for this collection
		loadedFromCache := false
		if cr := utils.GetCacheRefresh(); cr != nil {
			if cr.IsPrefetchInProgress(cacheKey) {
				logger.Debug("Waiting for collection prefetch to complete", "key", cacheKey)
				// Show a loading message while waiting
				gaba.ProcessMessage(
					i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."}, map[string]interface{}{"Name": input.Collection.Name}),
					gaba.ProcessMessageOptions{ShowThemeBackground: true},
					func() (interface{}, error) {
						cr.WaitForPrefetch(cacheKey)
						return nil, nil
					},
				)
				// After prefetch completes, load from cache
				cached, err := utils.LoadCachedGames(cacheKey)
				if err == nil {
					logger.Debug("Loaded collection from prefetch cache", "key", cacheKey, "count", len(cached))
					allGames = cached
					loadedFromCache = true
				}
			}
		}

		// Check if cache is fresh (skip loading screen if so)
		if !loadedFromCache {
			isFresh, _ := utils.CheckCacheFreshness(input.Host, input.Config, cacheKey, opt)
			if isFresh {
				cached, err := utils.LoadCachedGames(cacheKey)
				if err == nil {
					logger.Debug("Loaded collection games from cache (no loading screen)", "key", cacheKey, "count", len(cached))
					allGames = cached
					loadedFromCache = true
				}
			}
		}

		// If we didn't load from cache, fetch from API with loading screen
		if !loadedFromCache {
			var loadErr error
			_, err := gaba.ProcessMessage(
				i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."}, map[string]interface{}{"Name": input.Collection.Name}),
				gaba.ProcessMessageOptions{ShowThemeBackground: true},
				func() (interface{}, error) {
					// Fetch from API
					rc := utils.GetRommClient(input.Host)
					res, err := rc.GetRoms(opt)
					if err != nil {
						logger.Error("Error downloading game list", "error", err)
						loadErr = err
						return nil, err
					}
					allGames = res.Items

					// Save to cache
					if err := utils.SaveGamesToCache(cacheKey, allGames); err != nil {
						logger.Debug("Failed to save games to cache", "error", err)
					}

					return nil, nil
				},
			)

			if err != nil || loadErr != nil {
				return withCode(output, gaba.ExitCodeError), err
			}
		}
	}

	// Handle unified mode - skip platform selection and return all games
	if input.Config.CollectionView == "unified" {
		// Filter games to only include those with mapped platforms
		filteredGames := make([]romm.Rom, 0)
		for _, game := range allGames {
			if _, hasMapping := input.Config.DirectoryMappings[game.PlatformSlug]; hasMapping {
				filteredGames = append(filteredGames, game)
			}
		}

		output.AllGames = filteredGames
		output.SelectedPlatform = romm.Platform{ID: 0} // ID=0 signals unified mode
		return success(output), nil
	}

	platformMap := make(map[int]romm.Platform)
	for _, game := range allGames {
		if _, exists := platformMap[game.PlatformID]; !exists {
			if _, hasMapping := input.Config.DirectoryMappings[game.PlatformSlug]; hasMapping {
				platformMap[game.PlatformID] = romm.Platform{
					ID:   game.PlatformID,
					Slug: game.PlatformSlug,
					Name: game.PlatformDisplayName,
				}
			}
		}
	}

	if len(platformMap) == 0 {
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "collection_platform_no_mapped", Other: "No platforms with mapped games in\n{{.Name}}"}, map[string]interface{}{"Name": input.Collection.Name}),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				time.Sleep(time.Second * 2)
				return nil, nil
			},
		)
		return withCode(output, gaba.ExitCodeBack), nil
	}

	platforms := make([]romm.Platform, 0, len(platformMap))
	for _, platform := range platformMap {
		platforms = append(platforms, platform)
	}

	slices.SortFunc(platforms, func(a, b romm.Platform) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	gameCounts := make(map[int]int)
	for _, game := range allGames {
		if _, hasMapping := input.Config.DirectoryMappings[game.PlatformSlug]; hasMapping {
			gameCounts[game.PlatformID]++
		}
	}

	menuItems := make([]gaba.MenuItem, len(platforms))
	for i, platform := range platforms {
		gameCount := gameCounts[platform.ID]
		displayName := fmt.Sprintf("%s (%d)", platform.Name, gameCount)
		menuItems[i] = gaba.MenuItem{
			Text:     displayName,
			Selected: false,
			Focused:  false,
			Metadata: platform,
		}
	}

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
		{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)},
	}

	title := i18n.Localize(&goi18n.Message{ID: "collection_platform_title", Other: "{{.Name}} - Platforms"}, map[string]interface{}{"Name": input.Collection.Name})
	options := gaba.DefaultListOptions(title, menuItems)
	options.SmallTitle = true
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)
	options.StatusBar = utils.StatusBar()

	sel, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		return withCode(output, gaba.ExitCodeError), err
	}

	switch sel.Action {
	case gaba.ListActionSelected:
		platform := sel.Items[sel.Selected[0]].Metadata.(romm.Platform)

		output.SelectedPlatform = platform
		output.AllGames = allGames
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		return success(output), nil

	default:
		return withCode(output, gaba.ExitCodeBack), nil
	}
}
