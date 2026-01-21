package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/romm"
	"slices"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type CollectionSelectionInput struct {
	Config               *internal.Config
	Host                 romm.Host
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

	// Try to get collections from cache first
	var collections []romm.Collection
	cm := cache.GetCacheManager()

	if cm != nil && cm.HasCollections() {
		// Load from cache, filtering by enabled types
		if input.Config.ShowRegularCollections {
			if regular, err := cm.GetCollectionsByType("regular"); err == nil {
				collections = append(collections, regular...)
			}
		}
		if input.Config.ShowSmartCollections {
			if smart, err := cm.GetCollectionsByType("smart"); err == nil {
				collections = append(collections, smart...)
			}
		}
		if input.Config.ShowVirtualCollections {
			if virtual, err := cm.GetCollectionsByType("virtual"); err == nil {
				collections = append(collections, virtual...)
			}
		}

		// Filter collections to only show those with games from mapped platforms
		cachedGameIDs := cm.GetCachedGameIDs()
		if len(cachedGameIDs) > 0 {
			filteredCollections := make([]romm.Collection, 0, len(collections))
			for _, coll := range collections {
				// Check if any of the collection's ROM IDs are in cached games
				for _, romID := range coll.ROMIDs {
					if cachedGameIDs[romID] {
						filteredCollections = append(filteredCollections, coll)
						break
					}
				}
			}
			collections = filteredCollections
		}
	}

	// Sort collections alphabetically
	slices.SortFunc(collections, func(a, b romm.Collection) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	displayCollections := collections
	if input.SearchFilter != "" {
		filteredCollections := make([]romm.Collection, 0)
		for _, collection := range collections {
			if strings.Contains(strings.ToLower(collection.Name), strings.ToLower(input.SearchFilter)) {
				filteredCollections = append(filteredCollections, collection)
			}
		}
		displayCollections = filteredCollections
	}

	if len(displayCollections) == 0 {
		return output, nil
	}

	var menuItems []gaba.MenuItem
	for _, collection := range displayCollections {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:     collection.Name,
			Selected: false,
			Focused:  false,
			Metadata: collection,
		})
	}

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
		{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_search", Other: "Search"}, nil)},
		{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)},
	}

	title := "Collections"
	if input.SearchFilter != "" {
		title = fmt.Sprintf("[Search: \"%s\"] | Collections", input.SearchFilter)
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
