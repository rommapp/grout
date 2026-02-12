package ui

import (
	"errors"
	"grout/cfw"
	"grout/internal"
	"grout/internal/artutil"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type GeneralSettingsInput struct {
	Config *internal.Config
}

type GeneralSettingsOutput struct {
	Action GeneralSettingsAction
	Config *internal.Config
}

type GeneralSettingsScreen struct{}

func NewGeneralSettingsScreen() *GeneralSettingsScreen {
	return &GeneralSettingsScreen{}
}

func (s *GeneralSettingsScreen) Draw(input GeneralSettingsInput) (GeneralSettingsOutput, error) {
	config := input.Config
	output := GeneralSettingsOutput{Action: GeneralSettingsActionBack, Config: config}

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_general", Other: "General"}, nil),
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
		gaba.GetLogger().Error("General settings error", "error", err)
		return output, err
	}

	s.applySettings(config, result.Items)

	err = internal.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving general settings", "error", err)
		return output, err
	}

	output.Action = GeneralSettingsActionSaved
	return output, nil
}

func (s *GeneralSettingsScreen) buildMenuItems(config *internal.Config) []gaba.ItemWithOptions {
	showArtKind := atomic.Bool{}
	showArtKind.Store(config.DownloadArt)

	downloadArtUpdateFunc := func(val interface{}) {
		showArtKind.Store(val.(bool))
	}

	isMuOS := atomic.Bool{}
	isMuOS.Store(cfw.GetCFW() == cfw.MuOS)
	displayDownloadArtPreview := atomic.Bool{}
	displayDownloadArtPreview.Store(showArtKind.Load() && isMuOS.Load())

	return []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_box_art", Other: "Box Art"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_show", Other: "Show"}, nil), Value: true},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_hide", Other: "Hide"}, nil), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowBoxArt),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_downloaded_games", Other: "Downloaded Games"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "downloaded_games_do_nothing", Other: "Do Nothing"}, nil), Value: internal.DownloadedGamesModeDoNothing},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "downloaded_games_mark", Other: "Mark"}, nil), Value: internal.DownloadedGamesModeMark},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "downloaded_games_filter", Other: "Filter"}, nil), Value: internal.DownloadedGamesModeFilter},
			},
			SelectedOption: downloadedGamesActionToIndex(config.DownloadedGames),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_compressed_downloads", Other: "Archived Downloads"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_compressed_downloads_uncompress", Other: "Uncompress"}, nil), Value: true},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_compressed_downloads_do_nothing", Other: "Do Nothing"}, nil), Value: false},
			},
			SelectedOption: boolToIndex(!config.UnzipDownloads),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_download_art", Other: "Download Art"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_true", Other: "True"}, nil), Value: true, OnUpdate: downloadArtUpdateFunc},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_false", Other: "False"}, nil), Value: false, OnUpdate: downloadArtUpdateFunc},
			},
			SelectedOption: boolToIndex(!config.DownloadArt),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind", Other: "Download Art Kind"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_default", Other: "Default"}, nil), Value: artutil.ArtKindDefault},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_box2d", Other: "Box2D"}, nil), Value: artutil.ArtKindBox2D},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_box3d", Other: "Box3D"}, nil), Value: artutil.ArtKindBox3D},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_miximage", Other: "MixImage"}, nil), Value: artutil.ArtKindMixImage},
			},
			SelectedOption: boxArtToIndex(config.ArtKind),
			VisibleWhen:    &showArtKind,
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_download_art_preview", Other: "Download Screenshot Preview"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_true", Other: "True"}, nil), Value: true},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_false", Other: "False"}, nil), Value: false},
			},
			SelectedOption: boolToIndex(!config.DownloadArtScreenshotPreview),
			VisibleWhen:    &displayDownloadArtPreview,
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_download_art_splash", Other: "Download Splash Art"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_none", Other: "None"}, nil), Value: artutil.ArtKindNone},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_marquee", Other: "Marquee"}, nil), Value: artutil.ArtKindMarquee},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind_title", Other: "Title"}, nil), Value: artutil.ArtKindTitle},
			},
			SelectedOption: boxArtToIndex(config.DownloadSplashArt),
			VisibleWhen:    &displayDownloadArtPreview,
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_language", Other: "Language"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_english", Other: "English"}, nil), Value: "en"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_german", Other: "Deutsch"}, nil), Value: "de"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_spanish", Other: "Español"}, nil), Value: "es"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_french", Other: "Français"}, nil), Value: "fr"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_italian", Other: "Italiano"}, nil), Value: "it"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_portuguese", Other: "Português"}, nil), Value: "pt"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_russian", Other: "Русский"}, nil), Value: "ru"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_language_japanese", Other: "日本語"}, nil), Value: "ja"},
			},
			SelectedOption: languageToIndex(config.Language),
		},
	}
}

func (s *GeneralSettingsScreen) applySettings(config *internal.Config, items []gaba.ItemWithOptions) {
	for _, item := range items {
		selectedText := item.Item.Text

		switch selectedText {
		case i18n.Localize(&goi18n.Message{ID: "settings_box_art", Other: "Box Art"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.ShowBoxArt = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_downloaded_games", Other: "Downloaded Games"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(internal.DownloadedGamesMode); ok {
				config.DownloadedGames = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_download_art", Other: "Download Art"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.DownloadArt = val
			}
		case i18n.Localize(&goi18n.Message{ID: "settings_download_art_kind", Other: "Download Art Kind"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(artutil.ArtKind); ok {
				config.ArtKind = val
			}
		case i18n.Localize(&goi18n.Message{ID: "settings_download_art_preview", Other: "Download Screenshot Preview"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.DownloadArtScreenshotPreview = val
			}
		case i18n.Localize(&goi18n.Message{ID: "settings_download_art_splash", Other: "Download Splash Art"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(artutil.ArtKind); ok {
				config.DownloadSplashArt = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_compressed_downloads", Other: "Archived Downloads"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.UnzipDownloads = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_language", Other: "Language"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.Language = val
			}
		}
	}
}

func downloadedGamesActionToIndex(action internal.DownloadedGamesMode) int {
	switch action {
	case internal.DownloadedGamesModeDoNothing:
		return 0
	case internal.DownloadedGamesModeMark:
		return 1
	case internal.DownloadedGamesModeFilter:
		return 2
	default:
		return 0
	}
}
