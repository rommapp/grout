package ui

import (
	"errors"
	"sync/atomic"

	"grout/cfw"
	"grout/library"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type GeneralSettingsInput struct {
	Config *settings.Config
}

type GeneralSettingsOutput struct {
	Action GeneralSettingsAction
	Config *settings.Config
}

type GeneralSettingsScreen struct{}

func NewGeneralSettingsScreen() *GeneralSettingsScreen {
	return &GeneralSettingsScreen{}
}

func (s *GeneralSettingsScreen) Draw(input GeneralSettingsInput) (GeneralSettingsOutput, error) {
	config := input.Config
	output := GeneralSettingsOutput{Action: GeneralSettingsActionBack, Config: config}

	rows := generalSettings(*config)

	result, err := gaba.OptionsList(
		localize("settings_general", "General"),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		settingItems(rows, *config),
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("General settings error", "error", err)
		return output, err
	}

	applySettingRows(rows, config, result.Items)

	err = settings.SaveConfig(config)
	ApplyRuntimeSettings(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving general settings", "error", err)
		return output, err
	}

	output.Action = GeneralSettingsActionSaved
	return output, nil
}

func artKindOption(id, fallback string, kind library.ArtKind) gaba.Option {
	return gaba.Option{DisplayName: localize(id, fallback), Value: kind}
}

func generalSettings(config settings.Config) []settingRow {
	activeCFW := cfw.GetCFW()

	// Art options only mean something while art is being downloaded at all,
	// and each firmware family keeps a different set of it.
	downloadingArt := &atomic.Bool{}
	previewArt := &atomic.Bool{}
	emulationStationArt := &atomic.Bool{}

	// Only EmulationStation reads a gamelist, whether or not art is wanted.
	emulationStation := &atomic.Bool{}
	emulationStation.Store(activeCFW.IsBasedOnEmulationStation())

	refresh := func(wanted bool) {
		downloadingArt.Store(wanted)
		previewArt.Store(wanted && activeCFW == cfw.MuOS)
		emulationStationArt.Store(wanted && activeCFW.IsBasedOnEmulationStation())
	}
	refresh(config.DownloadArt)

	downloadArtOptions := trueFalse()
	for i := range downloadArtOptions {
		wanted := downloadArtOptions[i].Value.(bool)
		downloadArtOptions[i].OnUpdate = func(any) { refresh(wanted) }
	}

	return []settingRow{
		{
			key: "box_art", label: localize("settings_box_art", "Box Art"),
			options: showHide(),
			get:     func(c settings.Config) any { return c.ShowBoxArt },
			set:     assign(func(c *settings.Config, v bool) { c.ShowBoxArt = v }),
		},
		{
			key: "downloaded_games", label: localize("settings_downloaded_games", "Downloaded Games"),
			options: []gaba.Option{
				{DisplayName: localize("downloaded_games_do_nothing", "Do Nothing"), Value: settings.DownloadedGamesModeDoNothing},
				{DisplayName: localize("downloaded_games_mark", "Mark"), Value: settings.DownloadedGamesModeMark},
				{DisplayName: localize("downloaded_games_filter", "Filter"), Value: settings.DownloadedGamesModeFilter},
			},
			get: func(c settings.Config) any { return c.DownloadedGames },
			set: assign(func(c *settings.Config, v settings.DownloadedGamesMode) { c.DownloadedGames = v }),
			def: settings.DownloadedGamesModeDoNothing,
		},
		{
			key: "archived_downloads", label: localize("settings_compressed_downloads", "Archived Downloads"),
			options: []gaba.Option{
				{DisplayName: localize("settings_compressed_downloads_uncompress", "Uncompress"), Value: true},
				{DisplayName: localize("settings_compressed_downloads_do_nothing", "Do Nothing"), Value: false},
			},
			get: func(c settings.Config) any { return c.UnzipDownloads },
			set: assign(func(c *settings.Config, v bool) { c.UnzipDownloads = v }),
		},
		{
			key: "download_art", label: localize("settings_download_art", "Download Art"),
			options: downloadArtOptions,
			get:     func(c settings.Config) any { return c.DownloadArt },
			set:     assign(func(c *settings.Config, v bool) { c.DownloadArt = v }),
		},
		{
			key: "art_kind", label: localize("settings_download_art_kind", "Download Art Kind"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_default", "Default", library.ArtKindDefault),
				artKindOption("settings_download_art_kind_box2d", "Box2D", library.ArtKindBox2D),
				artKindOption("settings_download_art_kind_box3d", "Box3D", library.ArtKindBox3D),
				artKindOption("settings_download_art_kind_miximage", "MixImage", library.ArtKindMixImage),
			},
			get:     func(c settings.Config) any { return c.ArtKind },
			set:     assign(func(c *settings.Config, v library.ArtKind) { c.ArtKind = v }),
			visible: downloadingArt,
			def:     library.ArtKindDefault,
		},
		{
			key: "screenshot_preview", label: localize("settings_download_art_preview", "Download Screenshot Preview"),
			options: trueFalse(),
			get:     func(c settings.Config) any { return c.DownloadArtScreenshotPreview },
			set:     assign(func(c *settings.Config, v bool) { c.DownloadArtScreenshotPreview = v }),
			visible: previewArt,
		},
		{
			key: "splash_art", label: localize("settings_download_art_splash", "Download Splash Art"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_none", "None", library.ArtKindNone),
				artKindOption("settings_download_art_kind_marquee", "Marquee", library.ArtKindMarquee),
				artKindOption("settings_download_art_kind_title", "Title", library.ArtKindTitle),
			},
			get:     func(c settings.Config) any { return c.DownloadSplashArt },
			set:     assign(func(c *settings.Config, v library.ArtKind) { c.DownloadSplashArt = v }),
			visible: previewArt,
		},
		{
			key: "thumbnail", label: localize("settings_download_emulationstation_art_thumbnail", "Download Game Thumbnail"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_none", "None", library.ArtKindNone),
				artKindOption("settings_download_art_kind_box2d", "Box2D", library.ArtKindBox2D),
				artKindOption("settings_download_art_kind_box3d", "Box3D", library.ArtKindBox3D),
			},
			get:     func(c settings.Config) any { return c.AdditionalDownloads.Thumbnail },
			set:     assign(func(c *settings.Config, v library.ArtKind) { c.AdditionalDownloads.Thumbnail = v }),
			visible: emulationStationArt,
			def:     library.ArtKindNone,
		},
		{
			key: "marquee", label: localize("settings_download_emulationstation_art_marquee", "Download Marquee Image"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_none", "None", library.ArtKindNone),
				artKindOption("settings_download_art_kind_marquee", "Marquee", library.ArtKindMarquee),
				artKindOption("settings_download_art_kind_logo", "Logo", library.ArtKindLogo),
			},
			get:     func(c settings.Config) any { return c.AdditionalDownloads.Marquee },
			set:     assign(func(c *settings.Config, v library.ArtKind) { c.AdditionalDownloads.Marquee = v }),
			visible: emulationStationArt,
			def:     library.ArtKindNone,
		},
		{
			key: "video", label: localize("settings_download_emulationstation_art_video", "Download Game Video"),
			options: trueFalse(),
			get:     func(c settings.Config) any { return c.AdditionalDownloads.Video },
			set:     assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Video = v }),
			visible: emulationStationArt,
		},
		{
			key: "bezel", label: localize("settings_download_emulationstation_art_bezel", "Download Game Bezel"),
			options: trueFalse(),
			get:     func(c settings.Config) any { return c.AdditionalDownloads.Bezel },
			set:     assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Bezel = v }),
			visible: emulationStationArt,
		},
		{
			key: "manual", label: localize("settings_download_emulationstation_art_manual", "Download Game Manual"),
			options: trueFalse(),
			get:     func(c settings.Config) any { return c.AdditionalDownloads.Manual },
			set:     assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Manual = v }),
			visible: emulationStationArt,
		},
		{
			key: "boxback", label: localize("settings_download_emulationstation_art_boxback", "Download Game Box back"),
			options: trueFalse(),
			get:     func(c settings.Config) any { return c.AdditionalDownloads.BoxBack },
			set:     assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.BoxBack = v }),
			visible: emulationStationArt,
		},
		{
			key: "fanart", label: localize("settings_download_emulationstation_art_fanart", "Download Game Fan Art"),
			options: trueFalse(),
			get:     func(c settings.Config) any { return c.AdditionalDownloads.Fanart },
			set:     assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Fanart = v }),
			visible: emulationStationArt,
		},
		{
			key: "gamelist_region", label: localize("settings_gamelist_region", "Region in Gamelist Names"),
			options: []gaba.Option{
				{DisplayName: localize("settings_gamelist_region_include", "Include"), Value: false},
				{DisplayName: localize("settings_gamelist_region_omit", "Omit"), Value: true},
			},
			get:     func(c settings.Config) any { return c.GamelistOmitsRegion },
			set:     assign(func(c *settings.Config, v bool) { c.GamelistOmitsRegion = v }),
			visible: emulationStation,
		},
		{
			key: "language", label: localize("settings_language", "Language"),
			options: languageOptions(),
			get:     func(c settings.Config) any { return c.Language },
			set:     assign(func(c *settings.Config, v string) { c.Language = v }),
			def:     "en",
		},
		{
			key: "swap_face_buttons", label: localize("settings_swap_face_buttons", "Swap Face Buttons"),
			options: []gaba.Option{
				{DisplayName: localize("common_false", "False"), Value: false},
				{DisplayName: localize("common_true", "True"), Value: true},
			},
			get: func(c settings.Config) any { return c.SwapFaceButtons },
			set: assign(func(c *settings.Config, v bool) {
				c.SwapFaceButtons = v
				gaba.SetFlipFaceButtons(v)
			}),
		},
	}
}

func languageOptions() []gaba.Option {
	languages := []struct{ id, name, code string }{
		{"settings_language_english", "English", "en"},
		{"settings_language_german", "Deutsch", "de"},
		{"settings_language_spanish", "Español", "es"},
		{"settings_language_french", "Français", "fr"},
		{"settings_language_italian", "Italiano", "it"},
		{"settings_language_portuguese", "Português", "pt"},
		{"settings_language_russian", "Русский", "ru"},
		{"settings_language_japanese", "日本語", "ja"},
	}

	options := make([]gaba.Option, 0, len(languages))
	for _, language := range languages {
		options = append(options, gaba.Option{DisplayName: localize(language.id, language.name), Value: language.code})
	}
	return options
}
