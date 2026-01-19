package ui

import (
	"errors"
	"grout/cfw"
	"grout/internal"
	"grout/internal/constants"
	"grout/romm"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type settingsVisibility struct {
	saveSyncSettings atomic.Bool
}

type SettingsInput struct {
	Config                *internal.Config
	CFW                   cfw.CFW
	Host                  romm.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SettingsOutput struct {
	Config                     *internal.Config
	GeneralSettingsClicked     bool
	InfoClicked                bool
	CollectionsSettingsClicked bool
	DirectoryMappingsClicked   bool
	AdvancedSettingsClicked    bool
	SaveSyncSettingsClicked    bool
	CheckUpdatesClicked        bool
	LastSelectedIndex          int
	LastVisibleStartIndex      int
}

type SettingsScreen struct{}

func NewSettingsScreen() *SettingsScreen {
	return &SettingsScreen{}
}

type SettingType string

const (
	SettingGeneralSettings     SettingType = "general_settings"
	SettingCollectionsSettings SettingType = "collections_settings"
	SettingDirectoryMappings   SettingType = "directory_mappings"
	SettingSaveSync            SettingType = "save_sync"
	SettingSaveSyncSettings    SettingType = "save_sync_settings"
	SettingAdvancedSettings    SettingType = "advanced_settings"
	SettingInfo                SettingType = "info"
	SettingCheckUpdates        SettingType = "check_updates"
)

var settingsOrder = []SettingType{
	SettingGeneralSettings,
	SettingCollectionsSettings,
	SettingDirectoryMappings,
	SettingSaveSync,
	SettingSaveSyncSettings,
	SettingAdvancedSettings,
	SettingInfo,
	SettingCheckUpdates,
}

func (s *SettingsScreen) Draw(input SettingsInput) (ScreenResult[SettingsOutput], error) {
	config := input.Config
	output := SettingsOutput{Config: config}

	visibility := &settingsVisibility{}
	visibility.saveSyncSettings.Store(config.SaveSyncMode != "off")

	items := s.buildMenuItems(config, visibility)

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_title", Other: "Settings"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
			StatusBar:            StatusBar(),
			SmallTitle:           true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(SettingsOutput{}), nil
		}
		return withCode(SettingsOutput{}, gaba.ExitCodeError), err
	}

	output.LastSelectedIndex = result.Selected
	output.LastVisibleStartIndex = result.VisibleStartIndex

	// Apply settings before any navigation or exit
	s.applySettings(config, result.Items)

	if result.Action == gaba.ListActionSelected {
		selectedText := items[result.Selected].Item.Text

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_general", Other: "General"}, nil) {
			output.GeneralSettingsClicked = true
			return withCode(output, constants.ExitCodeGeneralSettings), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_info", Other: "Grout Info"}, nil) {
			output.InfoClicked = true
			return withCode(output, constants.ExitCodeInfo), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_collections", Other: "Collections Settings"}, nil) {
			output.CollectionsSettingsClicked = true
			return withCode(output, constants.ExitCodeCollectionsSettings), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_edit_mappings", Other: "Directory Mappings"}, nil) {
			output.DirectoryMappingsClicked = true
			return withCode(output, constants.ExitCodeEditMappings), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_advanced", Other: "Advanced"}, nil) {
			output.AdvancedSettingsClicked = true
			return withCode(output, constants.ExitCodeAdvancedSettings), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_save_sync_settings", Other: "Save Sync Settings"}, nil) {
			output.SaveSyncSettingsClicked = true
			return withCode(output, constants.ExitCodeSaveSyncSettings), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "update_check_for_updates", Other: "Check for Updates"}, nil) {
			output.CheckUpdatesClicked = true
			return withCode(output, constants.ExitCodeCheckUpdate), nil
		}
	}

	output.Config = config
	return success(output), nil
}

func (s *SettingsScreen) buildMenuItems(config *internal.Config, visibility *settingsVisibility) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(settingsOrder))
	for _, settingType := range settingsOrder {
		items = append(items, s.buildMenuItem(settingType, config, visibility))
	}
	return items
}

func (s *SettingsScreen) buildMenuItem(settingType SettingType, config *internal.Config, visibility *settingsVisibility) gaba.ItemWithOptions {
	switch settingType {
	case SettingGeneralSettings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_general", Other: "General"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingCollectionsSettings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_collections", Other: "Collections Settings"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingDirectoryMappings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_edit_mappings", Other: "Directory Mappings"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingSaveSync:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_save_sync", Other: "Save Sync"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "save_sync_mode_off", Other: "Off"}, nil), Value: "off", OnUpdate: func(v interface{}) {
					visibility.saveSyncSettings.Store(false)
				}},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "save_sync_mode_manual", Other: "Manual"}, nil), Value: "manual", OnUpdate: func(v interface{}) {
					visibility.saveSyncSettings.Store(true)
				}},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "save_sync_mode_automatic", Other: "Automatic"}, nil), Value: "automatic", OnUpdate: func(v interface{}) {
					visibility.saveSyncSettings.Store(true)
				}},
			},
			SelectedOption: saveSyncModeToIndex(config.SaveSyncMode),
		}

	case SettingSaveSyncSettings:
		return gaba.ItemWithOptions{
			Item:        gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_save_sync_settings", Other: "Save Sync Settings"}, nil)},
			Options:     []gaba.Option{{Type: gaba.OptionTypeClickable}},
			VisibleWhen: &visibility.saveSyncSettings,
		}

	case SettingAdvancedSettings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_advanced", Other: "Advanced"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingInfo:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_info", Other: "Grout Info"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingCheckUpdates:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "update_check_for_updates", Other: "Check for Updates"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	default:
		// Should never happen, but return a safe default
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: "Unknown Setting"},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}
	}
}

func (s *SettingsScreen) applySettings(config *internal.Config, items []gaba.ItemWithOptions) {
	for _, item := range items {
		text := item.Item.Text
		switch text {
		case i18n.Localize(&goi18n.Message{ID: "settings_save_sync", Other: "Save Sync"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.SaveSyncMode = val
			}
		}
	}
}

func boolToIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

func logLevelToIndex(level string) int {
	switch level {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "ERROR":
		return 2
	default:
		return 1 // Default to INFO
	}
}

func releaseChannelToIndex(releaseChannel internal.ReleaseChannel) int {
	switch releaseChannel {
	case internal.ReleaseChannelMatchRomM:
		return 0
	case internal.ReleaseChannelStable:
		return 1
	case internal.ReleaseChannelBeta:
		return 2
	default:
		return 0
	}
}

func languageToIndex(lang string) int {
	switch lang {
	case "en":
		return 0
	case "de":
		return 1
	case "es":
		return 2
	case "fr":
		return 3
	case "it":
		return 4
	case "pt":
		return 5
	case "ru":
		return 6
	case "ja":
		return 7
	default:
		return 0
	}
}

func saveSyncModeToIndex(mode string) int {
	switch mode {
	case "off":
		return 0
	case "manual":
		return 1
	case "automatic":
		return 2
	default:
		return 0
	}
}

func collectionViewToIndex(view string) int {
	switch view {
	case "platform":
		return 0
	case "unified":
		return 1
	default:
		return 0
	}
}
