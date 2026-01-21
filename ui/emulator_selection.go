package ui

import (
	"errors"
	"sort"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type EmulatorSelectionInput struct {
	PlatformFSSlug       string
	PlatformName         string
	EmulatorChoices      []EmulatorChoice
	LastSelectedIndex    int
	LastSelectedPosition int
}

type EmulatorChoice struct {
	DirectoryName    string
	DisplayName      string
	HasExistingSaves bool
	SaveCount        int
}

type EmulatorSelectionOutput struct {
	SelectedEmulator     string
	LastSelectedIndex    int
	LastSelectedPosition int
}

type EmulatorSelectionScreen struct{}

func (s *EmulatorSelectionScreen) Draw(input EmulatorSelectionInput) (EmulatorSelectionOutput, error) {
	output := EmulatorSelectionOutput{
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	sortedChoices := make([]EmulatorChoice, len(input.EmulatorChoices))

	if len(input.EmulatorChoices) > 0 {
		sortedChoices[0] = input.EmulatorChoices[0]

		rest := make([]EmulatorChoice, len(input.EmulatorChoices)-1)
		copy(rest, input.EmulatorChoices[1:])

		sort.Slice(rest, func(i, j int) bool {
			return strings.ToLower(rest[i].DirectoryName) < strings.ToLower(rest[j].DirectoryName)
		})

		copy(sortedChoices[1:], rest)
	} else {
		copy(sortedChoices, input.EmulatorChoices)
	}

	var menuItems []gaba.MenuItem
	for _, choice := range sortedChoices {
		displayText := choice.DisplayName
		if choice.HasExistingSaves {
			displayText = choice.DisplayName + i18n.Localize(&goi18n.Message{ID: "emulator_saves_count", Other: " ({{.Count}} saves)"}, map[string]interface{}{"Count": choice.SaveCount})
		}

		menuItems = append(menuItems, gaba.MenuItem{
			Text:     displayText,
			Selected: false,
			Focused:  false,
			Metadata: choice.DirectoryName,
		})
	}

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_cancel", Other: "Cancel"}, nil)},
		{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil)},
	}

	title := i18n.Localize(&goi18n.Message{ID: "emulator_selection_title", Other: "Select {{.Platform}} Emulator"}, map[string]interface{}{"Platform": input.PlatformName})
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
		selectedEmulator := sel.Items[sel.Selected[0]].Metadata.(string)

		output.SelectedEmulator = selectedEmulator
		output.LastSelectedIndex = sel.Selected[0]
		output.LastSelectedPosition = sel.VisiblePosition
		return output, nil

	default:
		return output, nil
	}
}
