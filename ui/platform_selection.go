package ui

import (
	"errors"
	"grout/constants"
	"grout/romm"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

type PlatformSelectionInput struct {
	Platforms            []romm.Platform
	QuitOnBack           bool // If true, back button quits the app; if false, it navigates back
	ShowCollections      bool
	LastSelectedIndex    int
	LastSelectedPosition int
}

type PlatformSelectionOutput struct {
	SelectedPlatform     romm.Platform
	LastSelectedIndex    int
	LastSelectedPosition int
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
		return WithCode(output, gaba.ExitCode(404)), nil
	}

	var menuItems []gaba.MenuItem

	if input.ShowCollections {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:     "Collections",
			Selected: false,
			Focused:  false,
			Metadata: romm.Platform{Slug: "collections"},
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
		footerItems = []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: "Quit"},
			{ButtonName: "X", HelpText: "Settings"},
			{ButtonName: "A", HelpText: "Select"},
		}
	} else {
		footerItems = []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: "Back"},
			{ButtonName: "A", HelpText: "Select"},
		}
	}

	options := gaba.DefaultListOptions("Grout", menuItems)
	options.EnableAction = input.QuitOnBack
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
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition

		if platform.Slug == "collections" {
			return WithCode(output, constants.ExitCodeCollections), nil
		}

		return Success(output), nil

	case gaba.ListActionTriggered:
		// Settings action (X button) - only available when QuitOnBack is true
		if input.QuitOnBack {
			return WithCode(output, gaba.ExitCodeAction), nil
		}
	}

	return WithCode(output, gaba.ExitCodeBack), nil
}
