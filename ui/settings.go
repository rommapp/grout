package ui

import (
	"errors"
	"grout/cfw"
	"grout/internal"
	"grout/internal/artutil"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SettingsInput struct {
	Config                *internal.Config
	CFW                   cfw.CFW
	Host                  romm.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SettingsOutput struct {
	Action                     SettingsAction
	Config                     *internal.Config
	GeneralSettingsClicked     bool
	InfoClicked                bool
	CollectionsSettingsClicked bool
	DirectoryMappingsClicked   bool
	AdvancedSettingsClicked    bool
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
	SettingAdvancedSettings    SettingType = "advanced_settings"
	SettingInfo                SettingType = "info"
	SettingCheckUpdates        SettingType = "check_updates"
)

var settingsOrder = []SettingType{
	SettingGeneralSettings,
	SettingCollectionsSettings,
	SettingDirectoryMappings,
	SettingAdvancedSettings,
	SettingInfo,
	SettingCheckUpdates,
}

func (s *SettingsScreen) Draw(input SettingsInput) (SettingsOutput, error) {
	config := input.Config
	output := SettingsOutput{Action: SettingsActionBack, Config: config}

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_title", Other: "Settings"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return SettingsOutput{Action: SettingsActionBack}, nil
		}
		return SettingsOutput{Action: SettingsActionBack}, err
	}

	output.LastSelectedIndex = result.Selected
	output.LastVisibleStartIndex = result.VisibleStartIndex

	// Apply settings before any navigation or exit
	s.applySettings(config, result.Items)

	if result.Action == gaba.ListActionSelected {
		selectedText := items[result.Selected].Item.Text

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_general", Other: "General"}, nil) {
			output.GeneralSettingsClicked = true
			output.Action = SettingsActionGeneral
			return output, nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_info", Other: "Grout Info"}, nil) {
			output.InfoClicked = true
			output.Action = SettingsActionInfo
			return output, nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_collections", Other: "Collections Settings"}, nil) {
			output.CollectionsSettingsClicked = true
			output.Action = SettingsActionCollections
			return output, nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_edit_mappings", Other: "Directory Mappings"}, nil) {
			output.DirectoryMappingsClicked = true
			output.Action = SettingsActionPlatformMapping
			return output, nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_advanced", Other: "Advanced"}, nil) {
			output.AdvancedSettingsClicked = true
			output.Action = SettingsActionAdvanced
			return output, nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "update_check_for_updates", Other: "Check for Updates"}, nil) {
			output.CheckUpdatesClicked = true
			output.Action = SettingsActionCheckUpdate
			return output, nil
		}
	}

	output.Config = config
	output.Action = SettingsActionSaved
	return output, nil
}

func (s *SettingsScreen) buildMenuItems(config *internal.Config) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(settingsOrder))
	for _, settingType := range settingsOrder {
		items = append(items, s.buildMenuItem(settingType))
	}
	return items
}

func (s *SettingsScreen) buildMenuItem(settingType SettingType) gaba.ItemWithOptions {
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

func (s *SettingsScreen) applySettings(_ *internal.Config, _ []gaba.ItemWithOptions) {
	// No toggle settings remain on the settings screen
}

func boolToIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

func logLevelToIndex(level internal.LogLevel) int {
	switch level {
	case internal.LogLevelDebug:
		return 0
	case internal.LogLevelInfo:
		return 1
	case internal.LogLevelError:
		return 2
	default:
		return 1
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

func boxArtToIndex(boxArt artutil.ArtKind) int {
	switch boxArt {
	case artutil.ArtKindDefault:
		return 0
	case artutil.ArtKindBox2D:
		return 1
	case artutil.ArtKindBox3D:
		return 2
	case artutil.ArtKindMixImage:
		return 3
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

func collectionViewToIndex(view internal.CollectionView) int {
	switch view {
	case internal.CollectionViewPlatform:
		return 0
	case internal.CollectionViewUnified:
		return 1
	default:
		return 0
	}
}
