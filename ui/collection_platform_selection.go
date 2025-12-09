package ui

import (
	"errors"
	"fmt"
	"grout/models"
	"grout/romm"
	"grout/utils"
	"slices"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

type CollectionPlatformSelectionInput struct {
	Config               *models.Config
	Host                 models.Host
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
		var loadErr error
		_, err := gaba.ProcessMessage(
			fmt.Sprintf("Loading %s...", input.Collection.Name),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				rc := utils.GetRommClient(input.Host)
				opt := &romm.GetRomsOptions{
					Limit:        10000,
					CollectionID: &input.Collection.ID,
				}

				res, err := rc.GetRoms(opt)
				if err != nil {
					logger.Error("Error downloading game list", "error", err)
					loadErr = err
					return nil, err
				}
				allGames = res.Items
				return nil, nil
			},
		)

		if err != nil || loadErr != nil {
			return WithCode(output, gaba.ExitCodeError), err
		}
	}

	platformMap := make(map[int]romm.Platform)
	for _, game := range allGames {
		if _, exists := platformMap[game.PlatformID]; !exists {
			// Check if this platform is mapped in config
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
			fmt.Sprintf("No platforms with mapped games in %s", input.Collection.Name),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				time.Sleep(time.Second * 2)
				return nil, nil
			},
		)
		return WithCode(output, gaba.ExitCodeBack), nil
	}

	// Convert map to sorted slice
	platforms := make([]romm.Platform, 0, len(platformMap))
	for _, platform := range platformMap {
		platforms = append(platforms, platform)
	}

	// Sort platforms by name
	slices.SortFunc(platforms, func(a, b romm.Platform) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	// Build menu items
	menuItems := make([]gaba.MenuItem, len(platforms))
	for i, platform := range platforms {
		// Count games for this platform
		gameCount := 0
		for _, game := range allGames {
			if game.PlatformID == platform.ID {
				gameCount++
			}
		}

		displayName := fmt.Sprintf("%s (%d)", platform.Name, gameCount)
		menuItems[i] = gaba.MenuItem{
			Text:     displayName,
			Selected: false,
			Focused:  false,
			Metadata: platform,
		}
	}

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: "Back"},
		{ButtonName: "A", HelpText: "Select"},
	}

	title := fmt.Sprintf("%s - Platforms", input.Collection.Name)
	options := gaba.DefaultListOptions(title, menuItems)
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)

	sel, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return Back(output), nil
		}
		return WithCode(output, gaba.ExitCodeError), err
	}

	switch sel.Action {
	case gaba.ListActionSelected:
		platform := sel.Items[sel.Selected[0]].Metadata.(romm.Platform)

		output.SelectedPlatform = platform
		output.AllGames = allGames
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		return Success(output), nil
	}

	return WithCode(output, gaba.ExitCodeBack), nil
}
