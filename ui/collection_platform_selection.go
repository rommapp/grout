package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/romm"
	"slices"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type CollectionPlatformSelectionInput struct {
	Config               *internal.Config
	Host                 romm.Host
	Collection           romm.Collection
	CachedGames          []romm.Rom
	LastSelectedIndex    int
	LastSelectedPosition int
}

type CollectionPlatformSelectionOutput struct {
	Action               CollectionPlatformSelectionAction
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

func (s *CollectionPlatformSelectionScreen) Draw(input CollectionPlatformSelectionInput) (CollectionPlatformSelectionOutput, error) {
	logger := gaba.GetLogger()
	output := CollectionPlatformSelectionOutput{
		Action:               CollectionPlatformSelectionActionBack,
		Collection:           input.Collection,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	var allGames []romm.Rom
	if len(input.CachedGames) > 0 {
		allGames = input.CachedGames
	} else {
		cm := cache.GetCacheManager()
		if cm != nil {
			// Try to load from cache via game_collections join
			if cached, err := cm.GetCollectionGames(input.Collection); err == nil && len(cached) > 0 {
				logger.Debug("Loaded collection games from cache", "collection", input.Collection.Name, "count", len(cached))
				allGames = cached
			} else if len(input.Collection.ROMIDs) > 0 {
				// Fallback: collection has ROM IDs, fetch games directly by ID from cache
				logger.Debug("Cache join miss, trying direct ID lookup", "collection", input.Collection.Name, "romIDs", len(input.Collection.ROMIDs))
				if games, err := cm.GetGamesByIDs(input.Collection.ROMIDs); err == nil && len(games) > 0 {
					logger.Debug("Loaded collection games by ID from cache", "collection", input.Collection.Name, "count", len(games))
					allGames = games
				}
			}
		}

		// If still no games, show error - cache should be populated
		if len(allGames) == 0 {
			gaba.ProcessMessage(
				i18n.Localize(&goi18n.Message{ID: "collection_cache_missing", Other: "Collection not cached.\nPlease refresh the cache."}, nil),
				gaba.ProcessMessageOptions{ShowThemeBackground: true},
				func() (interface{}, error) {
					time.Sleep(time.Second * 2)
					return nil, nil
				},
			)
			return output, nil
		}
	}

	// Handle unified mode - skip platform selection and return all games
	if input.Config.CollectionView == internal.CollectionViewUnified {
		// Filter games to only include those with mapped platforms
		filteredGames := make([]romm.Rom, 0)
		for _, game := range allGames {
			if _, hasMapping := input.Config.DirectoryMappings[game.PlatformFSSlug]; hasMapping {
				filteredGames = append(filteredGames, game)
			}
		}

		output.AllGames = filteredGames
		output.SelectedPlatform = romm.Platform{ID: 0} // ID=0 signals unified mode
		output.Action = CollectionPlatformSelectionActionSelected
		return output, nil
	}

	platformMap := make(map[int]romm.Platform)
	for _, game := range allGames {
		if _, exists := platformMap[game.PlatformID]; !exists {
			if _, hasMapping := input.Config.DirectoryMappings[game.PlatformFSSlug]; hasMapping {
				platformMap[game.PlatformID] = romm.Platform{
					ID:     game.PlatformID,
					FSSlug: game.PlatformFSSlug,
					Name:   game.PlatformDisplayName,
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
		return output, nil
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
		if _, hasMapping := input.Config.DirectoryMappings[game.PlatformFSSlug]; hasMapping {
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
	options.UseSmallTitle = true
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)
	options.StatusBar = StatusBar()

	sel, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	switch sel.Action {
	case gaba.ListActionSelected:
		platform := sel.Items[sel.Selected[0]].Metadata.(romm.Platform)

		output.SelectedPlatform = platform
		output.AllGames = allGames
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		output.Action = CollectionPlatformSelectionActionSelected
		return output, nil

	default:
		return output, nil
	}
}
