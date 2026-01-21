package ui

import (
	"errors"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"sort"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SaveSyncSettingsInput struct {
	Config *internal.Config
	CFW    cfw.CFW
}

type SaveSyncSettingsOutput struct {
	Action SaveSyncSettingsAction
	Config *internal.Config
}

type SaveSyncSettingsScreen struct {
	displayToFSSlug map[string]string
}

func NewSaveSyncSettingsScreen() *SaveSyncSettingsScreen {
	return &SaveSyncSettingsScreen{}
}

func (s *SaveSyncSettingsScreen) Draw(input SaveSyncSettingsInput) (SaveSyncSettingsOutput, error) {
	config := input.Config
	output := SaveSyncSettingsOutput{Action: SaveSyncSettingsActionBack, Config: config}

	items := s.buildMenuItems(config)

	if len(items) == 0 {
		gaba.GetLogger().Warn("No platforms configured for save sync settings")
		return output, nil
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "save_sync_settings_title", Other: "Save Sync Settings"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{
				FooterBack(),
				FooterCycle(),
				FooterSave(),
			},
			InitialSelectedIndex: 0,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Save sync settings error", "error", err)
		return output, err
	}

	s.applySettings(config, result.Items)

	err = internal.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving save sync settings", "error", err)
		return output, err
	}

	output.Action = SaveSyncSettingsActionSaved
	return output, nil
}

func (s *SaveSyncSettingsScreen) buildMenuItems(config *internal.Config) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0)
	s.displayToFSSlug = make(map[string]string)

	// Build a map of fsSlug -> platform display name from cache
	platformNames := make(map[string]string)
	if cm := cache.GetCacheManager(); cm != nil {
		if platforms, err := cm.GetPlatforms(); err == nil {
			for _, p := range platforms {
				platformNames[p.FSSlug] = p.Name
			}
		}
	}

	// Get all platform fsSlugs from directory mappings
	fsSlugs := make([]string, 0, len(config.DirectoryMappings))
	for fsSlug := range config.DirectoryMappings {
		fsSlugs = append(fsSlugs, fsSlug)
	}
	sort.Strings(fsSlugs)

	for _, fsSlug := range fsSlugs {
		// Resolve fsSlug through platform binding for CFW lookup
		effectiveFSSlug := config.ResolveFSSlug(fsSlug)
		saveDirectories := cfw.EmulatorFoldersForFSSlug(effectiveFSSlug)
		if len(saveDirectories) == 0 {
			continue
		}

		options := make([]gaba.Option, 0, len(saveDirectories)+1)

		options = append(options, gaba.Option{
			DisplayName: i18n.Localize(&goi18n.Message{ID: "common_default", Other: "Default"}, nil),
			Value:       "",
		})

		for _, dir := range saveDirectories {
			dn := dir

			if cfw.GetCFW() == cfw.MuOS {
				dn = strings.ReplaceAll(dn, "file/", "")   // remove leading file path (caused by fixing #89)
				dn = strings.ReplaceAll(dn, "/backup", "") // remove trailing backup path (caused by fixing #89)
			}

			options = append(options, gaba.Option{
				DisplayName: dn,
				Value:       dir,
			})
		}

		// Determine currently selected option
		selectedIndex := 0
		if config.SaveDirectoryMappings != nil {
			if currentMapping, ok := config.SaveDirectoryMappings[fsSlug]; ok && currentMapping != "" {
				for i, opt := range options {
					if val, ok := opt.Value.(string); ok && val == currentMapping {
						selectedIndex = i
						break
					}
				}
			}
		}

		// Use platform display name if available, otherwise fall back to fsSlug
		displayName := fsSlug
		if name, ok := platformNames[fsSlug]; ok {
			displayName = name
		}

		// Store mapping from display name to fsSlug for applying settings
		s.displayToFSSlug[displayName] = fsSlug

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: displayName},
			Options:        options,
			SelectedOption: selectedIndex,
		})
	}

	return items
}

func (s *SaveSyncSettingsScreen) applySettings(config *internal.Config, items []gaba.ItemWithOptions) {
	if config.SaveDirectoryMappings == nil {
		config.SaveDirectoryMappings = make(map[string]string)
	}

	for _, item := range items {
		// Look up fsSlug from display name
		fsSlug, ok := s.displayToFSSlug[item.Item.Text]
		if !ok {
			continue
		}
		if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
			if val == "" {
				// Remove from map if set to default
				delete(config.SaveDirectoryMappings, fsSlug)
			} else {
				config.SaveDirectoryMappings[fsSlug] = val
			}
		}
	}
}
