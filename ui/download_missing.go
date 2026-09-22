package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
)

// DownloadMissingForPlatform downloads every not-yet-downloaded game of a platform.
// It confirms with the user first, then hands off to the shared download pipeline
// (progress modal, cancel, artwork/unzip/gamelist handling).
func DownloadMissingForPlatform(config internal.Config, host romm.Host, platform romm.Platform) {
	logger := gaba.GetLogger()

	games := loadPlatformGamesForDownload(platform)
	if games == nil {
		return
	}

	missing := missingGames(games, config)
	if len(missing) == 0 {
		gaba.ConfirmationMessage(
			fmt.Sprintf(
				i18n.Localize(&goi18n.Message{ID: "download_missing_none", Other: "All games for %s are already downloaded."}, nil),
				platform.Name,
			),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	_, err := gaba.ConfirmationMessage(
		fmt.Sprintf(
			i18n.Localize(&goi18n.Message{ID: "download_missing_confirm", Other: "Download %d missing game(s) for %s?"}, nil),
			len(missing), platform.Name,
		),
		[]gaba.FooterHelpItem{
			FooterCancel(),
			{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_confirm", Other: "Confirm"}, nil)},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return
		}
		logger.Error("Download missing confirmation failed", "error", err)
		return
	}

	result := NewDownloadScreen().Execute(config, host, platform, missing, games, "", 0)

	gaba.ConfirmationMessage(
		fmt.Sprintf(
			i18n.Localize(&goi18n.Message{ID: "download_missing_done", Other: "Downloaded %d of %d game(s)."}, nil),
			len(result.DownloadedGames), len(missing),
		),
		ContinueFooter(),
		gaba.MessageOptions{},
	)
}

// loadPlatformGamesForDownload returns the platform's games from cache, refreshing
// from the server if the cache is empty. Returns nil when nothing could be loaded.
func loadPlatformGamesForDownload(platform romm.Platform) []romm.Rom {
	logger := gaba.GetLogger()

	cm := cache.GetCacheManager()
	if cm == nil {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "download_missing_load_error", Other: "Failed to load games. Please try again later."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return nil
	}

	if games, err := cm.GetPlatformGames(platform.ID); err == nil && len(games) > 0 {
		return games
	}

	progress := atomic.NewFloat64(0)
	var games []romm.Rom
	_, err := gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."}, map[string]interface{}{"Name": platform.Name}),
		gaba.ProcessMessageOptions{ShowThemeBackground: true, ShowProgressBar: true, Progress: progress},
		func() (interface{}, error) {
			if err := cm.RefreshPlatformGamesWithProgress(platform, progress); err != nil {
				return nil, err
			}
			loaded, err := cm.GetPlatformGames(platform.ID)
			if err != nil {
				return nil, err
			}
			games = loaded
			return nil, nil
		},
	)
	if err != nil {
		logger.Error("Failed to refresh platform games for download", "platform", platform.Name, "error", err)
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "download_missing_load_error", Other: "Failed to load games. Please try again later."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return nil
	}

	return games
}

// missingGames keeps only the games that are not present on disk.
func missingGames(games []romm.Rom, resolver romm.PlatformDirResolver) []romm.Rom {
	missing := make([]romm.Rom, 0)
	for i := range games {
		if !games[i].IsDownloaded(resolver) {
			missing = append(missing, games[i])
		}
	}
	return missing
}
