package main

import (
	"grout/constants"
	"grout/constants/cfw/muos"
	"grout/resources"
	"grout/ui"
	"grout/utils"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	_ "github.com/UncleJunVIP/certifiable"
)

const (
	platformSelection           gaba.StateName = "platform_selection"
	gameList                                   = "game_list"
	gameDetails                                = "game_details"
	collectionList                             = "collection_list"
	collectionPlatformSelection                = "collection_platform_selection"
	search                                     = "search"
	collectionSearch                           = "collection_search"
	settings                                   = "settings"
	settingsPlatformMapping                    = "platform_mapping"
	info                                       = "info"
	saveSync                                   = "save_sync"
	biosDownload                               = "bios_download"
)

type (
	currentGamesList       []romm.Rom
	fullGamesList          []romm.Rom
	searchFilterString     string
	collectionSearchFilter string
	quitOnBackBool         bool

	showCollectionsBool bool

	gameListPosition struct {
		Index int
		Pos   int
	}

	platformListPosition struct {
		Index int
		Pos   int
	}

	collectionListPosition struct {
		Index int
		Pos   int
	}

	collectionPlatformListPosition struct {
		Index int
		Pos   int
	}

	settingsPosition struct {
		Index int
	}

	cachedCollectionGames    []romm.Rom
	cachedRegularCollections []romm.Collection
	cachedSmartCollections   []romm.Collection
	cachedVirtualCollections []romm.VirtualCollection
)

func setup() *utils.Config {
	cfw := utils.GetCFW()

	// Set up input mapping for muOS with auto-detection
	if cfw == constants.MuOS && !utils.IsDevelopment() {
		if cwd, err := os.Getwd(); err == nil {
			cwdMappingPath := filepath.Join(cwd, "input_mapping.json")
			if _, err := os.Stat(cwdMappingPath); err == nil {
				// User-provided mapping takes priority
				os.Setenv("INPUT_MAPPING_PATH", cwdMappingPath)
			} else {
				// Use embedded mapping with auto-detection
				if mappingBytes, err := muos.GetInputMappingBytes(); err == nil {
					gaba.SetInputMappingBytes(mappingBytes)
				}
			}
		}
	}

	gaba.Init(gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             cfw == constants.NextUI,
		LogFilename:          "grout.log",
	})

	localeFiles, err := resources.GetLocaleMessageFiles()
	if err != nil {
		utils.LogStandardFatal("Failed to load locale files", err)
	}
	if err := i18n.InitI18NFromBytes(localeFiles); err != nil {
		utils.LogStandardFatal("Failed to initialize i18n", err)
	}

	gaba.SetLogLevel(slog.LevelDebug)

	splashBytes, err := resources.GetSplashImageBytes()
	if err != nil {
		utils.LogStandardFatal("Failed to load splash image", err)
	}

	gaba.ProcessMessage("", gaba.ProcessMessageOptions{
		ImageBytes:  splashBytes,
		ImageWidth:  768,
		ImageHeight: 540,
	}, func() (interface{}, error) {
		time.Sleep(750 * time.Millisecond)
		return nil, nil
	})

	gaba.GetLogger().Debug("Loading configuration from config.json")
	config, err := utils.LoadConfig()
	if err != nil || len(config.Hosts) == 0 {
		gaba.GetLogger().Debug("No RomM Host Configured", "error", err)
		gaba.GetLogger().Debug("Starting login flow for initial setup")
		loginConfig, loginErr := ui.LoginFlow(romm.Host{})
		if loginErr != nil {
			gaba.GetLogger().Error("Login flow failed", "error", loginErr)
			utils.LogStandardFatal("Login failed", loginErr)
		}
		gaba.GetLogger().Debug("Login successful, saving configuration")
		config = loginConfig
		utils.SaveConfig(config)
	} else {
		gaba.GetLogger().Debug("Configuration loaded successfully", "host_count", len(config.Hosts))
	}

	if config.LogLevel != "" {
		gaba.SetRawLogLevel(config.LogLevel)
	}

	if config.Language != "" {
		if err := i18n.SetWithCode(config.Language); err != nil {
			gaba.GetLogger().Error("Failed to set language", "error", err, "language", config.Language)
		}
	}

	if config.DirectoryMappings == nil || len(config.DirectoryMappings) == 0 {
		screen := ui.NewPlatformMappingScreen()
		result, err := screen.Draw(ui.PlatformMappingInput{
			Host:           config.Hosts[0],
			ApiTimeout:     config.ApiTimeout,
			CFW:            cfw,
			RomDirectory:   utils.GetRomDirectory(),
			AutoSelect:     false,
			HideBackButton: true,
		})

		if err == nil && result.ExitCode == gaba.ExitCodeSuccess {
			config.DirectoryMappings = result.Value.Mappings
			utils.SaveConfig(config)
		}
	}

	gaba.GetLogger().Debug("Configuration Loaded!", "config", config.ToLoggable())
	return config
}

func cleanup() {
	if err := os.RemoveAll(".tmp"); err != nil {
		gaba.GetLogger().Error("Failed to clean .tmp directory", "error", err)
	}
	gaba.Close()
}

func main() {
	defer cleanup()

	config := setup()

	logger := gaba.GetLogger()
	logger.Debug("Starting Grout")

	cfw := utils.GetCFW()

	quitOnBack := len(config.Hosts) == 1
	platforms, err := utils.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings)
	if err != nil {
		gaba.ConfirmationMessage(i18n.GetString("error_loading_platforms"),
			[]gaba.FooterHelpItem{
				{ButtonName: "A", HelpText: i18n.GetString("button_continue")},
			},
			gaba.MessageOptions{})
		gaba.GetLogger().Error("Failed to load platforms", "error", err)
		os.Exit(1)
	}

	platforms = utils.SortPlatformsByOrder(platforms, config.PlatformOrder)

	showCollections := utils.ShowCollections(config, config.Hosts[0])

	fsm := buildFSM(config, cfw, platforms, quitOnBack, showCollections)

	if err := fsm.Run(); err != nil {
		logger.Error("FSM error", "error", err)
	}
}

func buildFSM(config *utils.Config, cfw constants.CFW, platforms []romm.Platform, quitOnBack bool, showCollections bool) *gaba.FSM {
	fsm := gaba.NewFSM()

	gaba.Set(fsm.Context(), config)
	gaba.Set(fsm.Context(), cfw)
	gaba.Set(fsm.Context(), config.Hosts[0])
	gaba.Set(fsm.Context(), platforms)
	gaba.Set(fsm.Context(), quitOnBackBool(quitOnBack))
	gaba.Set(fsm.Context(), showCollectionsBool(showCollections))
	gaba.Set(fsm.Context(), searchFilterString(""))

	gaba.AddState(fsm, platformSelection, func(ctx *gaba.Context) (ui.PlatformSelectionOutput, gaba.ExitCode) {
		platforms, _ := gaba.Get[[]romm.Platform](ctx)
		quitOnBack, _ := gaba.Get[quitOnBackBool](ctx)
		showCollections, _ := gaba.Get[showCollectionsBool](ctx)
		platPos, _ := gaba.Get[platformListPosition](ctx)

		screen := ui.NewPlatformSelectionScreen()
		config, _ := gaba.Get[*utils.Config](ctx)
		result, err := screen.Draw(ui.PlatformSelectionInput{
			Platforms:            platforms,
			QuitOnBack:           bool(quitOnBack),
			ShowCollections:      bool(showCollections),
			ShowSaveSync:         config.SaveSyncMode != "off",
			LastSelectedIndex:    platPos.Index,
			LastSelectedPosition: platPos.Pos,
		})

		if err != nil {
			return ui.PlatformSelectionOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, platformListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})

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
			gaba.Set(ctx, searchFilterString(""))
			gaba.Set(ctx, currentGamesList(nil))
			gaba.Set(ctx, gameListPosition{Index: 0, Pos: 0})
			gaba.Set(ctx, ui.CollectionSelectionOutput{})
			return nil
		}).
		OnWithHook(constants.ExitCodeCollections, collectionList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, collectionListPosition{
				Index: 0,
				Pos:   0,
			})
			return nil
		}).
		On(gaba.ExitCodeAction, settings).
		On(constants.ExitCodeSaveSync, saveSync).
		Exit(gaba.ExitCodeQuit)

	gaba.AddState(fsm, collectionList, func(ctx *gaba.Context) (ui.CollectionSelectionOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		colPos, _ := gaba.Get[collectionListPosition](ctx)
		searchFilter, _ := gaba.Get[collectionSearchFilter](ctx)
		cachedRegular, _ := gaba.Get[cachedRegularCollections](ctx)
		cachedSmart, _ := gaba.Get[cachedSmartCollections](ctx)
		cachedVirtual, _ := gaba.Get[cachedVirtualCollections](ctx)

		screen := ui.NewCollectionSelectionScreen()
		result, err := screen.Draw(ui.CollectionSelectionInput{
			Config:                   config,
			Host:                     host,
			SearchFilter:             string(searchFilter),
			LastSelectedIndex:        colPos.Index,
			LastSelectedPosition:     colPos.Pos,
			CachedRegularCollections: cachedRegular,
			CachedSmartCollections:   cachedSmart,
			CachedVirtualCollections: cachedVirtual,
		})

		if err != nil {
			return ui.CollectionSelectionOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, collectionListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})
		gaba.Set(ctx, collectionSearchFilter(result.Value.SearchFilter))
		gaba.Set(ctx, cachedRegularCollections(result.Value.FetchedRegularCollections))
		gaba.Set(ctx, cachedSmartCollections(result.Value.FetchedSmartCollections))
		gaba.Set(ctx, cachedVirtualCollections(result.Value.FetchedVirtualCollections))

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, collectionPlatformSelection, func(ctx *gaba.Context) error {
			gaba.Set(ctx, searchFilterString(""))
			gaba.Set(ctx, currentGamesList(nil))
			gaba.Set(ctx, gameListPosition{Index: 0, Pos: 0})
			gaba.Set(ctx, collectionPlatformListPosition{Index: 0, Pos: 0})
			gaba.Set(ctx, cachedCollectionGames(nil))
			gaba.Set(ctx, ui.PlatformSelectionOutput{})
			return nil
		}).
		On(constants.ExitCodeSearch, collectionSearch).
		OnWithHook(constants.ExitCodeClearSearch, collectionList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, collectionSearchFilter(""))
			gaba.Set(ctx, collectionListPosition{Index: 0, Pos: 0})
			return nil
		}).
		On(gaba.ExitCodeBack, platformSelection)

	gaba.AddState(fsm, collectionPlatformSelection, func(ctx *gaba.Context) (ui.CollectionPlatformSelectionOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		collection, _ := gaba.Get[ui.CollectionSelectionOutput](ctx)
		pos, _ := gaba.Get[collectionPlatformListPosition](ctx)
		cachedGames, _ := gaba.Get[cachedCollectionGames](ctx)

		screen := ui.NewCollectionPlatformSelectionScreen()
		result, err := screen.Draw(ui.CollectionPlatformSelectionInput{
			Config:               config,
			Host:                 host,
			Collection:           collection.SelectedCollection,
			CachedGames:          cachedGames,
			LastSelectedIndex:    pos.Index,
			LastSelectedPosition: pos.Pos,
		})

		if err != nil {
			return ui.CollectionPlatformSelectionOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, collectionPlatformListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})

		gaba.Set(ctx, cachedCollectionGames(result.Value.AllGames))

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)

			filteredGames := make([]romm.Rom, 0)
			for _, game := range output.AllGames {
				if game.PlatformID == output.SelectedPlatform.ID {
					filteredGames = append(filteredGames, game)
				}
			}

			gaba.Set(ctx, searchFilterString(""))
			gaba.Set(ctx, fullGamesList(filteredGames))
			gaba.Set(ctx, currentGamesList(filteredGames))
			gaba.Set(ctx, gameListPosition{Index: 0, Pos: 0})
			return nil
		}).
		On(gaba.ExitCodeBack, collectionList)

	gaba.AddState(fsm, gameList, func(ctx *gaba.Context) (ui.GameListOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		platform, _ := gaba.Get[ui.PlatformSelectionOutput](ctx)
		collection, _ := gaba.Get[ui.CollectionSelectionOutput](ctx)
		collectionPlatform, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)
		games, _ := gaba.Get[currentGamesList](ctx)
		filter, _ := gaba.Get[searchFilterString](ctx)
		pos, _ := gaba.Get[gameListPosition](ctx)

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
			Games:                games,
			SearchFilter:         string(filter),
			LastSelectedIndex:    pos.Index,
			LastSelectedPosition: pos.Pos,
		})

		if err != nil {
			return ui.GameListOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, fullGamesList(result.Value.AllGames))
		gaba.Set(ctx, currentGamesList(result.Value.AllGames))
		gaba.Set(ctx, gameListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})
		gaba.Set(ctx, searchFilterString(result.Value.SearchFilter))

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeSuccess, gameDetails).
		On(constants.ExitCodeSearch, search).
		On(constants.ExitCodeBIOS, biosDownload).
		OnWithHook(constants.ExitCodeClearSearch, gameList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, searchFilterString(""))
			fullGames, _ := gaba.Get[fullGamesList](ctx)
			gaba.Set(ctx, currentGamesList(fullGames))
			gaba.Set(ctx, gameListPosition{Index: 0, Pos: 0})
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, platformSelection, func(ctx *gaba.Context) error {
			gaba.Set(ctx, currentGamesList(nil))
			return nil
		}).
		On(constants.ExitCodeBackToCollectionPlatform, collectionPlatformSelection).
		OnWithHook(constants.ExitCodeBackToCollection, collectionList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, currentGamesList(nil))
			return nil
		}).
		On(constants.ExitCodeNoResults, search)

	gaba.AddState(fsm, gameDetails, func(ctx *gaba.Context) (ui.GameDetailsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		gameListOutput, _ := gaba.Get[ui.GameListOutput](ctx)

		if !config.ShowGameDetails || len(gameListOutput.SelectedGames) != 1 {
			filter, _ := gaba.Get[searchFilterString](ctx)
			downloadScreen := ui.NewDownloadScreen()
			downloadOutput := downloadScreen.Execute(*config, host, gameListOutput.Platform, gameListOutput.SelectedGames, gameListOutput.AllGames, string(filter))
			gaba.Set(ctx, currentGamesList(downloadOutput.AllGames))
			gaba.Set(ctx, searchFilterString(downloadOutput.SearchFilter))
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
			filter, _ := gaba.Get[searchFilterString](ctx)

			if detailsOutput.DownloadRequested {
				downloadScreen := ui.NewDownloadScreen()
				downloadOutput := downloadScreen.Execute(*config, host, detailsOutput.Platform, []romm.Rom{detailsOutput.Game}, gameListOutput.AllGames, string(filter))
				gaba.Set(ctx, currentGamesList(downloadOutput.AllGames))
				gaba.Set(ctx, searchFilterString(downloadOutput.SearchFilter))
			}

			return nil
		}).
		On(gaba.ExitCodeBack, gameList)

	gaba.AddState(fsm, search, func(ctx *gaba.Context) (ui.SearchOutput, gaba.ExitCode) {
		filter, _ := gaba.Get[searchFilterString](ctx)

		screen := ui.NewSearchScreen()
		result, err := screen.Draw(ui.SearchInput{
			InitialText: string(filter),
		})

		if err != nil {
			return ui.SearchOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, gameList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SearchOutput](ctx)
			gaba.Set(ctx, searchFilterString(output.Query))
			fullGames, _ := gaba.Get[fullGamesList](ctx)
			gaba.Set(ctx, currentGamesList(fullGames))
			gaba.Set(ctx, gameListPosition{Index: 0, Pos: 0})
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, gameList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, searchFilterString(""))
			fullGames, _ := gaba.Get[fullGamesList](ctx)
			gaba.Set(ctx, currentGamesList(fullGames))
			return nil
		})

	gaba.AddState(fsm, collectionSearch, func(ctx *gaba.Context) (ui.SearchOutput, gaba.ExitCode) {
		filter, _ := gaba.Get[collectionSearchFilter](ctx)

		screen := ui.NewSearchScreen()
		result, err := screen.Draw(ui.SearchInput{
			InitialText: string(filter),
		})

		if err != nil {
			return ui.SearchOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, collectionList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SearchOutput](ctx)
			gaba.Set(ctx, collectionSearchFilter(output.Query))
			gaba.Set(ctx, collectionListPosition{Index: 0, Pos: 0})
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, collectionList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, collectionSearchFilter(""))
			return nil
		})

	gaba.AddState(fsm, settings, func(ctx *gaba.Context) (ui.SettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*utils.Config](ctx)
		cfw, _ := gaba.Get[constants.CFW](ctx)
		host, _ := gaba.Get[romm.Host](ctx)
		pos, _ := gaba.Get[settingsPosition](ctx)

		screen := ui.NewSettingsScreen()
		result, err := screen.Draw(ui.SettingsInput{
			Config:            config,
			CFW:               cfw,
			Host:              host,
			LastSelectedIndex: pos.Index,
		})

		if err != nil {
			return ui.SettingsOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, settingsPosition{Index: result.Value.LastSelectedIndex})

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, platformSelection, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SettingsOutput](ctx)
			host, _ := gaba.Get[romm.Host](ctx)
			utils.SaveConfig(output.Config)
			gaba.Set(ctx, output.Config)
			gaba.Set(ctx, settingsPosition{Index: 0})

			// Update showCollections based on new settings
			showCollections := utils.ShowCollections(output.Config, host)
			gaba.Set(ctx, showCollectionsBool(showCollections))
			return nil
		}).
		On(constants.ExitCodeEditMappings, settingsPlatformMapping).
		On(constants.ExitCodeInfo, info).
		OnWithHook(gaba.ExitCodeBack, platformSelection, func(ctx *gaba.Context) error {
			gaba.Set(ctx, settingsPosition{Index: 0})
			return nil
		})

	gaba.AddState(fsm, settingsPlatformMapping, func(ctx *gaba.Context) (ui.PlatformMappingOutput, gaba.ExitCode) {
		host, _ := gaba.Get[romm.Host](ctx)
		config, _ := gaba.Get[*utils.Config](ctx)
		cfw, _ := gaba.Get[constants.CFW](ctx)

		screen := ui.NewPlatformMappingScreen()
		result, err := screen.Draw(ui.PlatformMappingInput{
			Host:           host,
			ApiTimeout:     config.ApiTimeout,
			CFW:            cfw,
			RomDirectory:   utils.GetRomDirectory(),
			AutoSelect:     false,
			HideBackButton: false,
		})

		if err != nil {
			return ui.PlatformMappingOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, settings, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.PlatformMappingOutput](ctx)
			config, _ := gaba.Get[*utils.Config](ctx)
			host, _ := gaba.Get[romm.Host](ctx)

			config.DirectoryMappings = output.Mappings
			config.PlatformOrder = utils.PrunePlatformOrder(config.PlatformOrder, output.Mappings)
			utils.SaveConfig(config)
			gaba.Set(ctx, config)

			platforms, err := utils.GetMappedPlatforms(host, output.Mappings)
			if err != nil {
				gaba.GetLogger().Error("Failed to load platforms", "error", err)
				return err
			}
			gaba.Set(ctx, platforms)
			return nil
		}).
		On(gaba.ExitCodeBack, settings)

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
		OnWithHook(constants.ExitCodeLogout, platformSelection, func(ctx *gaba.Context) error {
			config, _ := gaba.Get[*utils.Config](ctx)
			config.Hosts = nil
			config.DirectoryMappings = nil
			config.PlatformOrder = nil

			if err := utils.SaveConfig(config); err != nil {
				gaba.GetLogger().Error("Failed to save config after logout", "error", err)
				return err
			}

			gaba.GetLogger().Info("User logged out successfully")
			return nil
		}).
		Exit(constants.ExitCodeLogout)

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

	return fsm.Start(platformSelection)
}
