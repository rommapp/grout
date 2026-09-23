package ui

import (
	"fmt"
	"strings"
	"testing"

	"grout/cfw"
	"grout/library"
	"grout/settings"
)

// fullConfig sets every field this screen owns to something other than its
// zero value, so a setting that fails to round trip cannot hide behind a
// default that happens to match.
func fullConfig() settings.Config {
	config := settings.Config{
		ShowBoxArt:                   true,
		DownloadedGames:              settings.DownloadedGamesModeFilter,
		UnzipDownloads:               true,
		DownloadArt:                  true,
		ArtKind:                      library.ArtKindBox3D,
		DownloadArtScreenshotPreview: true,
		DownloadSplashArt:            library.ArtKindTitle,
		Language:                     "ja",
		SwapFaceButtons:              true,
		GamelistOmitsRegion:          true,
	}
	config.AdditionalDownloads.Thumbnail = library.ArtKindBox3D
	config.AdditionalDownloads.Marquee = library.ArtKindLogo
	config.AdditionalDownloads.Video = true
	config.AdditionalDownloads.Bezel = true
	config.AdditionalDownloads.Manual = true
	config.AdditionalDownloads.BoxBack = true
	config.AdditionalDownloads.Fanart = true
	return config
}

// settingsDiff names the fields this screen owns that differ, so a failure
// says which setting was lost rather than dumping two configs.
func settingsDiff(before, after settings.Config) string {
	var diffs []string
	compare := func(name string, a, b any) {
		if a != b {
			diffs = append(diffs, fmt.Sprintf("%s: %v -> %v", name, a, b))
		}
	}

	compare("ShowBoxArt", before.ShowBoxArt, after.ShowBoxArt)
	compare("DownloadedGames", before.DownloadedGames, after.DownloadedGames)
	compare("UnzipDownloads", before.UnzipDownloads, after.UnzipDownloads)
	compare("DownloadArt", before.DownloadArt, after.DownloadArt)
	compare("ArtKind", before.ArtKind, after.ArtKind)
	compare("DownloadArtScreenshotPreview", before.DownloadArtScreenshotPreview, after.DownloadArtScreenshotPreview)
	compare("DownloadSplashArt", before.DownloadSplashArt, after.DownloadSplashArt)
	compare("Thumbnail", before.AdditionalDownloads.Thumbnail, after.AdditionalDownloads.Thumbnail)
	compare("Marquee", before.AdditionalDownloads.Marquee, after.AdditionalDownloads.Marquee)
	compare("Video", before.AdditionalDownloads.Video, after.AdditionalDownloads.Video)
	compare("Bezel", before.AdditionalDownloads.Bezel, after.AdditionalDownloads.Bezel)
	compare("Manual", before.AdditionalDownloads.Manual, after.AdditionalDownloads.Manual)
	compare("BoxBack", before.AdditionalDownloads.BoxBack, after.AdditionalDownloads.BoxBack)
	compare("Fanart", before.AdditionalDownloads.Fanart, after.AdditionalDownloads.Fanart)
	compare("Language", before.Language, after.Language)
	compare("SwapFaceButtons", before.SwapFaceButtons, after.SwapFaceButtons)
	compare("GamelistOmitsRegion", before.GamelistOmitsRegion, after.GamelistOmitsRegion)

	return strings.Join(diffs, "; ")
}

// Opening the settings screen and saving without touching anything must leave
// every setting exactly as it was.
//
// A row whose opening selection is computed wrongly shows the wrong value and
// then writes that back, so simply visiting the screen loses the choice. That
// is what one index helper shared across three different option lists did to
// Download Splash Art: Marquee and Title both fell through to the list's first
// entry, None, and saving made it so.
func TestGeneralSettings_RoundTripsEverySetting(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	before := fullConfig()
	after := before

	rows := generalSettings(before)
	applySettingRows(rows, &after, settingItems(rows, before))

	if diff := settingsDiff(before, after); diff != "" {
		t.Errorf("opening and saving the screen changed settings: %s", diff)
	}
}

// The EmulationStation art rows are the ones a muOS device never shows, so
// they need their own pass on a firmware that does.
func TestGeneralSettings_RoundTripsOnEmulationStation(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.Knulli))

	before := fullConfig()
	after := before

	rows := generalSettings(before)
	applySettingRows(rows, &after, settingItems(rows, before))

	if diff := settingsDiff(before, after); diff != "" {
		t.Errorf("opening and saving the screen changed settings: %s", diff)
	}
}

// A config with a value missing gets the same default the settings package
// would give it, rather than whatever the option list happens to list first.
// The two are written down separately, so they can drift.
func TestGeneralSettings_BlanksTakeTheSettingsDefaults(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	var config settings.Config
	rows := generalSettings(config)
	applySettingRows(rows, &config, settingItems(rows, config))

	if config.DownloadedGames != settings.DownloadedGamesModeDoNothing {
		t.Errorf("DownloadedGames = %q, want the package default", config.DownloadedGames)
	}
	if config.ArtKind != library.ArtKindDefault {
		t.Errorf("ArtKind = %q, want the package default", config.ArtKind)
	}
	if config.Language != "en" {
		t.Errorf("Language = %q, want the package default", config.Language)
	}
}

// Rows are read back by key. Two rows sharing one would make the second
// unreachable, and every row needs one or it is silently never saved.
func TestGeneralSettings_KeysAreUniqueAndPresent(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	seen := make(map[string]bool)
	for _, row := range generalSettings(fullConfig()) {
		if row.key == "" {
			t.Errorf("row %q has no key", row.label)
			continue
		}
		if seen[row.key] {
			t.Errorf("two rows share the key %q", row.key)
		}
		seen[row.key] = true
	}
}

// Every row must be able to show the value the config holds. An option list
// that cannot represent it falls back to its first entry, which is how a
// setting gets quietly rewritten.
func TestGeneralSettings_OptionsCoverTheStoredValue(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	config := fullConfig()
	for _, row := range generalSettings(config) {
		want := row.get(config)

		found := false
		for _, option := range row.options {
			if option.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: no option carries the stored value %v", row.key, want)
		}
	}
}

// A row that names a default has to name the one the settings package
// applies, or a config the defaults have not reached opens on a different
// value than the rest of the app is using.
func TestGeneralSettings_DefaultsMatchTheSettingsPackage(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))
	fresh := freshConfig(t)

	want := map[string]any{
		"downloaded_games": fresh.DownloadedGames,
		"art_kind":         fresh.ArtKind,
		"thumbnail":        fresh.AdditionalDownloads.Thumbnail,
		"marquee":          fresh.AdditionalDownloads.Marquee,
		"language":         fresh.Language,
	}

	for _, row := range generalSettings(*fresh) {
		expected, checked := want[row.key]
		if !checked {
			continue
		}
		if row.def != expected {
			t.Errorf("%q defaults to %v, settings defaults to %v", row.key, row.def, expected)
		}
	}
}

// Naming a default that none of a row's own options offers would fall through
// to the first entry, which is the thing the default is there to avoid.
func TestGeneralSettings_DefaultsAreOfferable(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	for _, row := range generalSettings(fullConfig()) {
		if row.def == nil {
			continue
		}

		found := false
		for _, option := range row.options {
			if option.Value == row.def {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q defaults to %v, which none of its options offers", row.key, row.def)
		}
	}
}
