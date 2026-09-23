package ui

import (
	"errors"
	"grout/catalog"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type CollectionSelectionInput struct {
	Config               *settings.Config
	Host                 settings.Host
	SearchFilter         string
	LastSelectedIndex    int
	LastSelectedPosition int
}

type CollectionSelectionOutput struct {
	Action               CollectionListAction
	SelectedCollection   romm.Collection
	SearchFilter         string
	LastSelectedIndex    int
	LastSelectedPosition int
}

type CollectionSelectionScreen struct{}

func NewCollectionSelectionScreen() *CollectionSelectionScreen {
	return &CollectionSelectionScreen{}
}

func (s *CollectionSelectionScreen) Draw(input CollectionSelectionInput) (CollectionSelectionOutput, error) {
	output := CollectionSelectionOutput{
		Action:               CollectionListActionBack,
		SearchFilter:         input.SearchFilter,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	collections := catalog.VisibleCollections(*input.Config)

	displayCollections := collections
	if input.SearchFilter != "" {
		displayCollections = catalog.FilterCollectionsByName(collections, input.SearchFilter)
	}

	if len(displayCollections) == 0 {
		return output, nil
	}

	var menuItems []gaba.MenuItem
	for _, collection := range displayCollections {
		menuItems = append(menuItems, gaba.MenuItem{Text: collection.Name, Metadata: collection})
	}

	footerItems := []gaba.FooterHelpItem{
		FooterBack(),
		{ButtonName: "X", HelpText: localize("button_search", "Search")},
		FooterSelect(),
	}

	title := localize("collections_title", "Collections")
	if input.SearchFilter != "" {
		title = localizeWith("games_list_search_prefix", `[Search: "{{.Query}}"]`,
			map[string]any{"Query": input.SearchFilter}) + " " + title
	}

	options := gaba.DefaultListOptions(title, menuItems)
	options.ActionButton = buttons.VirtualButtonX
	options.FooterHelpItems = footerItems
	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)
	options.StatusBar = StatusBar()

	sel, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			if input.SearchFilter != "" {
				output.SearchFilter = ""
				output.LastSelectedIndex = 0
				output.LastSelectedPosition = 0
				output.Action = CollectionListActionClearSearch
				return output, nil
			}
			return output, nil
		}
		return output, err
	}

	switch sel.Action {
	case gaba.ListActionSelected:
		collection := sel.Items[sel.Selected[0]].Metadata.(romm.Collection)

		output.SelectedCollection = collection
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		output.Action = CollectionListActionSelected
		return output, nil

	case gaba.ListActionTriggered:
		output.Action = CollectionListActionSearch
		return output, nil

	default:
		return output, nil
	}
}
