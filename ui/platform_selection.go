package ui

import (
	"errors"
	"grout/catalog"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type PlatformSelectionInput struct {
	Platforms            *[]romm.Platform // Pointer to allow dynamic updates from state
	QuitOnBack           bool
	ShowCollections      bool
	ShowSaveSync         bool
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

	// Collections sit above the platforms and stay there, so the row is not
	// draggable and carries a marker of its own rather than a platform with a
	// reserved slug that a real one could take.
	if input.ShowCollections {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:           localize("platform_selection_collections", "Collections"),
			Metadata:       collectionsRow{},
			NotReorderable: true,
		})
	}

	for _, platform := range platforms {
		menuItems = append(menuItems, gaba.MenuItem{Text: platform.Name, Metadata: platform})
	}

	var footerItems []gaba.FooterHelpItem
	if input.QuitOnBack {
		footerItems = []gaba.FooterHelpItem{}
		if !settings.IsKidModeEnabled() {
			footerItems = append(footerItems, gaba.FooterHelpItem{
				ButtonName: "X",
				HelpText:   localize("button_settings", "Settings"),
			})

			if input.ShowSaveSync {
				footerItems = append(footerItems, gaba.FooterHelpItem{
					ButtonName: "Y",
					HelpText:   localize("button_sync", "Sync"),
				})
			}
		} else {
			footerItems = append(footerItems, gaba.FooterHelpItem{
				ButtonName: "B",
				HelpText:   localize("button_quit", "Quit"),
			})
		}
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "A", HelpText: localize("button_select", "Select")})
	} else {
		footerItems = []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: localize("button_back", "Back")},
			{ButtonName: "A", HelpText: localize("button_select", "Select")},
		}
	}

	// R1 downloads whatever of the focused platform is not on the device yet.
	// Which games that is only gets worked out once it is pressed, so the list
	// costs no more to draw.
	downloadMissing := !settings.IsKidModeEnabled()
	if downloadMissing {
		footerItems = append(footerItems, gaba.FooterHelpItem{
			ButtonName: "R1",
			HelpText:   localize("button_download_missing", "Download Missing"),
			Group:      gaba.FooterGroupRight,
		})
	}

	options := gaba.DefaultListOptions("Grout", menuItems)
	if !settings.IsKidModeEnabled() {
		options.ActionButton = buttons.VirtualButtonX
		if input.ShowSaveSync {
			options.SecondaryActionButton = buttons.VirtualButtonY
		}
	}
	options.ReorderButton = buttons.VirtualButtonSelect
	if downloadMissing {
		options.TertiaryActionButton = buttons.VirtualButtonR1
	}
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)

	options.StatusBar = StatusBar()

	sel, err := gaba.List(options)

	// Read the order back before handling the error, so dragging the list and
	// then pressing B still saves what was dragged.
	if sel != nil {
		output.ReorderedPlatforms = catalog.Reordered(platforms, shownPlatforms(sel.Items))
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
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition

		platform, isPlatform := sel.Items[sel.Selected[0]].Metadata.(romm.Platform)
		if !isPlatform {
			output.Action = PlatformSelectionActionCollections
			return output, nil
		}

		output.SelectedPlatform = platform
		output.Action = PlatformSelectionActionSelected
		return output, nil

	case gaba.ListActionTriggered:
		if input.QuitOnBack {
			output.Action = PlatformSelectionActionSettings
			return output, nil
		}

	case gaba.ListActionSecondaryTriggered:
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		output.Action = PlatformSelectionActionSaveSync
		return output, nil

	case gaba.ListActionTertiaryTriggered:
		output.Action = PlatformSelectionActionDownloadMissing
		if len(sel.Selected) == 0 {
			return output, nil
		}
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		// The Collections row is not a platform, and leaves SelectedPlatform
		// empty for the transition to ignore.
		if platform, ok := sel.Items[sel.Selected[0]].Metadata.(romm.Platform); ok {
			output.SelectedPlatform = platform
		}
		return output, nil
	}

	output.Action = PlatformSelectionActionQuit
	return output, nil
}

// collectionsRow marks the Collections entry, which is not a platform.
type collectionsRow struct{}

// shownPlatforms is the platforms in the order the list now holds them,
// skipping anything that is not one.
func shownPlatforms(items []gaba.MenuItem) []romm.Platform {
	platforms := make([]romm.Platform, 0, len(items))
	for _, item := range items {
		if platform, ok := item.Metadata.(romm.Platform); ok {
			platforms = append(platforms, platform)
		}
	}
	return platforms
}
