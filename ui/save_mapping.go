package ui

import (
	"errors"
	"grout/cfw"
	"grout/internal"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SaveMappingInput struct {
	Config *internal.Config
}

type SaveMappingOutput struct {
	Action SaveMappingAction
	Config *internal.Config
}

type SaveMappingScreen struct{}

func NewSaveMappingScreen() *SaveMappingScreen {
	return &SaveMappingScreen{}
}

func (s *SaveMappingScreen) Draw(input SaveMappingInput) (SaveMappingOutput, error) {
	output := SaveMappingOutput{
		Action: SaveMappingActionBack,
		Config: input.Config,
	}

	currentCFW := cfw.GetCFW()
	emulatorMap := cfw.EmulatorFolderMap(currentCFW)
	if emulatorMap == nil {
		return output, nil
	}

	// Only show platforms the user has configured (from directory mappings)
	configuredPlatforms := input.Config.DirectoryMappings
	if len(configuredPlatforms) == 0 {
		return output, nil
	}

	var items []gaba.ItemWithOptions

	for fsSlug := range configuredPlatforms {
		effectiveFSSlug := input.Config.ResolveFSSlug(fsSlug)
		emulatorDirs, ok := emulatorMap[effectiveFSSlug]
		if !ok || len(emulatorDirs) < 2 {
			// Skip platforms with only one emulator option — no choice to make
			continue
		}

		options := make([]gaba.Option, 0, len(emulatorDirs))
		selectedIndex := 0

		for i, dir := range emulatorDirs {
			displayName := filepath.Base(dir)
			options = append(options, gaba.Option{
				DisplayName: displayName,
				Value:       dir,
			})

			// Check if this matches the current mapping
			if input.Config.SaveDirectoryMappings != nil {
				if mapped, exists := input.Config.SaveDirectoryMappings[fsSlug]; exists && mapped == dir {
					selectedIndex = i
				}
			}
		}

		platformName := configuredPlatforms[fsSlug].RomMSlug
		if platformName == "" {
			platformName = fsSlug
		}

		items = append(items, gaba.ItemWithOptions{
			Item: gaba.MenuItem{
				Text:     platformName,
				Metadata: fsSlug,
			},
			Options:        options,
			SelectedOption: selectedIndex,
		})
	}

	if len(items) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "save_mapping_no_platforms", Other: "No platforms with multiple emulators found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "save_mapping_title", Other: "Save Sync Mappings"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	// Apply selections to config
	if output.Config.SaveDirectoryMappings == nil {
		output.Config.SaveDirectoryMappings = make(map[string]string)
	}

	for _, item := range result.Items {
		fsSlug := item.Item.Metadata.(string)
		selectedDir := item.Options[item.SelectedOption].Value.(string)

		effectiveFSSlug := input.Config.ResolveFSSlug(fsSlug)
		emulatorDirs := emulatorMap[effectiveFSSlug]

		if len(emulatorDirs) > 0 && selectedDir == emulatorDirs[0] {
			// Default selection — remove mapping so the default is used
			delete(output.Config.SaveDirectoryMappings, fsSlug)
		} else {
			output.Config.SaveDirectoryMappings[fsSlug] = selectedDir
		}
	}

	// Clean up empty map
	if len(output.Config.SaveDirectoryMappings) == 0 {
		output.Config.SaveDirectoryMappings = nil
	}

	output.Action = SaveMappingActionSaved
	return output, nil
}
