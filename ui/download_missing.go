package ui

import (
	"errors"

	"grout/catalog"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	uatomic "go.uber.org/atomic"
)

// DownloadMissingForPlatform downloads every game of a platform that is not
// fully on the device, after asking. The download itself is the same screen a
// hand-picked selection goes through.
func DownloadMissingForPlatform(config settings.Config, host settings.Host, platform romm.Platform) {
	logger := gaba.GetLogger()

	games, ok := platformGames(platform)
	if !ok {
		return
	}

	missing := catalog.Missing(config, games)
	if len(missing) == 0 {
		gaba.ConfirmationMessage(
			localizeWith("download_missing_none", "All games for {{.Name}} are already downloaded.", map[string]any{"Name": platform.Name}),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	// X rather than A, so holding A through the list cannot start a download
	// of a whole platform.
	_, err := gaba.ConfirmationMessage(
		localizeWith("download_missing_confirm", "Download {{.Count}} missing games for {{.Name}}?",
			map[string]any{"Count": len(missing), "Name": platform.Name}),
		[]gaba.FooterHelpItem{FooterCancel(), footerItem("X", "button_download", "Download")},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	if err != nil {
		if !errors.Is(err, gaba.ErrCancelled) {
			logger.Error("Download missing confirmation failed", "error", err)
		}
		return
	}

	result := NewDownloadScreen().Execute(config, host, platform, missing, games, "", 0)

	gaba.ConfirmationMessage(
		localizeWith("download_missing_done", "Downloaded {{.Done}} of {{.Count}} games.",
			map[string]any{"Done": len(result.DownloadedGames), "Count": len(missing)}),
		ContinueFooter(),
		gaba.MessageOptions{},
	)
}

// platformGames returns a platform's games from the cache, fetching them when
// the cache has none. It has told the user when it returns false.
func platformGames(platform romm.Platform) ([]romm.Rom, bool) {
	source := catalog.GameSource{Platform: platform}
	if games, ok := catalog.CachedGames(source); ok {
		return games, true
	}

	progress := uatomic.NewFloat64(0)
	var games []romm.Rom
	_, err := gaba.ProcessMessage(
		localizeWith("games_list_loading", "Loading {{.Name}}...", map[string]any{"Name": platform.Name}),
		gaba.ProcessMessageOptions{ShowThemeBackground: true, ShowProgressBar: true, Progress: progress},
		func() (interface{}, error) {
			var err error
			games, err = catalog.RefreshGames(source, progress)
			return nil, err
		},
	)
	if err != nil {
		gaba.GetLogger().Error("Failed to load games to download", "platform", platform.Name, "error", err)
		gaba.ConfirmationMessage(
			localize("download_missing_load_error", "Failed to load games. Please try again later."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return nil, false
	}
	return games, true
}
