package ui

import (
	"errors"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"path/filepath"
	"sort"
	"strings"

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
	logger := gaba.GetLogger()
	output := SaveMappingOutput{
		Action: SaveMappingActionBack,
		Config: input.Config,
	}

	currentCFW := cfw.GetCFW()
	logger.Debug("Save mapping: detected CFW", "cfw", currentCFW)
	emulatorMap := cfw.EmulatorFolderMap(currentCFW)
	if emulatorMap == nil {
		logger.Debug("Save mapping: emulator map is nil, returning")
		return output, nil
	}
	logger.Debug("Save mapping: emulator map loaded", "entries", len(emulatorMap))

	// Only show platforms the user has configured (from directory mappings)
	configuredPlatforms := input.Config.DirectoryMappings
	if len(configuredPlatforms) == 0 {
		logger.Debug("Save mapping: no configured platforms, returning")
		return output, nil
	}
	logger.Debug("Save mapping: configured platforms", "count", len(configuredPlatforms))

	// Build a map of fsSlug -> platform display name from cache
	platformNames := make(map[string]string)
	if cm := cache.GetCacheManager(); cm != nil {
		if platforms, err := cm.GetPlatforms(); err == nil {
			for _, p := range platforms {
				platformNames[p.FSSlug] = p.Name
			}
		}
	}

	var items []gaba.ItemWithOptions

	for fsSlug := range configuredPlatforms {
		effectiveFSSlug := input.Config.ResolveFSSlug(fsSlug)
		emulatorDirs, ok := emulatorMap[effectiveFSSlug]
		if !ok || len(emulatorDirs) < 2 {
			logger.Debug("Save mapping: skipping platform", "fsSlug", fsSlug, "effectiveFSSlug", effectiveFSSlug, "found", ok, "emulatorDirs", len(emulatorDirs))
			// Skip platforms with only one emulator option — no choice to make
			continue
		}
		logger.Debug("Save mapping: processing platform", "fsSlug", fsSlug, "effectiveFSSlug", effectiveFSSlug, "emulatorDirs", emulatorDirs)

		options := make([]gaba.Option, 0, len(emulatorDirs))
		selectedIndex := 0

		for i, dir := range emulatorDirs {
			displayName := dir

			if currentCFW == cfw.MuOS {
				displayName = strings.ReplaceAll(displayName, "file/", "")
				displayName = strings.ReplaceAll(displayName, "/backup", "")
			}

			displayName = filepath.Base(displayName)
			options = append(options, gaba.Option{
				DisplayName: displayName,
				Value:       dir,
			})

			// Check if this matches the current mapping
			if input.Config.SaveDirectoryMappings != nil {
				if mapped, exists := input.Config.SaveDirectoryMappings[fsSlug]; exists && mapped == dir {
					logger.Debug("Save mapping: found existing mapping", "fsSlug", fsSlug, "dir", dir, "index", i)
					selectedIndex = i
				}
			}
		}

		platformName := fsSlug
		if name, ok := platformNames[fsSlug]; ok {
			platformName = name
		} else if slug := configuredPlatforms[fsSlug].RomMSlug; slug != "" {
			platformName = slug
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

	sort.Slice(items, func(i, j int) bool {
		return items[i].Item.Text < items[j].Item.Text
	})

	logger.Debug("Save mapping: total items to display", "count", len(items))

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
		logger.Debug("Save mapping: user selection", "fsSlug", fsSlug, "selectedDir", selectedDir)

		effectiveFSSlug := input.Config.ResolveFSSlug(fsSlug)
		emulatorDirs := emulatorMap[effectiveFSSlug]

		if len(emulatorDirs) > 0 && selectedDir == emulatorDirs[0] {
			// Default selection — remove mapping so the default is used
			logger.Debug("Save mapping: removing mapping (default selected)", "fsSlug", fsSlug)
			delete(output.Config.SaveDirectoryMappings, fsSlug)
		} else {
			logger.Debug("Save mapping: setting mapping", "fsSlug", fsSlug, "dir", selectedDir)
			output.Config.SaveDirectoryMappings[fsSlug] = selectedDir
		}
	}

	// Clean up empty map
	if len(output.Config.SaveDirectoryMappings) == 0 {
		output.Config.SaveDirectoryMappings = nil
	}

	logger.Debug("Save mapping: final mappings", "mappings", output.Config.SaveDirectoryMappings)
	output.Action = SaveMappingActionSaved
	return output, nil
}
