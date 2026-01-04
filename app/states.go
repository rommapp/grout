package main

import (
	"grout/cache"
	"grout/cfw"
	"grout/constants"
	"grout/romm"
	"grout/sync"
	"grout/ui"
	"grout/utils"
	"os"
	gosync "sync"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

var (
	autoSync       *sync.AutoSync
	autoSyncOnce   gosync.Once
	autoUpdate     *utils.AutoUpdate
	autoUpdateOnce gosync.Once
)

const (
	platformSelection           gaba.StateName = "platform_selection"
	gameList                    gaba.StateName = "game_list"
	gameDetails                 gaba.StateName = "game_details"
	gameOptions                 gaba.StateName = "game_options"
	collectionList              gaba.StateName = "collection_list"
	collectionPlatformSelection gaba.StateName = "collection_platform_selection"
	search                      gaba.StateName = "search"
	collectionSearch            gaba.StateName = "collection_search"
	settings                    gaba.StateName = "settings"
	generalSettings             gaba.StateName = "general_settings"
	collectionsSettings         gaba.StateName = "collections_settings"
	advancedSettings            gaba.StateName = "advanced_settings"
	settingsPlatformMapping     gaba.StateName = "platform_mapping"
	saveSyncSettings            gaba.StateName = "save_sync_settings"
	info                        gaba.StateName = "info"
	logoutConfirmation          gaba.StateName = "logout_confirmation"
	clearCacheConfirmation      gaba.StateName = "clear_cache_confirmation"
	saveSync                    gaba.StateName = "save_sync"
	biosDownload                gaba.StateName = "bios_download"
	artworkSync                 gaba.StateName = "artwork_sync"
	updateCheck                 gaba.StateName = "update_check"
)

type ListPosition struct {
	Index             int
	VisibleStartIndex int
}

type NavState struct {
	CurrentGames []romm.Rom
	FullGames    []romm.Rom
	SearchFilter string
	HasBIOS      bool
	GameListPos  ListPosition

	CollectionSearchFilter string
	CollectionGames        []romm.Rom
	CollectionListPos      ListPosition
	CollectionPlatformPos  ListPosition

	PlatformListPos ListPosition

	SettingsPos            ListPosition
	CollectionsSettingsPos ListPosition
	AdvancedSettingsPos    ListPosition

	QuitOnBack      bool
	ShowCollections bool
}

func (s *NavState) ResetGameList() {
	s.CurrentGames = nil
	s.FullGames = nil
	s.SearchFilter = ""
	s.HasBIOS = false
	s.GameListPos = ListPosition{}
}

func buildFSM(config *utils.Config, c cfw.CFW, platforms []romm.Platform, quitOnBack bool, showCollections bool) *gaba.FSM {
	fsm := gaba.NewFSM()

	nav := &NavState{
		QuitOnBack:      quitOnBack,
		ShowCollections: showCollections,
	}

	gaba.Set(fsm.Context(), config)
	gaba.Set(fsm.Context(), c)
	gaba.Set(fsm.Context(), config.Hosts[0])
	gaba.Set(fsm.Context(), platforms)
	gaba.Set(fsm.Context(), nav)

	// Start background cache refresh immediately
	cache.InitRefresh(config.Hosts[0], config, platforms)

	// Validate artwork cache in background
	go cache.ValidateArtworkCache()

	gaba.AddState(fsm, platformSelection, func(ctx *gaba.Context) (ui.PlatformSelectionOutput, gaba.ExitCode) {
		platforms, _ := gaba.Get[[]romm.Platform](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewPlatformSelectionScreen()
		config, _ := gaba.Get[*utils.Config](ctx)
		currentCFW, _ := gaba.Get[cfw.CFW](ctx)

		// Start auto-sync on first platform menu view
		if config.SaveSyncMode == "automatic" {
			autoSyncOnce.Do(func() {
				host, _ := gaba.Get[romm.Host](ctx)
				autoSync = sync.NewAutoSync(host, config)
				utils.AddIcon(autoSync.Icon())
				autoSync.Start()
			})
		}

		// Start auto-update check on first platform menu view
		if currentCFW != cfw.NextUI {
			autoUpdateOnce.Do(func() {
				autoUpdate = utils.NewAutoUpdate(currentCFW)
				utils.AddIcon(autoUpdate.Icon())
				autoUpdate.Start()
			})
		}

		// Determine the sync button visibility control
		// - "off": nil (never show)
		// - "manual": always true, shows "Sync" button
		// - "automatic": controlled by auto-sync
		var showSaveSync *atomic.Bool
		switch config.SaveSyncMode {
		case "manual":
			showSaveSync = &atomic.Bool{}
			showSaveSync.Store(true)
		case "automatic":
			if autoSync != nil {
				showSaveSync = autoSync.ShowButton()
			}
		}

		result, err := screen.Draw(ui.PlatformSelectionInput{
			Platforms:            platforms,
			QuitOnBack:           nav.QuitOnBack,
			ShowCollections:      nav.ShowCollections,
			ShowSaveSync:         showSaveSync,
			LastSelectedIndex:    nav.PlatformListPos.Index,
			LastSelectedPosition: nav.PlatformListPos.VisibleStartIndex,
		})

		if err != nil {
			return ui.PlatformSelectionOutput{}, gaba.ExitCodeError
		}

		nav.PlatformListPos.Index = result.Value.LastSelectedIndex
		nav.PlatformListPos.VisibleStartIndex = result.Value.LastSelectedPosition

		if len(result.Value.ReorderedPlatforms) > 0 {
			config, _ := gaba.Get[*utils.Config](ctx)

			var platformOrder []string
			for _, p := range result.Value.ReorderedPlatforms {
				platformOrder = append(platformOrder, p.Slug)
			}

			gaba.GetLogger().Debug("Saving platform order to config", "order", platformOrder)

			config.PlatformOrder = platformOrder
			if err := utils.SaveConfig(config); err != nil {
				gaba.GetLogger().Error("Failed to save platform order", "error", err)
			} else {
				gaba.GetLogger().Info("Platform order saved successfully", "order", platformOrder)
			}

			gaba.Set(ctx, result.Value.ReorderedPlatforms)
			gaba.GetLogger().Debug("Updated platforms in context")
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.ResetGameList()
			gaba.Set(ctx, ui.CollectionSelectionOutput{})
			return nil
		}).
		OnWithHook(constants.ExitCodeCollections, collectionList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CollectionListPos = ListPosition{}
			return nil
		}).
		On(gaba.ExitCodeAction, settings).
		On(constants.ExitCodeSaveSync, saveSync).
		Exit(gaba.ExitCodeQuit)

	gaba.AddState(fsm, collectionList, func(ctx *gaba.Context) (ui.CollectionSelectionOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewCollectionSelectionScreen()
		result, err := screen.Draw(ui.CollectionSelectionInput{
			Config:               config,
			Host:                 host,
			SearchFilter:         nav.CollectionSearchFilter,
			LastSelectedIndex:    nav.CollectionListPos.Index,
			LastSelectedPosition: nav.CollectionListPos.VisibleStartIndex,
		})

		if err != nil {
			return ui.CollectionSelectionOutput{}, gaba.ExitCodeError
		}

		nav.CollectionListPos.Index = result.Value.LastSelectedIndex
		nav.CollectionListPos.VisibleStartIndex = result.Value.LastSelectedPosition
		nav.CollectionSearchFilter = result.Value.SearchFilter

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, collectionPlatformSelection, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.ResetGameList()
			nav.CollectionPlatformPos = ListPosition{}
			nav.CollectionGames = nil
			gaba.Set(ctx, ui.PlatformSelectionOutput{})
			return nil
		}).
		On(constants.ExitCodeSearch, collectionSearch).
		OnWithHook(constants.ExitCodeClearSearch, collectionList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CollectionSearchFilter = ""
			nav.CollectionListPos = ListPosition{}
			return nil
		}).
		On(gaba.ExitCodeBack, platformSelection)

	gaba.AddState(fsm, collectionPlatformSelection, func(ctx *gaba.Context) (ui.CollectionPlatformSelectionOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		collection, _ := gaba.Get[ui.CollectionSelectionOutput](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewCollectionPlatformSelectionScreen()
		result, err := screen.Draw(ui.CollectionPlatformSelectionInput{
			Config:               config,
			Host:                 host,
			Collection:           collection.SelectedCollection,
			CachedGames:          nav.CollectionGames,
			LastSelectedIndex:    nav.CollectionPlatformPos.Index,
			LastSelectedPosition: nav.CollectionPlatformPos.VisibleStartIndex,
		})

		if err != nil {
			return ui.CollectionPlatformSelectionOutput{}, gaba.ExitCodeError
		}

		nav.CollectionPlatformPos.Index = result.Value.LastSelectedIndex
		nav.CollectionPlatformPos.VisibleStartIndex = result.Value.LastSelectedPosition
		nav.CollectionGames = result.Value.AllGames

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)
			config, _ := gaba.Get[*utils.Config](ctx)
			nav, _ := gaba.Get[*NavState](ctx)

			var finalGames []romm.Rom

			// In unified mode with Platform.ID == 0, use all games
			if config.CollectionView == "unified" && output.SelectedPlatform.ID == 0 {
				finalGames = output.AllGames
			} else {
				// Platform mode: filter by selected platform
				filteredGames := make([]romm.Rom, 0)
				for _, game := range output.AllGames {
					if game.PlatformID == output.SelectedPlatform.ID {
						filteredGames = append(filteredGames, game)
					}
				}
				finalGames = filteredGames
			}

			nav.SearchFilter = ""
			nav.FullGames = finalGames
			nav.CurrentGames = finalGames
			nav.GameListPos = ListPosition{}
			return nil
		}).
		On(gaba.ExitCodeBack, collectionList)

	gaba.AddState(fsm, gameList, func(ctx *gaba.Context) (ui.GameListOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		platform, _ := gaba.Get[ui.PlatformSelectionOutput](ctx)
		collection, _ := gaba.Get[ui.CollectionSelectionOutput](ctx)
		collectionPlatform, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		var selectedPlatform romm.Platform
		var selectedCollection romm.Collection

		if collectionPlatform.SelectedPlatform.ID != 0 {
			selectedPlatform = collectionPlatform.SelectedPlatform
			selectedCollection = collectionPlatform.Collection
		} else {
			selectedPlatform = platform.SelectedPlatform
			selectedCollection = collection.SelectedCollection
		}

		screen := ui.NewGameListScreen()
		result, err := screen.Draw(ui.GameListInput{
			Config:               config,
			Host:                 host,
			Platform:             selectedPlatform,
			Collection:           selectedCollection,
			Games:                nav.CurrentGames,
			HasBIOS:              nav.HasBIOS,
			SearchFilter:         nav.SearchFilter,
			LastSelectedIndex:    nav.GameListPos.Index,
			LastSelectedPosition: nav.GameListPos.VisibleStartIndex,
		})

		if err != nil {
			return ui.GameListOutput{}, gaba.ExitCodeError
		}

		nav.FullGames = result.Value.AllGames
		nav.CurrentGames = result.Value.AllGames
		nav.HasBIOS = result.Value.HasBIOS
		nav.GameListPos.Index = result.Value.LastSelectedIndex
		nav.GameListPos.VisibleStartIndex = result.Value.LastSelectedPosition
		nav.SearchFilter = result.Value.SearchFilter

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeSuccess, gameDetails).
		On(constants.ExitCodeSearch, search).
		On(constants.ExitCodeBIOS, biosDownload).
		OnWithHook(constants.ExitCodeClearSearch, gameList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.SearchFilter = ""
			nav.CurrentGames = nav.FullGames
			nav.GameListPos = ListPosition{}
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, platformSelection, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CurrentGames = nil
			return nil
		}).
		On(constants.ExitCodeBackToCollectionPlatform, collectionPlatformSelection).
		OnWithHook(constants.ExitCodeBackToCollection, collectionList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CurrentGames = nil
			return nil
		}).
		On(constants.ExitCodeNoResults, search)

	gaba.AddState(fsm, gameDetails, func(ctx *gaba.Context) (ui.GameDetailsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		gameListOutput, _ := gaba.Get[ui.GameListOutput](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		// If multiple games selected, skip details and go straight to download
		if len(gameListOutput.SelectedGames) != 1 {
			downloadScreen := ui.NewDownloadScreen()
			downloadOutput := downloadScreen.Execute(*config, host, gameListOutput.Platform, gameListOutput.SelectedGames, gameListOutput.AllGames, nav.SearchFilter)
			nav.CurrentGames = downloadOutput.AllGames
			nav.SearchFilter = downloadOutput.SearchFilter
			triggerAutoSync()
			return ui.GameDetailsOutput{}, gaba.ExitCodeBack
		}

		screen := ui.NewGameDetailsScreen()
		result, err := screen.Draw(ui.GameDetailsInput{
			Config:   config,
			Host:     host,
			Platform: gameListOutput.Platform,
			Game:     gameListOutput.SelectedGames[0],
		})

		if err != nil {
			return ui.GameDetailsOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameList, func(ctx *gaba.Context) error {
			detailsOutput, _ := gaba.Get[ui.GameDetailsOutput](ctx)
			config, _ := gaba.Get[*utils.Config](ctx)
			host, _ := gaba.Get[romm.Host](ctx)
			gameListOutput, _ := gaba.Get[ui.GameListOutput](ctx)
			nav, _ := gaba.Get[*NavState](ctx)

			if detailsOutput.DownloadRequested {
				downloadScreen := ui.NewDownloadScreen()
				downloadOutput := downloadScreen.Execute(*config, host, detailsOutput.Platform, []romm.Rom{detailsOutput.Game}, gameListOutput.AllGames, nav.SearchFilter)
				nav.CurrentGames = downloadOutput.AllGames
				nav.SearchFilter = downloadOutput.SearchFilter
				triggerAutoSync()
			}

			return nil
		}).
		On(gaba.ExitCodeBack, gameList).
		On(constants.ExitCodeGameOptions, gameOptions)

	// Game options state
	gaba.AddState(fsm, gameOptions, func(ctx *gaba.Context) (ui.GameOptionsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		gameListOutput, _ := gaba.Get[ui.GameListOutput](ctx)

		if len(gameListOutput.SelectedGames) != 1 {
			return ui.GameOptionsOutput{Config: config}, gaba.ExitCodeBack
		}

		screen := ui.NewGameOptionsScreen()
		result, err := screen.Draw(ui.GameOptionsInput{
			Config: config,
			Game:   gameListOutput.SelectedGames[0],
		})

		if err != nil {
			return ui.GameOptionsOutput{Config: config}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameDetails, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.GameOptionsOutput](ctx)
			gaba.Set(ctx, output.Config)
			return nil
		}).
		On(gaba.ExitCodeBack, gameDetails)

	gaba.AddState(fsm, search, func(ctx *gaba.Context) (ui.SearchOutput, gaba.ExitCode) {
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewSearchScreen()
		result, err := screen.Draw(ui.SearchInput{
			InitialText: nav.SearchFilter,
		})

		if err != nil {
			return ui.SearchOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SearchOutput](ctx)
			nav, _ := gaba.Get[*NavState](ctx)
			nav.SearchFilter = output.Query
			nav.CurrentGames = nav.FullGames
			nav.GameListPos = ListPosition{}
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, gameList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.SearchFilter = ""
			nav.CurrentGames = nav.FullGames
			return nil
		})

	gaba.AddState(fsm, collectionSearch, func(ctx *gaba.Context) (ui.SearchOutput, gaba.ExitCode) {
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewSearchScreen()
		result, err := screen.Draw(ui.SearchInput{
			InitialText: nav.CollectionSearchFilter,
		})

		if err != nil {
			return ui.SearchOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, collectionList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SearchOutput](ctx)
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CollectionSearchFilter = output.Query
			nav.CollectionListPos = ListPosition{}
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, collectionList, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CollectionSearchFilter = ""
			return nil
		})

	gaba.AddState(fsm, settings, func(ctx *gaba.Context) (ui.SettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		currentCFW, _ := gaba.Get[cfw.CFW](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewSettingsScreen()
		result, err := screen.Draw(ui.SettingsInput{
			Config:                config,
			CFW:                   currentCFW,
			Host:                  host,
			LastSelectedIndex:     nav.SettingsPos.Index,
			LastVisibleStartIndex: nav.SettingsPos.VisibleStartIndex,
		})

		if err != nil {
			return ui.SettingsOutput{}, gaba.ExitCodeError
		}

		nav.SettingsPos.Index = result.Value.LastSelectedIndex
		nav.SettingsPos.VisibleStartIndex = result.Value.LastVisibleStartIndex

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, platformSelection, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SettingsOutput](ctx)
			host, _ := gaba.Get[romm.Host](ctx)
			nav, _ := gaba.Get[*NavState](ctx)
			utils.SaveConfig(output.Config)
			gaba.Set(ctx, output.Config)
			nav.SettingsPos = ListPosition{}

			nav.ShowCollections = utils.ShowCollections(output.Config, host)
			return nil
		}).
		On(constants.ExitCodeGeneralSettings, generalSettings).
		On(constants.ExitCodeCollectionsSettings, collectionsSettings).
		On(constants.ExitCodeAdvancedSettings, advancedSettings).
		On(constants.ExitCodeSaveSyncSettings, saveSyncSettings).
		On(constants.ExitCodeInfo, info).
		On(constants.ExitCodeCheckUpdate, updateCheck).
		OnWithHook(gaba.ExitCodeBack, platformSelection, func(ctx *gaba.Context) error {
			nav, _ := gaba.Get[*NavState](ctx)
			nav.SettingsPos = ListPosition{}
			return nil
		})

	gaba.AddState(fsm, generalSettings, func(ctx *gaba.Context) (ui.GeneralSettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)

		screen := ui.NewGeneralSettingsScreen()
		result, err := screen.Draw(ui.GeneralSettingsInput{
			Config: config,
		})

		if err != nil {
			return ui.GeneralSettingsOutput{Config: config}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, settings, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.GeneralSettingsOutput](ctx)
			gaba.Set(ctx, output.Config)
			return nil
		}).
		On(gaba.ExitCodeBack, settings)

	gaba.AddState(fsm, collectionsSettings, func(ctx *gaba.Context) (ui.CollectionsSettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)

		screen := ui.NewCollectionsSettingsScreen()
		result, err := screen.Draw(ui.CollectionsSettingsInput{
			Config: config,
		})

		if err != nil {
			return ui.CollectionsSettingsOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, settings, func(ctx *gaba.Context) error {
			config, _ := gaba.Get[*utils.Config](ctx)
			host, _ := gaba.Get[romm.Host](ctx)
			nav, _ := gaba.Get[*NavState](ctx)
			nav.CollectionsSettingsPos = ListPosition{}
			nav.ShowCollections = utils.ShowCollections(config, host)
			return nil
		}).
		On(gaba.ExitCodeBack, settings)

	gaba.AddState(fsm, saveSyncSettings, func(ctx *gaba.Context) (ui.SaveSyncSettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		currentCFW, _ := gaba.Get[cfw.CFW](ctx)

		screen := ui.NewSaveSyncSettingsScreen()
		result, err := screen.Draw(ui.SaveSyncSettingsInput{
			Config: config,
			CFW:    currentCFW,
		})

		if err != nil {
			return ui.SaveSyncSettingsOutput{Config: config}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, settings, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SaveSyncSettingsOutput](ctx)
			gaba.Set(ctx, output.Config)
			triggerAutoSync()
			return nil
		}).
		On(gaba.ExitCodeBack, settings)

	gaba.AddState(fsm, advancedSettings, func(ctx *gaba.Context) (ui.AdvancedSettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		nav, _ := gaba.Get[*NavState](ctx)

		screen := ui.NewAdvancedSettingsScreen()
		result, err := screen.Draw(ui.AdvancedSettingsInput{
			Config:                config,
			Host:                  host,
			LastSelectedIndex:     nav.AdvancedSettingsPos.Index,
			LastVisibleStartIndex: nav.AdvancedSettingsPos.VisibleStartIndex,
		})

		if err != nil {
			return ui.AdvancedSettingsOutput{}, gaba.ExitCodeError
		}

		nav.AdvancedSettingsPos.Index = result.Value.LastSelectedIndex
		nav.AdvancedSettingsPos.VisibleStartIndex = result.Value.LastVisibleStartIndex

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeSuccess, settings).
		On(constants.ExitCodeEditMappings, settingsPlatformMapping).
		On(constants.ExitCodeClearCache, clearCacheConfirmation).
		On(constants.ExitCodeSyncArtwork, artworkSync).
		On(gaba.ExitCodeBack, settings)

	gaba.AddState(fsm, settingsPlatformMapping, func(ctx *gaba.Context) (ui.PlatformMappingOutput, gaba.ExitCode) {
		host, _ := gaba.Get[romm.Host](ctx)
		config, _ := gaba.Get[*utils.Config](ctx)
		currentCFW, _ := gaba.Get[cfw.CFW](ctx)

		screen := ui.NewPlatformMappingScreen()
		result, err := screen.Draw(ui.PlatformMappingInput{
			Host:           host,
			ApiTimeout:     config.ApiTimeout,
			CFW:            currentCFW,
			RomDirectory:   cfw.GetRomDirectory(),
			AutoSelect:     false,
			HideBackButton: false,
		})

		if err != nil {
			return ui.PlatformMappingOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, advancedSettings, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.PlatformMappingOutput](ctx)
			config, _ := gaba.Get[*utils.Config](ctx)
			host, _ := gaba.Get[romm.Host](ctx)

			config.DirectoryMappings = output.Mappings
			config.PlatformOrder = utils.PrunePlatformOrder(config.PlatformOrder, output.Mappings)
			utils.SaveConfig(config)
			gaba.Set(ctx, config)

			platforms, err := utils.GetMappedPlatforms(host, output.Mappings, config.ApiTimeout)
			if err != nil {
				gaba.GetLogger().Error("Failed to load platforms", "error", err)
				return err
			}
			gaba.Set(ctx, platforms)
			return nil
		}).
		On(gaba.ExitCodeBack, advancedSettings)

	gaba.AddState(fsm, info, func(ctx *gaba.Context) (ui.InfoOutput, gaba.ExitCode) {
		host, _ := gaba.Get[romm.Host](ctx)

		screen := ui.NewInfoScreen()
		result, err := screen.Draw(ui.InfoInput{
			Host: host,
		})

		if err != nil {
			return ui.InfoOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeBack, settings).
		On(constants.ExitCodeLogoutConfirm, logoutConfirmation)

	gaba.AddState(fsm, logoutConfirmation, func(ctx *gaba.Context) (ui.LogoutConfirmationOutput, gaba.ExitCode) {
		screen := ui.NewLogoutConfirmationScreen()
		result, err := screen.Draw()

		if err != nil {
			return ui.LogoutConfirmationOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeBack, info).
		OnWithHook(constants.ExitCodeLogout, platformSelection, func(ctx *gaba.Context) error {
			config, _ := gaba.Get[*utils.Config](ctx)
			currentCFW, _ := gaba.Get[cfw.CFW](ctx)

			config.Hosts = nil
			config.DirectoryMappings = nil
			config.PlatformOrder = nil

			if err := utils.SaveConfig(config); err != nil {
				gaba.GetLogger().Error("Failed to save config after logout", "error", err)
				return err
			}

			gaba.GetLogger().Info("User logged out successfully")

			loginConfig, err := ui.LoginFlow(romm.Host{})
			if err != nil {
				gaba.GetLogger().Error("Login flow failed after logout", "error", err)
				return err
			}

			config.Hosts = loginConfig.Hosts
			if err := utils.SaveConfig(config); err != nil {
				gaba.GetLogger().Error("Failed to save config after re-login", "error", err)
				return err
			}

			gaba.Set(ctx, config)
			gaba.Set(ctx, config.Hosts[0])

			if len(config.DirectoryMappings) == 0 {
				screen := ui.NewPlatformMappingScreen()
				result, err := screen.Draw(ui.PlatformMappingInput{
					Host:           config.Hosts[0],
					ApiTimeout:     config.ApiTimeout,
					CFW:            currentCFW,
					RomDirectory:   cfw.GetRomDirectory(),
					AutoSelect:     false,
					HideBackButton: true,
				})

				if err == nil && result.ExitCode == gaba.ExitCodeSuccess {
					config.DirectoryMappings = result.Value.Mappings
					utils.SaveConfig(config)
				}
			}

			platforms, err := utils.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings, config.ApiTimeout)
			if err != nil {
				gaba.GetLogger().Error("Failed to load platforms after re-login", "error", err)
				return err
			}
			gaba.Set(ctx, platforms)

			nav, _ := gaba.Get[*NavState](ctx)
			nav.ResetGameList()
			nav.PlatformListPos = ListPosition{}

			return nil
		})

	gaba.AddState(fsm, clearCacheConfirmation, func(ctx *gaba.Context) (ui.ClearCacheOutput, gaba.ExitCode) {
		screen := ui.NewClearCacheScreen()
		result, err := screen.Draw()

		if err != nil {
			return ui.ClearCacheOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeBack, advancedSettings).
		On(gaba.ExitCodeSuccess, advancedSettings)

	gaba.AddState(fsm, saveSync, func(ctx *gaba.Context) (ui.SaveSyncOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)

		screen := ui.NewSaveSyncScreen()
		result, err := screen.Draw(ui.SaveSyncInput{
			Config: config,
			Host:   host,
		})

		if err != nil {
			return ui.SaveSyncOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeBack, platformSelection)

	gaba.AddState(fsm, biosDownload, func(ctx *gaba.Context) (ui.BIOSDownloadOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		platform, _ := gaba.Get[ui.PlatformSelectionOutput](ctx)
		collectionPlatform, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)

		var selectedPlatform romm.Platform
		if collectionPlatform.SelectedPlatform.ID != 0 {
			selectedPlatform = collectionPlatform.SelectedPlatform
		} else {
			selectedPlatform = platform.SelectedPlatform
		}

		screen := ui.NewBIOSDownloadScreen()
		output := screen.Execute(*config, host, selectedPlatform)

		return output, gaba.ExitCodeBack
	}).
		On(gaba.ExitCodeBack, gameList)

	gaba.AddState(fsm, artworkSync, func(ctx *gaba.Context) (ui.ArtworkSyncOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)

		screen := ui.NewArtworkSyncScreen()
		output := screen.Execute(*config, host)

		return output, gaba.ExitCodeBack
	}).
		On(gaba.ExitCodeBack, advancedSettings)

	gaba.AddState(fsm, updateCheck, func(ctx *gaba.Context) (ui.UpdateOutput, gaba.ExitCode) {
		currentCFW, _ := gaba.Get[cfw.CFW](ctx)

		screen := ui.NewUpdateScreen()
		result, err := screen.Draw(ui.UpdateInput{
			CFW: currentCFW,
		})

		if err != nil {
			return ui.UpdateOutput{}, gaba.ExitCodeError
		}

		if result.Value.UpdatePerformed {
			os.Exit(0)
		}

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeBack, settings).
		On(gaba.ExitCodeSuccess, settings)

	return fsm.Start(platformSelection)
}

func triggerAutoSync() {
	if autoSync != nil {
		autoSync.Trigger()
	}
}
