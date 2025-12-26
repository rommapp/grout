package ui

import (
	"errors"
	"grout/constants"
	"grout/romm"
	"grout/utils"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

type SettingsInput struct {
	Config                *utils.Config
	CFW                   constants.CFW
	Host                  romm.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SettingsOutput struct {
	Config                     *utils.Config
	EditMappingsClicked        bool
	InfoClicked                bool
	SyncArtworkClicked         bool
	CollectionsSettingsClicked bool
	AdvancedSettingsClicked    bool
	LastSelectedIndex          int
	LastVisibleStartIndex      int
}

type SettingsScreen struct{}

func NewSettingsScreen() *SettingsScreen {
	return &SettingsScreen{}
}

type SettingType string

const (
	SettingEditMappings        SettingType = "edit_mappings"
	SettingGameDetails         SettingType = "game_details"
	SettingCollections         SettingType = "collections"
	SettingSmartCollections    SettingType = "smart_collections"
	SettingVirtualCollections  SettingType = "virtual_collections"
	SettingCollectionView      SettingType = "collection_view"
	SettingCollectionsSettings SettingType = "collections_settings"
	SettingAdvancedSettings    SettingType = "advanced_settings"
	SettingDownloadedGames     SettingType = "downloaded_games"
	SettingSaveSync            SettingType = "save_sync"
	SettingShowBIOS            SettingType = "show_bios"
	SettingDownloadArt         SettingType = "download_art"
	SettingBoxArt              SettingType = "box_art"
	SettingSyncArtwork         SettingType = "sync_artwork"
	SettingUnzipDownloads      SettingType = "unzip_downloads"
	SettingDownloadTimeout     SettingType = "download_timeout"
	SettingAPITimeout          SettingType = "api_timeout"
	SettingLanguage            SettingType = "language"
	SettingLogLevel            SettingType = "log_level"
	SettingInfo                SettingType = "info"
)

var settingsOrder = []SettingType{

	SettingBoxArt,
	SettingGameDetails,
	SettingShowBIOS,

	SettingDownloadedGames,
	SettingDownloadArt,
	SettingUnzipDownloads,

	SettingSaveSync,

	SettingCollectionsSettings,
	SettingSyncArtwork,

	SettingLanguage,
	SettingAdvancedSettings,
}

var (
	apiTimeoutOptions = []struct {
		I18nKey string
		Value   time.Duration
	}{
		{"time_15_seconds", 15 * time.Second},
		{"time_30_seconds", 30 * time.Second},
		{"time_45_seconds", 45 * time.Second},
		{"time_60_seconds", 60 * time.Second},
		{"time_75_seconds", 75 * time.Second},
		{"time_90_seconds", 90 * time.Second},
		{"time_120_seconds", 120 * time.Second},
		{"time_180_seconds", 180 * time.Second},
		{"time_240_seconds", 240 * time.Second},
		{"time_300_seconds", 300 * time.Second},
	}

	downloadTimeoutOptions = []struct {
		I18nKey string
		Value   time.Duration
	}{
		{"time_15_minutes", 15 * time.Minute},
		{"time_30_minutes", 30 * time.Minute},
		{"time_45_minutes", 45 * time.Minute},
		{"time_60_minutes", 60 * time.Minute},
		{"time_75_minutes", 75 * time.Minute},
		{"time_90_minutes", 90 * time.Minute},
		{"time_105_minutes", 105 * time.Minute},
		{"time_120_minutes", 120 * time.Minute},
	}
)

func (s *SettingsScreen) Draw(input SettingsInput) (ScreenResult[SettingsOutput], error) {
	config := input.Config
	output := SettingsOutput{Config: config}

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		i18n.GetString("settings_title"),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{
				{ButtonName: "B", HelpText: i18n.GetString("button_cancel")},
				{ButtonName: "←→", HelpText: i18n.GetString("button_cycle")},
				{ButtonName: "Start", HelpText: i18n.GetString("button_save")},
			},
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
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

	if result.Action == gaba.ListActionSelected {
		selectedText := items[result.Selected].Item.Text

		if selectedText == i18n.GetString("settings_sync_artwork") {
			output.SyncArtworkClicked = true
			return withCode(output, constants.ExitCodeSyncArtwork), nil
		}

		if selectedText == i18n.GetString("settings_edit_mappings") {
			output.EditMappingsClicked = true
			return withCode(output, constants.ExitCodeEditMappings), nil
		}

		if selectedText == i18n.GetString("settings_info") {
			output.InfoClicked = true
			return withCode(output, constants.ExitCodeInfo), nil
		}

		if selectedText == i18n.GetString("settings_collections") {
			output.CollectionsSettingsClicked = true
			return withCode(output, constants.ExitCodeCollectionsSettings), nil
		}

		if selectedText == i18n.GetString("settings_advanced") {
			output.AdvancedSettingsClicked = true
			return withCode(output, constants.ExitCodeAdvancedSettings), nil
		}
	}

	s.applySettings(config, result.Items)

	output.Config = config
	return success(output), nil
}

func (s *SettingsScreen) buildMenuItems(config *utils.Config) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(settingsOrder))
	for _, settingType := range settingsOrder {
		items = append(items, s.buildMenuItem(settingType, config))
	}
	return items
}

func (s *SettingsScreen) buildMenuItem(settingType SettingType, config *utils.Config) gaba.ItemWithOptions {
	switch settingType {
	case SettingEditMappings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.GetString("settings_edit_mappings")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingGameDetails:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_show_game_details")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_show"), Value: true},
				{DisplayName: i18n.GetString("common_hide"), Value: false},
			},
			SelectedOption: boolToIndex(!config.GameDetails),
		}

	case SettingCollections:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_show_collections")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_show"), Value: true},
				{DisplayName: i18n.GetString("common_hide"), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowCollections),
		}

	case SettingSmartCollections:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_show_smart_collections")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_show"), Value: true},
				{DisplayName: i18n.GetString("common_hide"), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowSmartCollections),
		}

	case SettingVirtualCollections:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_show_virtual_collections")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_show"), Value: true},
				{DisplayName: i18n.GetString("common_hide"), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowVirtualCollections),
		}

	case SettingCollectionView:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_collection_view")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("collection_view_platform"), Value: "platform"},
				{DisplayName: i18n.GetString("collection_view_unified"), Value: "unified"},
			},
			SelectedOption: collectionViewToIndex(config.CollectionView),
		}

	case SettingCollectionsSettings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.GetString("settings_collections")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingDownloadedGames:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_downloaded_games")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("downloaded_games_do_nothing"), Value: "do_nothing"},
				{DisplayName: i18n.GetString("downloaded_games_mark"), Value: "mark"},
				{DisplayName: i18n.GetString("downloaded_games_filter"), Value: "filter"},
			},
			SelectedOption: s.downloadedGamesActionToIndex(config.DownloadedGames),
		}

	case SettingSaveSync:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_save_sync")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("save_sync_mode_off"), Value: "off"},
				{DisplayName: i18n.GetString("save_sync_mode_manual"), Value: "manual"},
				// {DisplayName: i18n.GetString("save_sync_mode_daemon"), Value: "daemon"},
			},
			SelectedOption: saveSyncModeToIndex(config.SaveSyncMode),
		}

	case SettingShowBIOS:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_show_bios")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_show"), Value: true},
				{DisplayName: i18n.GetString("common_hide"), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowBIOSDownload),
		}

	case SettingDownloadArt:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_download_art")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_true"), Value: true},
				{DisplayName: i18n.GetString("common_false"), Value: false},
			},
			SelectedOption: boolToIndex(!config.DownloadArt),
		}

	case SettingBoxArt:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_box_art")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_show"), Value: true},
				{DisplayName: i18n.GetString("common_hide"), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowBoxArt),
		}

	case SettingSyncArtwork:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.GetString("settings_sync_artwork")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingUnzipDownloads:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_unzip_downloads")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("common_true"), Value: true},
				{DisplayName: i18n.GetString("common_false"), Value: false},
			},
			SelectedOption: boolToIndex(!config.UnzipDownloads),
		}

	case SettingDownloadTimeout:
		return gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: i18n.GetString("settings_download_timeout")},
			Options:        s.buildDownloadTimeoutOptions(),
			SelectedOption: s.findDownloadTimeoutIndex(config.DownloadTimeout),
		}

	case SettingAPITimeout:
		return gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: i18n.GetString("settings_api_timeout")},
			Options:        s.buildApiTimeoutOptions(),
			SelectedOption: s.findApiTimeoutIndex(config.ApiTimeout),
		}

	case SettingLanguage:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_language")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("settings_language_english"), Value: "en"},
				{DisplayName: i18n.GetString("settings_language_german"), Value: "de"},
				{DisplayName: i18n.GetString("settings_language_spanish"), Value: "es"},
				{DisplayName: i18n.GetString("settings_language_french"), Value: "fr"},
				{DisplayName: i18n.GetString("settings_language_italian"), Value: "it"},
				{DisplayName: i18n.GetString("settings_language_portuguese"), Value: "pt"},
				{DisplayName: i18n.GetString("settings_language_russian"), Value: "ru"},
				{DisplayName: i18n.GetString("settings_language_japanese"), Value: "ja"},
			},
			SelectedOption: languageToIndex(config.Language),
		}

	case SettingLogLevel:
		return gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: i18n.GetString("settings_log_level")},
			Options: []gaba.Option{
				{DisplayName: i18n.GetString("log_level_debug"), Value: "DEBUG"},
				{DisplayName: i18n.GetString("log_level_error"), Value: "ERROR"},
			},
			SelectedOption: logLevelToIndex(config.LogLevel),
		}

	case SettingInfo:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.GetString("settings_info")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		}

	case SettingAdvancedSettings:
		return gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: i18n.GetString("settings_advanced")},
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

func (s *SettingsScreen) buildApiTimeoutOptions() []gaba.Option {
	options := make([]gaba.Option, len(apiTimeoutOptions))
	for i, opt := range apiTimeoutOptions {
		options[i] = gaba.Option{DisplayName: i18n.GetString(opt.I18nKey), Value: opt.Value}
	}
	return options
}

func (s *SettingsScreen) buildDownloadTimeoutOptions() []gaba.Option {
	options := make([]gaba.Option, len(downloadTimeoutOptions))
	for i, opt := range downloadTimeoutOptions {
		options[i] = gaba.Option{DisplayName: i18n.GetString(opt.I18nKey), Value: opt.Value}
	}
	return options
}

func (s *SettingsScreen) findApiTimeoutIndex(timeout time.Duration) int {
	for i, opt := range apiTimeoutOptions {
		if opt.Value == timeout {
			return i
		}
	}
	return 0
}

func (s *SettingsScreen) findDownloadTimeoutIndex(timeout time.Duration) int {
	for i, opt := range downloadTimeoutOptions {
		if opt.Value == timeout {
			return i
		}
	}
	return 0
}

func (s *SettingsScreen) applySettings(config *utils.Config, items []gaba.ItemWithOptions) {
	for _, item := range items {
		text := item.Item.Text
		switch text {
		case i18n.GetString("settings_download_art"):
			config.DownloadArt = item.SelectedOption == 0
		case i18n.GetString("settings_box_art"):
			config.ShowBoxArt = item.SelectedOption == 0
		case i18n.GetString("settings_auto_sync_saves"):
			config.AutoSyncSaves = item.SelectedOption == 0
		case i18n.GetString("settings_save_sync"):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.SaveSyncMode = val
			}
		case i18n.GetString("settings_show_bios"):
			config.ShowBIOSDownload = item.SelectedOption == 0
		case i18n.GetString("settings_unzip_downloads"):
			config.UnzipDownloads = item.SelectedOption == 0
		case i18n.GetString("settings_show_game_details"):
			config.GameDetails = item.SelectedOption == 0
		case i18n.GetString("settings_show_collections"):
			config.ShowCollections = item.SelectedOption == 0
		case i18n.GetString("settings_show_smart_collections"):
			config.ShowSmartCollections = item.SelectedOption == 0
		case i18n.GetString("settings_show_virtual_collections"):
			config.ShowVirtualCollections = item.SelectedOption == 0
		case i18n.GetString("settings_api_timeout"):
			idx := item.SelectedOption
			if idx < len(apiTimeoutOptions) {
				config.ApiTimeout = apiTimeoutOptions[idx].Value
			}
		case i18n.GetString("settings_download_timeout"):
			idx := item.SelectedOption
			if idx < len(downloadTimeoutOptions) {
				config.DownloadTimeout = downloadTimeoutOptions[idx].Value
			}
		case i18n.GetString("settings_log_level"):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.LogLevel = val
			}
		case i18n.GetString("settings_language"):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.Language = val
			}
		case i18n.GetString("settings_downloaded_games"):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.DownloadedGames = val
			}
		case i18n.GetString("settings_collection_view"):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.CollectionView = val
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
	case "ERROR":
		return 1
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
	// case "daemon":
	// 	return 2
	default:
		return 0
	}
}

func (s *SettingsScreen) downloadedGamesActionToIndex(action string) int {
	switch action {
	case "do_nothing":
		return 0
	case "mark":
		return 1
	case "filter":
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
