package ui

import (
	"errors"
	"fmt"
	"grout/catalog"
	"grout/romm"
	"grout/settings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type CollectionPlatformSelectionInput struct {
	Config               *settings.Config
	Host                 settings.Host
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

	allGames := input.CachedGames
	if len(allGames) == 0 {
		games, err := catalog.CollectionGames(input.Collection)
		if err != nil {
			logger.Debug("Cannot read a collection's games", "collection", input.Collection.Name, "error", err)
			s.tell(localize("collection_cache_missing", "Collection not cached.\nPlease refresh the cache."))
			return output, nil
		}
		allGames = games
	}

	// Unified mode skips this screen: every platform's games are shown at once
	// and the caller reads the empty platform as meaning all of them.
	if input.Config.CollectionView == settings.CollectionViewUnified {
		output.AllGames = catalog.GamesOnMappedPlatforms(*input.Config, allGames)
		output.SelectedPlatform = romm.Platform{}
		output.Action = CollectionPlatformSelectionActionSelected
		return output, nil
	}

	groups := catalog.PlatformsIn(*input.Config, allGames)
	if len(groups) == 0 {
		s.tell(localizeWith("collection_platform_no_mapped",
			"No platforms with mapped games in\n{{.Name}}", map[string]any{"Name": input.Collection.Name}))
		return output, nil
	}

	menuItems := make([]gaba.MenuItem, len(groups))
	for i, group := range groups {
		menuItems[i] = gaba.MenuItem{
			Text:     fmt.Sprintf("%s (%d)", group.Name, len(group.Games)),
			Metadata: i,
		}
	}

	footerItems := []gaba.FooterHelpItem{FooterBack(), FooterSelect()}

	title := localizeWith("collection_platform_title", "{{.Name}} - Platforms", map[string]any{"Name": input.Collection.Name})
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
		index, _ := sel.Items[sel.Selected[0]].Metadata.(int)
		group := groups[index]

		output.SelectedPlatform = romm.Platform{ID: group.Games[0].PlatformID, FSSlug: group.FSSlug, Name: group.Name}
		output.AllGames = allGames
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		output.Action = CollectionPlatformSelectionActionSelected
		return output, nil

	default:
		return output, nil
	}
}

// tell shows a message that closes itself, for a dead end the user cannot act
// on from here.
func (s *CollectionPlatformSelectionScreen) tell(message string) {
	gaba.ProcessMessage(message, gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			time.Sleep(2 * time.Second)
			return nil, nil
		},
	)
}
