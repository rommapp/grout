package ui

import (
	"errors"

	"grout/saves"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SaveMappingInput struct {
	Config *settings.Config
}

type SaveMappingOutput struct {
	Action SaveMappingAction
	Config *settings.Config
}

type SaveMappingScreen struct{}

func NewSaveMappingScreen() *SaveMappingScreen {
	return &SaveMappingScreen{}
}

// Draw asks which emulator's folder each platform's saves live in, for the
// platforms whose firmware keeps them in more than one place.
func (s *SaveMappingScreen) Draw(input SaveMappingInput) (SaveMappingOutput, error) {
	output := SaveMappingOutput{Action: SaveMappingActionBack, Config: input.Config}

	choices := saves.EmulatorChoices(*input.Config)
	if len(choices) == 0 {
		gaba.ConfirmationMessage(
			localize("save_mapping_no_platforms", "No platforms with multiple emulators found."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	result, err := gaba.OptionsList(
		localize("save_mapping_title", "Save Sync Mappings"),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		emulatorItems(choices),
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	byslug := make(map[string]saves.EmulatorChoice, len(choices))
	for _, choice := range choices {
		byslug[choice.FSSlug] = choice
	}

	for _, item := range result.Items {
		fsSlug, ok := item.Item.Metadata.(string)
		if !ok {
			continue
		}
		choice, known := byslug[fsSlug]
		if !known || item.SelectedOption < 0 || item.SelectedOption >= len(item.Options) {
			continue
		}

		directory, _ := item.Options[item.SelectedOption].Value.(string)
		saves.ChooseEmulator(input.Config, fsSlug, directory, choice.Directories)
	}

	output.Action = SaveMappingActionSaved
	return output, nil
}

func emulatorItems(choices []saves.EmulatorChoice) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(choices))

	for _, choice := range choices {
		options := make([]gaba.Option, 0, len(choice.Directories))
		for i, dir := range choice.Directories {
			options = append(options, gaba.Option{DisplayName: choice.Labels[i], Value: dir})
		}

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: choice.Name, Metadata: choice.FSSlug},
			Options:        options,
			SelectedOption: optionIndex(options, choice.Chosen),
		})
	}

	return items
}
