package main

import (
	"grout/constants"
	"grout/models"
	"grout/ui"
	"grout/utils"
	"log/slog"
	"time"

	"grout/romm"

	_ "github.com/UncleJunVIP/certifiable"
	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

const (
	PlatformSelection           = "platform_selection"
	GameList                    = "game_list"
	GameDetails                 = "game_details"
	CollectionList              = "collection_list"
	CollectionPlatformSelection = "collection_platform_selection"
	Search                      = "search"
	Settings                    = "settings"
	SettingsPlatformMapping     = "platform_mapping"
)

type (
	CurrentGamesList   []romm.Rom
	FullGamesList      []romm.Rom
	SearchFilterString string
	QuitOnBackBool     bool

	ShowCollectionsBool bool

	GameListPosition struct {
		Index int
		Pos   int
	}

	PlatformListPosition struct {
		Index int
		Pos   int
	}

	CollectionListPosition struct {
		Index int
		Pos   int
	}

	CollectionPlatformListPosition struct {
		Index int
		Pos   int
	}

	CachedCollectionGames []romm.Rom
)

func setup() *models.Config {
	cfw := utils.GetCFW()

	gaba.Init(gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             cfw == constants.NextUI,
		LogFilename:          "grout.log",
	})

	gaba.SetLogLevel(slog.LevelDebug)

	gaba.ProcessMessage("", gaba.ProcessMessageOptions{
		Image:       "resources/splash.png",
		ImageWidth:  gaba.GetWindow().GetWidth(),
		ImageHeight: gaba.GetWindow().GetHeight(),
	}, func() (interface{}, error) {
		time.Sleep(750 * time.Millisecond)
		return nil, nil
	})

	config, err := utils.LoadConfig()
	if err != nil {
		gaba.GetLogger().Debug("No RomM Host Configured")
		loginConfig, loginErr := ui.LoginFlow(models.Host{})
		if loginErr != nil {
			utils.LogStandardFatal("Login failed", loginErr)
		}
		config = loginConfig
		utils.SaveConfig(config)
	}

	if config.LogLevel != "" {
		gaba.SetRawLogLevel(config.LogLevel)
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
	gaba.Close()
}

func main() {
	defer cleanup()

	config := setup()

	logger := gaba.GetLogger()
	logger.Debug("Starting Grout")

	cfw := utils.GetCFW()
	quitOnBack := len(config.Hosts) == 1
	platforms := utils.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings)
	showCollections := utils.ShowCollections(config.Hosts[0])

	fsm := buildFSM(config, cfw, platforms, quitOnBack, showCollections)

	if err := fsm.Run(); err != nil {
		logger.Error("FSM error", "error", err)
	}
}

func buildFSM(config *models.Config, cfw constants.CFW, platforms []romm.Platform, quitOnBack bool, showCollections bool) *gaba.FSM {
	fsm := gaba.NewFSM()

	gaba.Set(fsm.Context(), config)
	gaba.Set(fsm.Context(), cfw)
	gaba.Set(fsm.Context(), config.Hosts[0])
	gaba.Set(fsm.Context(), platforms)
	gaba.Set(fsm.Context(), QuitOnBackBool(quitOnBack))
	gaba.Set(fsm.Context(), ShowCollectionsBool(showCollections))
	gaba.Set(fsm.Context(), SearchFilterString(""))

	gaba.AddState(fsm, PlatformSelection, func(ctx *gaba.Context) (ui.PlatformSelectionOutput, gaba.ExitCode) {
		platforms, _ := gaba.Get[[]romm.Platform](ctx)
		quitOnBack, _ := gaba.Get[QuitOnBackBool](ctx)
		showCollections, _ := gaba.Get[ShowCollectionsBool](ctx)
		platPos, _ := gaba.Get[PlatformListPosition](ctx)

		screen := ui.NewPlatformSelectionScreen()
		result, err := screen.Draw(ui.PlatformSelectionInput{
			Platforms:            platforms,
			QuitOnBack:           bool(quitOnBack),
			ShowCollections:      bool(showCollections),
			LastSelectedIndex:    platPos.Index,
			LastSelectedPosition: platPos.Pos,
		})

		if err != nil {
			return ui.PlatformSelectionOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, PlatformListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, GameList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, SearchFilterString(""))
			gaba.Set(ctx, CurrentGamesList(nil))
			gaba.Set(ctx, GameListPosition{Index: 0, Pos: 0})
			gaba.Set(ctx, ui.CollectionSelectionOutput{})
			return nil
		}).
		OnWithHook(constants.ExitCodeCollections, CollectionList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, CollectionListPosition{
				Index: 0,
				Pos:   0,
			})
			return nil
		}).
		On(gaba.ExitCodeAction, Settings).
		Exit(gaba.ExitCodeQuit)

	gaba.AddState(fsm, CollectionList, func(ctx *gaba.Context) (ui.CollectionSelectionOutput, gaba.ExitCode) {
		host, _ := gaba.Get[models.Host](ctx)
		colPos, _ := gaba.Get[CollectionListPosition](ctx)

		screen := ui.NewCollectionSelectionScreen()
		result, err := screen.Draw(ui.CollectionSelectionInput{
			Host:                 host,
			LastSelectedIndex:    colPos.Index,
			LastSelectedPosition: colPos.Pos,
		})

		if err != nil {
			return ui.CollectionSelectionOutput{}, gaba.ExitCodeError
		}

		gaba.Set(ctx, CollectionListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, CollectionPlatformSelection, func(ctx *gaba.Context) error {
			gaba.Set(ctx, SearchFilterString(""))
			gaba.Set(ctx, CurrentGamesList(nil))
			gaba.Set(ctx, GameListPosition{Index: 0, Pos: 0})
			gaba.Set(ctx, CollectionPlatformListPosition{Index: 0, Pos: 0})
			gaba.Set(ctx, CachedCollectionGames(nil))
			gaba.Set(ctx, ui.PlatformSelectionOutput{})
			return nil
		}).
		On(gaba.ExitCodeBack, PlatformSelection)

	gaba.AddState(fsm, CollectionPlatformSelection, func(ctx *gaba.Context) (ui.CollectionPlatformSelectionOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*models.Config](ctx)
		host, _ := gaba.Get[models.Host](ctx)
		collection, _ := gaba.Get[ui.CollectionSelectionOutput](ctx)
		pos, _ := gaba.Get[CollectionPlatformListPosition](ctx)
		cachedGames, _ := gaba.Get[CachedCollectionGames](ctx)

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

		gaba.Set(ctx, CollectionPlatformListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})

		gaba.Set(ctx, CachedCollectionGames(result.Value.AllGames))

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, GameList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)

			filteredGames := make([]romm.Rom, 0)
			for _, game := range output.AllGames {
				if game.PlatformID == output.SelectedPlatform.ID {
					filteredGames = append(filteredGames, game)
				}
			}

			gaba.Set(ctx, SearchFilterString(""))
			gaba.Set(ctx, FullGamesList(filteredGames))
			gaba.Set(ctx, CurrentGamesList(filteredGames))
			gaba.Set(ctx, GameListPosition{Index: 0, Pos: 0})
			return nil
		}).
		On(gaba.ExitCodeBack, CollectionList)

	gaba.AddState(fsm, GameList, func(ctx *gaba.Context) (ui.GameListOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*models.Config](ctx)
		host, _ := gaba.Get[models.Host](ctx)
		platform, _ := gaba.Get[ui.PlatformSelectionOutput](ctx)
		collection, _ := gaba.Get[ui.CollectionSelectionOutput](ctx)
		collectionPlatform, _ := gaba.Get[ui.CollectionPlatformSelectionOutput](ctx)
		games, _ := gaba.Get[CurrentGamesList](ctx)
		filter, _ := gaba.Get[SearchFilterString](ctx)
		pos, _ := gaba.Get[GameListPosition](ctx)

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

		gaba.Set(ctx, FullGamesList(result.Value.AllGames))
		gaba.Set(ctx, CurrentGamesList(result.Value.AllGames))
		gaba.Set(ctx, GameListPosition{
			Index: result.Value.LastSelectedIndex,
			Pos:   result.Value.LastSelectedPosition,
		})
		gaba.Set(ctx, SearchFilterString(result.Value.SearchFilter))

		return result.Value, result.ExitCode
	}).
		On(gaba.ExitCodeSuccess, GameDetails).
		On(constants.ExitCodeSearch, Search).
		OnWithHook(constants.ExitCodeClearSearch, GameList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, SearchFilterString(""))
			fullGames, _ := gaba.Get[FullGamesList](ctx)
			gaba.Set(ctx, CurrentGamesList(fullGames))
			gaba.Set(ctx, GameListPosition{Index: 0, Pos: 0})
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, PlatformSelection, func(ctx *gaba.Context) error {
			gaba.Set(ctx, CurrentGamesList(nil))
			return nil
		}).
		On(constants.ExitCodeBackToCollectionPlatform, CollectionPlatformSelection).
		OnWithHook(constants.ExitCodeBackToCollection, CollectionList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, CurrentGamesList(nil))
			return nil
		}).
		On(constants.ExitCodeNoResults, Search)

	gaba.AddState(fsm, GameDetails, func(ctx *gaba.Context) (ui.GameDetailsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*models.Config](ctx)
		host, _ := gaba.Get[models.Host](ctx)
		gameListOutput, _ := gaba.Get[ui.GameListOutput](ctx)

		if !config.ShowGameDetails || len(gameListOutput.SelectedGames) != 1 {
			filter, _ := gaba.Get[SearchFilterString](ctx)
			downloadScreen := ui.NewDownloadScreen()
			downloadOutput := downloadScreen.Execute(*config, host, gameListOutput.Platform, gameListOutput.SelectedGames, gameListOutput.AllGames, string(filter))
			gaba.Set(ctx, CurrentGamesList(downloadOutput.AllGames))
			gaba.Set(ctx, SearchFilterString(downloadOutput.SearchFilter))
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
		OnWithHook(gaba.ExitCodeSuccess, GameList, func(ctx *gaba.Context) error {
			detailsOutput, _ := gaba.Get[ui.GameDetailsOutput](ctx)
			config, _ := gaba.Get[*models.Config](ctx)
			host, _ := gaba.Get[models.Host](ctx)
			gameListOutput, _ := gaba.Get[ui.GameListOutput](ctx)
			filter, _ := gaba.Get[SearchFilterString](ctx)

			if detailsOutput.DownloadRequested {
				downloadScreen := ui.NewDownloadScreen()
				downloadOutput := downloadScreen.Execute(*config, host, detailsOutput.Platform, []romm.Rom{detailsOutput.Game}, gameListOutput.AllGames, string(filter))
				gaba.Set(ctx, CurrentGamesList(downloadOutput.AllGames))
				gaba.Set(ctx, SearchFilterString(downloadOutput.SearchFilter))
			}

			return nil
		}).
		On(gaba.ExitCodeBack, GameList)

	gaba.AddState(fsm, Search, func(ctx *gaba.Context) (ui.SearchOutput, gaba.ExitCode) {
		filter, _ := gaba.Get[SearchFilterString](ctx)

		screen := ui.NewSearchScreen()
		result, err := screen.Draw(ui.SearchInput{
			InitialText: string(filter),
		})

		if err != nil {
			return ui.SearchOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, GameList, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SearchOutput](ctx)
			gaba.Set(ctx, SearchFilterString(output.Query))
			fullGames, _ := gaba.Get[FullGamesList](ctx)
			gaba.Set(ctx, CurrentGamesList(fullGames))
			gaba.Set(ctx, GameListPosition{Index: 0, Pos: 0})
			return nil
		}).
		OnWithHook(gaba.ExitCodeBack, GameList, func(ctx *gaba.Context) error {
			gaba.Set(ctx, SearchFilterString(""))
			fullGames, _ := gaba.Get[FullGamesList](ctx)
			gaba.Set(ctx, CurrentGamesList(fullGames))
			return nil
		})

	gaba.AddState(fsm, Settings, func(ctx *gaba.Context) (ui.SettingsOutput, gaba.ExitCode) {
		config, _ := gaba.Get[*models.Config](ctx)
		cfw, _ := gaba.Get[constants.CFW](ctx)
		host, _ := gaba.Get[models.Host](ctx)

		screen := ui.NewSettingsScreen()
		result, err := screen.Draw(ui.SettingsInput{
			Config: config,
			CFW:    cfw,
			Host:   host,
		})

		if err != nil {
			return ui.SettingsOutput{}, gaba.ExitCodeError
		}

		return result.Value, result.ExitCode
	}).
		OnWithHook(gaba.ExitCodeSuccess, PlatformSelection, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.SettingsOutput](ctx)
			utils.SaveConfig(output.Config)
			gaba.Set(ctx, output.Config)
			return nil
		}).
		On(constants.ExitCodeEditMappings, SettingsPlatformMapping).
		On(gaba.ExitCodeBack, PlatformSelection)

	gaba.AddState(fsm, SettingsPlatformMapping, func(ctx *gaba.Context) (ui.PlatformMappingOutput, gaba.ExitCode) {
		host, _ := gaba.Get[models.Host](ctx)
		config, _ := gaba.Get[*models.Config](ctx)
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
		OnWithHook(gaba.ExitCodeSuccess, Settings, func(ctx *gaba.Context) error {
			output, _ := gaba.Get[ui.PlatformMappingOutput](ctx)
			config, _ := gaba.Get[*models.Config](ctx)
			host, _ := gaba.Get[models.Host](ctx)

			config.DirectoryMappings = output.Mappings
			utils.SaveConfig(config)
			gaba.Set(ctx, config)
			gaba.Set(ctx, utils.GetMappedPlatforms(host, output.Mappings))
			return nil
		}).
		On(gaba.ExitCodeBack, Settings)

	return fsm.Start(PlatformSelection)
}
