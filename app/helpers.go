package main

import (
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"grout/ui"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/router"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	uatomic "go.uber.org/atomic"
)

func filterGamesByPlatform(games []romm.Rom, platformID int) []romm.Rom {
	filtered := make([]romm.Rom, 0)
	for _, game := range games {
		if game.PlatformID == platformID {
			filtered = append(filtered, game)
		}
	}
	return filtered
}

func savePlatformOrder(state *AppState, platforms []romm.Platform) {
	var platformOrder []string
	for _, p := range platforms {
		platformOrder = append(platformOrder, p.Slug)
	}
	state.Config.PlatformOrder = platformOrder
	state.Platforms = platforms
	internal.SaveConfig(state.Config)
}

func triggerAutoSyncRouter(state *AppState) {
	if state.AutoSync != nil {
		state.AutoSync.Trigger()
	}
}

func executeDownloadUI(state *AppState, r ui.GameDetailsOutput, stack *router.Stack) {
	entry := stack.Peek()
	var allGames []romm.Rom
	var searchFilter string
	if entry != nil {
		if input, ok := entry.Input.(ui.GameListInput); ok {
			allGames = input.Games
			searchFilter = input.SearchFilter
		}
	}

	downloadScreen := ui.NewDownloadScreen()
	downloadScreen.Execute(*state.Config, state.Host, r.Platform, []romm.Rom{r.Game}, allGames, searchFilter, r.SelectedFileID)
}

func executeMultiDownloadUI(state *AppState, r ui.GameListOutput) {
	downloadScreen := ui.NewDownloadScreen()
	downloadScreen.Execute(*state.Config, state.Host, r.Platform, r.SelectedGames, r.AllGames, r.SearchFilter, 0)
}

func handlePlatformMappingUpdateUI(state *AppState, r ui.PlatformMappingOutput) {
	state.Config.DirectoryMappings = r.Mappings
	state.Config.PlatformOrder = internal.PrunePlatformOrder(state.Config.PlatformOrder, r.Mappings)
	internal.SaveConfig(state.Config)

	platforms, err := internal.GetMappedPlatforms(state.Host, r.Mappings, state.Config.ApiTimeout)
	if err != nil {
		gaba.GetLogger().Error("Failed to refresh platforms after mapping update", "error", err)
		return
	}

	oldIDs := make(map[int]bool)
	for _, p := range state.Platforms {
		oldIDs[p.ID] = true
	}

	var newPlatforms []romm.Platform
	for _, p := range platforms {
		if !oldIDs[p.ID] {
			newPlatforms = append(newPlatforms, p)
		}
	}

	state.Platforms = platforms

	if len(newPlatforms) > 0 && state.CacheSync != nil {
		state.CacheSync.SyncPlatforms(newPlatforms)
	}
}

func handleLogout(state *AppState) {
	logger := gaba.GetLogger()

	if err := cache.DeleteCacheFolder(); err != nil {
		logger.Error("Failed to delete cache folder", "error", err)
	}

	state.Config.Hosts = nil
	state.Config.DirectoryMappings = nil
	state.Config.PlatformOrder = nil

	if err := internal.SaveConfig(state.Config); err != nil {
		logger.Error("Failed to save config after logout", "error", err)
		return
	}

	logger.Info("User logged out successfully")

	loginConfig, err := ui.LoginFlow(romm.Host{})
	if err != nil {
		logger.Error("Login flow failed after logout", "error", err)
		return
	}

	state.Config.Hosts = loginConfig.Hosts
	if err := internal.SaveConfig(state.Config); err != nil {
		logger.Error("Failed to save config after re-login", "error", err)
		return
	}

	state.Host = state.Config.Hosts[0]

	if len(state.Config.DirectoryMappings) == 0 {
		screen := ui.NewPlatformMappingScreen()
		result, err := screen.Draw(ui.PlatformMappingInput{
			Host:             state.Config.Hosts[0],
			ApiTimeout:       state.Config.ApiTimeout,
			CFW:              state.CFW,
			RomDirectory:     cfw.GetRomDirectory(),
			AutoSelect:       false,
			HideBackButton:   true,
			PlatformsBinding: state.Config.PlatformsBinding,
		})

		if err == nil && result.Action == ui.PlatformMappingActionSaved {
			state.Config.DirectoryMappings = result.Mappings
			internal.SaveConfig(state.Config)
		}
	}

	if err := cache.InitCacheManager(state.Config.Hosts[0], state.Config); err != nil {
		logger.Error("Failed to initialize cache manager after re-login", "error", err)
	}

	platforms, err := internal.GetMappedPlatforms(state.Config.Hosts[0], state.Config.DirectoryMappings, state.Config.ApiTimeout)
	if err != nil {
		logger.Error("Failed to load platforms after re-login", "error", err)
		return
	}
	state.Platforms = platforms

	if cm := cache.GetCacheManager(); cm != nil && cm.IsFirstRun() {
		progress := uatomic.NewFloat64(0)
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "cache_building", Other: "Building cache..."}, nil),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (interface{}, error) {
				_, err := cm.PopulateFullCacheWithProgress(platforms, progress)
				return nil, err
			},
		)
	}
}
