package ui

import (
	"errors"
	"grout/constants"
	"grout/romm"
	"grout/utils"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type PlatformSelectionInput struct {
	Platforms            []romm.Platform
	QuitOnBack           bool
	ShowCollections      bool
	ShowSaveSync         *atomic.Bool // nil = hidden, otherwise controls visibility dynamically
	LastSelectedIndex    int
	LastSelectedPosition int
}

type PlatformSelectionOutput struct {
	SelectedPlatform     romm.Platform
	LastSelectedIndex    int
	LastSelectedPosition int
	ReorderedPlatforms   []romm.Platform
}

type PlatformSelectionScreen struct{}

func NewPlatformSelectionScreen() *PlatformSelectionScreen {
	return &PlatformSelectionScreen{}
}

func (s *PlatformSelectionScreen) Draw(input PlatformSelectionInput) (ScreenResult[PlatformSelectionOutput], error) {
	output := PlatformSelectionOutput{
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	if len(input.Platforms) == 0 {
		return withCode(output, gaba.ExitCode(404)), nil
	}

	var menuItems []gaba.MenuItem

	if input.ShowCollections {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:           i18n.Localize(&goi18n.Message{ID: "platform_selection_collections", Other: "Collections"}, nil),
			Selected:       false,
			Focused:        false,
			Metadata:       romm.Platform{Slug: "collections"},
			NotReorderable: true,
		})
	}

	for _, platform := range input.Platforms {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:     platform.Name,
			Selected: false,
			Focused:  false,
			Metadata: platform,
		})
	}

	var footerItems []gaba.FooterHelpItem
	if input.QuitOnBack {
		footerItems = []gaba.FooterHelpItem{}
		if !utils.IsKidModeEnabled() {
			footerItems = append(footerItems, gaba.FooterHelpItem{
				ButtonName: "X",
				HelpText:   i18n.Localize(&goi18n.Message{ID: "button_settings", Other: "Settings"}, nil),
			})
		} else {
			footerItems = append(footerItems, gaba.FooterHelpItem{
				ButtonName: "B",
				HelpText:   i18n.Localize(&goi18n.Message{ID: "button_quit", Other: "Quit"}, nil),
			})
		}
		if input.ShowSaveSync != nil && !utils.IsKidModeEnabled() {
			footerItems = append(footerItems, gaba.FooterHelpItem{
				ButtonName: "Y",
				HelpText:   i18n.Localize(&goi18n.Message{ID: "button_save_sync", Other: "Sync"}, nil),
				Show:       input.ShowSaveSync,
			})
		}
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)})
	} else {
		footerItems = []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
			{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)},
		}
	}

	options := gaba.DefaultListOptions("Grout", menuItems)
	if !utils.IsKidModeEnabled() {
		options.ActionButton = buttons.VirtualButtonX
	}
	if input.ShowSaveSync != nil {
		options.SecondaryActionButton = buttons.VirtualButtonY
	}
	options.ReorderButton = buttons.VirtualButtonSelect
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)

	options.StatusBar = utils.StatusBar()

	sel, err := gaba.List(options)

	// Check for reordering before handling errors
	// This ensures we save the order even when user presses B (cancel)
	platformsReordered := false
	startIndex := 0
	if input.ShowCollections {
		startIndex = 1
	}

	if sel != nil && len(sel.Items) > 0 {
		if len(sel.Items)-startIndex == len(input.Platforms) {
			for i := 0; i < len(input.Platforms); i++ {
				originalPlatform := input.Platforms[i]
				returnedPlatform := sel.Items[i+startIndex].Metadata.(romm.Platform)
				if originalPlatform.Slug != returnedPlatform.Slug {
					platformsReordered = true
					break
				}
			}
		}

		if platformsReordered {
			var reorderedPlatforms []romm.Platform
			for i := startIndex; i < len(sel.Items); i++ {
				platform := sel.Items[i].Metadata.(romm.Platform)
				reorderedPlatforms = append(reorderedPlatforms, platform)
			}
			output.ReorderedPlatforms = reorderedPlatforms
		}
	}

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
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition

		if platform.Slug == "collections" {
			return withCode(output, constants.ExitCodeCollections), nil
		}

		return success(output), nil

	case gaba.ListActionTriggered:
		if input.QuitOnBack {
			return withCode(output, gaba.ExitCodeAction), nil
		}

	case gaba.ListActionSecondaryTriggered:
		if input.QuitOnBack && input.ShowSaveSync != nil {
			return withCode(output, constants.ExitCodeSaveSync), nil
		}
	}

	return withCode(output, gaba.ExitCodeBack), nil
}
