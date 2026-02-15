package main

import (
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/romm"
	"grout/ui"
	"grout/update"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/router"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	uatomic "go.uber.org/atomic"
)

func runWithRouter(config *internal.Config, currentCFW cfw.CFW, platforms []romm.Platform, quitOnBack bool, showCollections bool) error {
	state := &AppState{
		Config:    config,
		Host:      config.Hosts[0],
		CFW:       currentCFW,
		Platforms: platforms,
	}
	currentAppState = state

	go func() {
		client := romm.NewClientFromHost(state.Host)
		if heartbeat, err := client.GetHeartbeat(); err == nil {
			state.RommVersion.Store(heartbeat.System.Version)
		}
	}()

	r := buildRouter(state, quitOnBack, showCollections)

	initialInput := ui.PlatformSelectionInput{
		Platforms:       &state.Platforms,
		QuitOnBack:      quitOnBack,
		ShowCollections: showCollections,
	}

	return r.Run(ScreenPlatformSelection, initialInput)
}

func buildRouter(state *AppState, quitOnBack bool, showCollections bool) *router.Router {
	r := router.New()

	state.CacheSync = cache.NewBackgroundSync(state.Platforms)
	ui.AddStatusBarIcon(state.CacheSync.Icon())

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
				_, err := cm.PopulateFullCacheWithProgress(state.Platforms, progress)
				return nil, err
			},
		)
		state.CacheSync.SetSynced()
	} else {
		state.CacheSync.Start()
	}

	cache.RunArtworkValidation()

	registerScreens(r, state)
	r.OnTransition(buildTransitionFunc(state, quitOnBack, showCollections))

	return r
}

func registerScreens(r *router.Router, state *AppState) {
	r.Register(ScreenPlatformSelection, func(input any) (any, error) {
		in := input.(ui.PlatformSelectionInput)

		state.autoUpdateOnce.Do(func() {
			state.AutoUpdate = update.NewAutoUpdate(state.CFW, state.Config.ReleaseChannel, &state.Host)
			ui.AddStatusBarIcon(state.AutoUpdate.Icon())
			state.AutoUpdate.Start()
		})

		screen := ui.NewPlatformSelectionScreen()
		return screen.Draw(in)
	})

	r.Register(ScreenGameList, func(input any) (any, error) {
		screen := ui.NewGameListScreen()
		return screen.Draw(input.(ui.GameListInput))
	})

	r.Register(ScreenGameDetails, func(input any) (any, error) {
		screen := ui.NewGameDetailsScreen()
		return screen.Draw(input.(ui.GameDetailsInput))
	})

	r.Register(ScreenGameOptions, func(input any) (any, error) {
		screen := ui.NewGameOptionsScreen()
		return screen.Draw(input.(ui.GameOptionsInput))
	})

	r.Register(ScreenGameQR, func(input any) (any, error) {
		screen := ui.NewGameQRScreen()
		return screen.Draw(input.(ui.GameQRInput))
	})

	r.Register(ScreenSearch, func(input any) (any, error) {
		screen := ui.NewSearchScreen()
		return screen.Draw(input.(ui.SearchInput))
	})

	r.Register(ScreenCollectionList, func(input any) (any, error) {
		screen := ui.NewCollectionSelectionScreen()
		return screen.Draw(input.(ui.CollectionSelectionInput))
	})

	r.Register(ScreenCollectionPlatformSelection, func(input any) (any, error) {
		screen := ui.NewCollectionPlatformSelectionScreen()
		return screen.Draw(input.(ui.CollectionPlatformSelectionInput))
	})

	r.Register(ScreenSettings, func(input any) (any, error) {
		screen := ui.NewSettingsScreen()
		return screen.Draw(input.(ui.SettingsInput))
	})

	r.Register(ScreenGeneralSettings, func(input any) (any, error) {
		screen := ui.NewGeneralSettingsScreen()
		return screen.Draw(input.(ui.GeneralSettingsInput))
	})

	r.Register(ScreenCollectionsSettings, func(input any) (any, error) {
		screen := ui.NewCollectionsSettingsScreen()
		return screen.Draw(input.(ui.CollectionsSettingsInput))
	})

	r.Register(ScreenAdvancedSettings, func(input any) (any, error) {
		screen := ui.NewAdvancedSettingsScreen()
		return screen.Draw(input.(ui.AdvancedSettingsInput))
	})

	r.Register(ScreenPlatformMapping, func(input any) (any, error) {
		screen := ui.NewPlatformMappingScreen()
		return screen.Draw(input.(ui.PlatformMappingInput))
	})

	r.Register(ScreenInfo, func(input any) (any, error) {
		screen := ui.NewInfoScreen()
		return screen.Draw(input.(ui.InfoInput))
	})

	r.Register(ScreenLogoutConfirmation, func(input any) (any, error) {
		screen := ui.NewLogoutConfirmationScreen()
		return screen.Draw()
	})

	r.Register(ScreenRebuildCache, func(input any) (any, error) {
		in := input.(ui.RebuildCacheInput)
		in.CacheSync = state.CacheSync
		screen := ui.NewRebuildCacheScreen()
		return screen.Draw(in)
	})

	r.Register(ScreenBIOSDownload, func(input any) (any, error) {
		in := input.(ui.BIOSDownloadInput)
		screen := ui.NewBIOSDownloadScreen()
		screen.Execute(in.Config, in.Host, in.Platform)
		return ui.BIOSDownloadOutput{Platform: in.Platform}, nil
	})

	r.Register(ScreenArtworkSync, func(input any) (any, error) {
		in := input.(ui.ArtworkSyncInput)
		screen := ui.NewArtworkSyncScreen()
		screen.Execute(in.Config, in.Host)
		return ui.ArtworkSyncOutput{}, nil
	})

	r.Register(ScreenUpdateCheck, func(input any) (any, error) {
		screen := ui.NewUpdateScreen()
		return screen.Draw(input.(ui.UpdateInput))
	})

	r.Register(ScreenGameFilters, func(input any) (any, error) {
		screen := ui.NewGameFiltersScreen()
		return screen.Draw(input.(ui.GameFiltersInput))
	})

}
