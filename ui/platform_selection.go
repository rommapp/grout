package ui

import (
	"errors"
	"grout/internal"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type PlatformSelectionInput struct {
	Platforms            *[]romm.Platform // Pointer to allow dynamic updates from state
	QuitOnBack           bool
	ShowCollections      bool
	LastSelectedIndex    int
	LastSelectedPosition int
}

type PlatformSelectionOutput struct {
	Action               PlatformSelectionAction
	SelectedPlatform     romm.Platform
	LastSelectedIndex    int
	LastSelectedPosition int
	ReorderedPlatforms   []romm.Platform
}

type PlatformSelectionScreen struct{}

func NewPlatformSelectionScreen() *PlatformSelectionScreen {
	return &PlatformSelectionScreen{}
}

func (s *PlatformSelectionScreen) Draw(input PlatformSelectionInput) (PlatformSelectionOutput, error) {
	output := PlatformSelectionOutput{
		Action:               PlatformSelectionActionQuit,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	if input.Platforms == nil || len(*input.Platforms) == 0 {
		return output, nil
	}
	platforms := *input.Platforms

	var menuItems []gaba.MenuItem

	if input.ShowCollections {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:           i18n.Localize(&goi18n.Message{ID: "platform_selection_collections", Other: "Collections"}, nil),
			Selected:       false,
			Focused:        false,
			Metadata:       romm.Platform{FSSlug: "collections"},
			NotReorderable: true,
		})
	}

	for _, platform := range platforms {
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
		if !internal.IsKidModeEnabled() {
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
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)})
	} else {
		footerItems = []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
			{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)},
		}
	}

	options := gaba.DefaultListOptions("Grout", menuItems)
	if !internal.IsKidModeEnabled() {
		options.ActionButton = buttons.VirtualButtonX
	}
	options.ReorderButton = buttons.VirtualButtonSelect
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)

	options.StatusBar = StatusBar()

	sel, err := gaba.List(options)

	// Check for reordering before handling errors
	// This ensures we save the order even when user presses B (cancel)
	platformsReordered := false
	startIndex := 0
	if input.ShowCollections {
		startIndex = 1
	}

	if sel != nil && len(sel.Items) > 0 {
		if len(sel.Items)-startIndex == len(platforms) {
			for i := 0; i < len(platforms); i++ {
				originalPlatform := platforms[i]
				returnedPlatform := sel.Items[i+startIndex].Metadata.(romm.Platform)
				if originalPlatform.FSSlug != returnedPlatform.FSSlug {
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
			output.Action = PlatformSelectionActionQuit
			return output, nil
		}
		return output, err
	}

	switch sel.Action {
	case gaba.ListActionSelected:
		platform := sel.Items[sel.Selected[0]].Metadata.(romm.Platform)

		output.SelectedPlatform = platform
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition

		if platform.FSSlug == "collections" {
			output.Action = PlatformSelectionActionCollections
			return output, nil
		}

		output.Action = PlatformSelectionActionSelected
		return output, nil

	case gaba.ListActionTriggered:
		if input.QuitOnBack {
			output.Action = PlatformSelectionActionSettings
			return output, nil
		}
	}

	output.Action = PlatformSelectionActionQuit
	return output, nil
}
