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

// setting is one row of the screen: what it is called, what it offers, where
// its value lives, and when it is worth showing.
type setting struct {
	// key identifies the row when the screen is read back. Matching on the
	// label instead would break the day two settings read alike in some
	// language, silently and with nothing to catch it.
	key     string
	label   string
	options []gaba.Option
	value   func(settings.Config) any
	apply   func(*settings.Config, any)
	// visible, when set, hides the row while it holds no meaning.
	visible *atomic.Bool
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
		menuItems(rows, *config),
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("General settings error", "error", err)
		return output, err
	}

	applySettings(rows, config, result.Items)

	err = settings.SaveConfig(config)
	ApplyRuntimeSettings(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving general settings", "error", err)
		return output, err
	}

	output.Action = GeneralSettingsActionSaved
	return output, nil
}

// menuItems renders the rows, each opened on the value the config holds.
func menuItems(rows []setting, config settings.Config) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(rows))
	for _, row := range rows {
		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: row.label, Metadata: row.key},
			Options:        row.options,
			SelectedOption: optionIndex(row.options, row.value(config)),
			VisibleWhen:    row.visible,
		})
	}
	return items
}

// applySettings writes back what the user chose. Rows are found by key, so a
// row the screen did not show keeps whatever the config already had.
func applySettings(rows []setting, config *settings.Config, items []gaba.ItemWithOptions) {
	byKey := make(map[string]setting, len(rows))
	for _, row := range rows {
		byKey[row.key] = row
	}

	for _, item := range items {
		key, _ := item.Item.Metadata.(string)
		row, ok := byKey[key]
		if !ok {
			continue
		}
		row.apply(config, item.Options[item.SelectedOption].Value)
	}
}

// assign stores a value of the type a row deals in, ignoring anything else, so
// a mistyped option cannot write nonsense into the config.
func assign[T any](set func(*settings.Config, T)) func(*settings.Config, any) {
	return func(config *settings.Config, value any) {
		if typed, ok := value.(T); ok {
			set(config, typed)
		}
	}
}

// trueFalse is the option pair most of this screen uses.
func trueFalse() []gaba.Option {
	return []gaba.Option{
		{DisplayName: localize("common_true", "True"), Value: true},
		{DisplayName: localize("common_false", "False"), Value: false},
	}
}

func artKindOption(id, fallback string, kind library.ArtKind) gaba.Option {
	return gaba.Option{DisplayName: localize(id, fallback), Value: kind}
}

func generalSettings(config settings.Config) []setting {
	activeCFW := cfw.GetCFW()

	// Art options only mean something while art is being downloaded at all,
	// and each firmware family keeps a different set of it.
	downloadingArt := &atomic.Bool{}
	previewArt := &atomic.Bool{}
	emulationStationArt := &atomic.Bool{}

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

	return []setting{
		{
			key: "box_art", label: localize("settings_box_art", "Box Art"),
			options: []gaba.Option{
				{DisplayName: localize("common_show", "Show"), Value: true},
				{DisplayName: localize("common_hide", "Hide"), Value: false},
			},
			value: func(c settings.Config) any { return c.ShowBoxArt },
			apply: assign(func(c *settings.Config, v bool) { c.ShowBoxArt = v }),
		},
		{
			key: "downloaded_games", label: localize("settings_downloaded_games", "Downloaded Games"),
			options: []gaba.Option{
				{DisplayName: localize("downloaded_games_do_nothing", "Do Nothing"), Value: settings.DownloadedGamesModeDoNothing},
				{DisplayName: localize("downloaded_games_mark", "Mark"), Value: settings.DownloadedGamesModeMark},
				{DisplayName: localize("downloaded_games_filter", "Filter"), Value: settings.DownloadedGamesModeFilter},
			},
			value: func(c settings.Config) any { return c.DownloadedGames },
			apply: assign(func(c *settings.Config, v settings.DownloadedGamesMode) { c.DownloadedGames = v }),
		},
		{
			key: "archived_downloads", label: localize("settings_compressed_downloads", "Archived Downloads"),
			options: []gaba.Option{
				{DisplayName: localize("settings_compressed_downloads_uncompress", "Uncompress"), Value: true},
				{DisplayName: localize("settings_compressed_downloads_do_nothing", "Do Nothing"), Value: false},
			},
			value: func(c settings.Config) any { return c.UnzipDownloads },
			apply: assign(func(c *settings.Config, v bool) { c.UnzipDownloads = v }),
		},
		{
			key: "download_art", label: localize("settings_download_art", "Download Art"),
			options: downloadArtOptions,
			value:   func(c settings.Config) any { return c.DownloadArt },
			apply:   assign(func(c *settings.Config, v bool) { c.DownloadArt = v }),
		},
		{
			key: "art_kind", label: localize("settings_download_art_kind", "Download Art Kind"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_default", "Default", library.ArtKindDefault),
				artKindOption("settings_download_art_kind_box2d", "Box2D", library.ArtKindBox2D),
				artKindOption("settings_download_art_kind_box3d", "Box3D", library.ArtKindBox3D),
				artKindOption("settings_download_art_kind_miximage", "MixImage", library.ArtKindMixImage),
			},
			value:   func(c settings.Config) any { return c.ArtKind },
			apply:   assign(func(c *settings.Config, v library.ArtKind) { c.ArtKind = v }),
			visible: downloadingArt,
		},
		{
			key: "screenshot_preview", label: localize("settings_download_art_preview", "Download Screenshot Preview"),
			options: trueFalse(),
			value:   func(c settings.Config) any { return c.DownloadArtScreenshotPreview },
			apply:   assign(func(c *settings.Config, v bool) { c.DownloadArtScreenshotPreview = v }),
			visible: previewArt,
		},
		{
			key: "splash_art", label: localize("settings_download_art_splash", "Download Splash Art"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_none", "None", library.ArtKindNone),
				artKindOption("settings_download_art_kind_marquee", "Marquee", library.ArtKindMarquee),
				artKindOption("settings_download_art_kind_title", "Title", library.ArtKindTitle),
			},
			value:   func(c settings.Config) any { return c.DownloadSplashArt },
			apply:   assign(func(c *settings.Config, v library.ArtKind) { c.DownloadSplashArt = v }),
			visible: previewArt,
		},
		{
			key: "thumbnail", label: localize("settings_download_emulationstation_art_thumbnail", "Download Game Thumbnail"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_none", "None", library.ArtKindNone),
				artKindOption("settings_download_art_kind_box2d", "Box2D", library.ArtKindBox2D),
				artKindOption("settings_download_art_kind_box3d", "Box3D", library.ArtKindBox3D),
			},
			value:   func(c settings.Config) any { return c.AdditionalDownloads.Thumbnail },
			apply:   assign(func(c *settings.Config, v library.ArtKind) { c.AdditionalDownloads.Thumbnail = v }),
			visible: emulationStationArt,
		},
		{
			key: "marquee", label: localize("settings_download_emulationstation_art_marquee", "Download Marquee Image"),
			options: []gaba.Option{
				artKindOption("settings_download_art_kind_none", "None", library.ArtKindNone),
				artKindOption("settings_download_art_kind_marquee", "Marquee", library.ArtKindMarquee),
				artKindOption("settings_download_art_kind_logo", "Logo", library.ArtKindLogo),
			},
			value:   func(c settings.Config) any { return c.AdditionalDownloads.Marquee },
			apply:   assign(func(c *settings.Config, v library.ArtKind) { c.AdditionalDownloads.Marquee = v }),
			visible: emulationStationArt,
		},
		{
			key: "video", label: localize("settings_download_emulationstation_art_video", "Download Game Video"),
			options: trueFalse(),
			value:   func(c settings.Config) any { return c.AdditionalDownloads.Video },
			apply:   assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Video = v }),
			visible: emulationStationArt,
		},
		{
			key: "bezel", label: localize("settings_download_emulationstation_art_bezel", "Download Game Bezel"),
			options: trueFalse(),
			value:   func(c settings.Config) any { return c.AdditionalDownloads.Bezel },
			apply:   assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Bezel = v }),
			visible: emulationStationArt,
		},
		{
			key: "manual", label: localize("settings_download_emulationstation_art_manual", "Download Game Manual"),
			options: trueFalse(),
			value:   func(c settings.Config) any { return c.AdditionalDownloads.Manual },
			apply:   assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Manual = v }),
			visible: emulationStationArt,
		},
		{
			key: "boxback", label: localize("settings_download_emulationstation_art_boxback", "Download Game Box back"),
			options: trueFalse(),
			value:   func(c settings.Config) any { return c.AdditionalDownloads.BoxBack },
			apply:   assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.BoxBack = v }),
			visible: emulationStationArt,
		},
		{
			key: "fanart", label: localize("settings_download_emulationstation_art_fanart", "Download Game Fan Art"),
			options: trueFalse(),
			value:   func(c settings.Config) any { return c.AdditionalDownloads.Fanart },
			apply:   assign(func(c *settings.Config, v bool) { c.AdditionalDownloads.Fanart = v }),
			visible: emulationStationArt,
		},
		{
			key: "language", label: localize("settings_language", "Language"),
			options: languageOptions(),
			value:   func(c settings.Config) any { return c.Language },
			apply:   assign(func(c *settings.Config, v string) { c.Language = v }),
		},
		{
			key: "swap_face_buttons", label: localize("settings_swap_face_buttons", "Swap Face Buttons"),
			options: []gaba.Option{
				{DisplayName: localize("common_false", "False"), Value: false},
				{DisplayName: localize("common_true", "True"), Value: true},
			},
			value: func(c settings.Config) any { return c.SwapFaceButtons },
			apply: assign(func(c *settings.Config, v bool) {
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
