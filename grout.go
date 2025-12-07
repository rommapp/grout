package main

import (
	"grout/models"
	"grout/ui"
	"grout/utils"
	"log/slog"
	"time"

	_ "github.com/UncleJunVIP/certifiable"
	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/brandonkowalski/go-romm"
)

const (
	PlatformSelection       = "platform_selection"
	GameList                = "game_list"
	Search                  = "search"
	Settings                = "settings"
	SettingsPlatformMapping = "platform_mapping"
)

const (
	KeyConfig                       = "config"
	KeyCFW                          = "cfw"
	KeyHost                         = "host"
	KeyPlatforms                    = "platforms"
	KeyQuitOnBack                   = "quit_on_back"
	KeyCurrentPlatform              = "current_platform"
	KeyCurrentGames                 = "current_games"
	KeyFullGamesList                = "full_games_list"
	KeySearchFilter                 = "search_filter"
	KeySelectedGames                = "selected_games"
	KeyNewConfig                    = "new_config"
	KeyNewMappings                  = "new_mappings"
	KeySearchQuery                  = "search_query"
	KeyLastSelectedIndex            = "last_selected_index"
	KeyLastSelectedPosition         = "last_selected_position"
	KeyPlatformLastSelectedIndex    = "platform_last_selected_index"
	KeyPlatformLastSelectedPosition = "platform_last_selected_position"
)

var appConfig *models.Config

func init() {
	cfw := utils.GetCFW()

	gaba.Init(gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             cfw == models.NEXTUI,
		LogFilename:          "grout.log",
	})

	gaba.SetLogLevel(slog.LevelDebug)

	if !utils.IsConnectedToInternet() {
		_, _ = gaba.ConfirmationMessage("No Internet Connection!\nMake sure you are connected to Wi-Fi.", []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: "Quit"},
		}, gaba.MessageOptions{})
		defer cleanup()
		utils.LogStandardFatal("No Internet Connection", nil)
	}

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

	appConfig = config

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
			appConfig = config
		}
	}

	gaba.GetLogger().Debug("Configuration Loaded!", "config", config.ToLoggable())
}

func cleanup() {
	gaba.Close()
}

func main() {
	defer cleanup()

	logger := gaba.GetLogger()
	logger.Debug("Starting Grout")

	config := appConfig
	cfw := utils.GetCFW()
	quitOnBack := len(config.Hosts) == 1
	platforms := utils.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings)

	fsm := buildFSM(config, cfw, platforms, quitOnBack)

	if err := fsm.Run(); err != nil {
		logger.Error("FSM error", "error", err)
	}
}

func buildFSM(config *models.Config, cfw models.CFW, platforms []romm.Platform, quitOnBack bool) *gaba.FSM {
	builder := gaba.NewFSMBuilder()

	builder.
		WithData(KeyConfig, config).
		WithData(KeyCFW, cfw).
		WithData(KeyHost, config.Hosts[0]).
		WithData(KeyPlatforms, platforms).
		WithData(KeyQuitOnBack, quitOnBack).
		WithData(KeySearchFilter, "").
		StartWith(PlatformSelection)

	// Platform Selection
	gaba.RegisterScreenWithHandler(builder, PlatformSelection,
		ui.NewPlatformSelectionScreen(),
		func(ctx *gaba.FSMContext) ui.PlatformSelectionInput {
			platforms, _ := gaba.Get[[]romm.Platform](ctx, KeyPlatforms)
			quitOnBack, _ := gaba.Get[bool](ctx, KeyQuitOnBack)
			return ui.PlatformSelectionInput{Platforms: platforms, QuitOnBack: quitOnBack}
		},
		func(ctx *gaba.FSMContext, output ui.PlatformSelectionOutput) {
			ctx.Set(KeyCurrentPlatform, output.SelectedPlatform)
		})

	// Game List
	gaba.RegisterScreenWithHandler(builder, GameList,
		ui.NewGameListScreen(),
		func(ctx *gaba.FSMContext) ui.GameListInput {
			config, _ := gaba.Get[*models.Config](ctx, KeyConfig)
			host, _ := gaba.Get[models.Host](ctx, KeyHost)
			platform, _ := gaba.Get[romm.Platform](ctx, KeyCurrentPlatform)
			games, _ := gaba.Get[[]romm.DetailedRom](ctx, KeyCurrentGames)
			filter, _ := gaba.Get[string](ctx, KeySearchFilter)
			lastIdx, _ := gaba.Get[int](ctx, KeyLastSelectedIndex)
			lastPos, _ := gaba.Get[int](ctx, KeyLastSelectedPosition)
			return ui.GameListInput{
				Config:               config,
				Host:                 host,
				Platform:             platform,
				Games:                games,
				SearchFilter:         filter,
				LastSelectedIndex:    lastIdx,
				LastSelectedPosition: lastPos,
			}
		},
		func(ctx *gaba.FSMContext, output ui.GameListOutput) {
			ctx.Set(KeySelectedGames, output.SelectedGames)
			ctx.Set(KeyFullGamesList, output.AllGames)
			ctx.Set(KeySearchFilter, output.SearchFilter)
			ctx.Set(KeyLastSelectedIndex, output.LastSelectedIndex)
			ctx.Set(KeyLastSelectedPosition, output.LastSelectedPosition)
		})

	// Search
	gaba.RegisterScreenWithHandler(builder, Search,
		ui.NewSearchScreen(),
		func(ctx *gaba.FSMContext) ui.SearchInput {
			filter, _ := gaba.Get[string](ctx, KeySearchFilter)
			return ui.SearchInput{InitialText: filter}
		},
		func(ctx *gaba.FSMContext, output ui.SearchOutput) {
			ctx.Set(KeySearchQuery, output.Query)
		})

	// Settings
	gaba.RegisterScreenWithHandler(builder, Settings,
		ui.NewSettingsScreen(),
		func(ctx *gaba.FSMContext) ui.SettingsInput {
			config, _ := gaba.Get[*models.Config](ctx, KeyConfig)
			cfw, _ := gaba.Get[models.CFW](ctx, KeyCFW)
			host, _ := gaba.Get[models.Host](ctx, KeyHost)
			return ui.SettingsInput{Config: config, CFW: cfw, Host: host}
		},
		func(ctx *gaba.FSMContext, output ui.SettingsOutput) {
			ctx.Set(KeyNewConfig, output.Config)
		})

	// Platform Mapping
	gaba.RegisterScreenWithHandler(builder, SettingsPlatformMapping,
		ui.NewPlatformMappingScreen(),
		func(ctx *gaba.FSMContext) ui.PlatformMappingInput {
			host, _ := gaba.Get[models.Host](ctx, KeyHost)
			config, _ := gaba.Get[*models.Config](ctx, KeyConfig)
			cfw, _ := gaba.Get[models.CFW](ctx, KeyCFW)
			return ui.PlatformMappingInput{
				Host: host, ApiTimeout: config.ApiTimeout, CFW: cfw,
				RomDirectory: utils.GetRomDirectory(), AutoSelect: false, HideBackButton: false,
			}
		},
		func(ctx *gaba.FSMContext, output ui.PlatformMappingOutput) {
			ctx.Set(KeyNewMappings, output.Mappings)
		})

	gaba.RegisterScreenWithHandler(builder, PlatformSelection,
		ui.NewPlatformSelectionScreen(),
		func(ctx *gaba.FSMContext) ui.PlatformSelectionInput {
			platforms, _ := gaba.Get[[]romm.Platform](ctx, KeyPlatforms)
			quitOnBack, _ := gaba.Get[bool](ctx, KeyQuitOnBack)
			lastIdx, _ := gaba.Get[int](ctx, KeyPlatformLastSelectedIndex)
			lastPos, _ := gaba.Get[int](ctx, KeyPlatformLastSelectedPosition)
			return ui.PlatformSelectionInput{
				Platforms:            platforms,
				QuitOnBack:           quitOnBack,
				LastSelectedIndex:    lastIdx,
				LastSelectedPosition: lastPos,
			}
		},
		func(ctx *gaba.FSMContext, output ui.PlatformSelectionOutput) {
			ctx.Set(KeyCurrentPlatform, output.SelectedPlatform)
			ctx.Set(KeyPlatformLastSelectedIndex, output.LastSelectedIndex)
			ctx.Set(KeyPlatformLastSelectedPosition, output.LastSelectedPosition)
		})

	builder.
		On(PlatformSelection, gaba.ExitCodeSuccess).
		Before(func(ctx *gaba.FSMContext) error {
			ctx.Set(KeySearchFilter, "")
			ctx.Set(KeyCurrentGames, []romm.DetailedRom(nil))
			ctx.Set(KeyLastSelectedIndex, 0)
			ctx.Set(KeyLastSelectedPosition, 0)
			return nil
		}).GoTo(GameList)

	builder.
		On(PlatformSelection, gaba.ExitCodeSettings).GoTo(Settings)

	builder.
		On(PlatformSelection, gaba.ExitCodeBack).Exit()

	builder.
		On(GameList, gaba.ExitCodeSuccess).
		Before(func(ctx *gaba.FSMContext) error {
			selectedGames, _ := gaba.Get[[]romm.DetailedRom](ctx, KeySelectedGames)
			config, _ := gaba.Get[models.Config](ctx, KeyConfig)
			host, _ := gaba.Get[models.Host](ctx, KeyHost)
			platform, _ := gaba.Get[romm.Platform](ctx, KeyCurrentPlatform)
			fullGames, _ := gaba.Get[[]romm.DetailedRom](ctx, KeyFullGamesList)
			searchFilter, _ := gaba.Get[string](ctx, KeySearchFilter)
			output := downloadGames(config, host, platform, selectedGames, fullGames, searchFilter)
			ctx.Set(KeyCurrentGames, output.AllGames)
			ctx.Set(KeySearchFilter, output.SearchFilter)
			return nil
		}).GoTo(GameList).
		On(GameList, gaba.ExitCodeSearch).GoTo(Search).
		On(GameList, utils.ExitCodeClearSearch).
		Before(func(ctx *gaba.FSMContext) error {
			ctx.Set(KeySearchFilter, "")
			fullGames, _ := gaba.Get[[]romm.DetailedRom](ctx, KeyFullGamesList)
			ctx.Set(KeyCurrentGames, fullGames)
			ctx.Set(KeyLastSelectedIndex, 0)
			ctx.Set(KeyLastSelectedPosition, 0)
			return nil
		}).GoTo(GameList).
		On(GameList, gaba.ExitCodeBack).
		Before(func(ctx *gaba.FSMContext) error {
			ctx.Set(KeyCurrentGames, []romm.DetailedRom(nil))
			return nil
		}).GoTo(PlatformSelection).
		On(GameList, utils.ExitCodeNoResults).GoTo(Search)

	builder.
		On(Search, gaba.ExitCodeSuccess).
		Before(func(ctx *gaba.FSMContext) error {
			query, _ := gaba.Get[string](ctx, KeySearchQuery)
			ctx.Set(KeySearchFilter, query)
			fullGames, _ := gaba.Get[[]romm.DetailedRom](ctx, KeyFullGamesList)
			ctx.Set(KeyCurrentGames, fullGames)
			ctx.Set(KeyLastSelectedIndex, 0)
			ctx.Set(KeyLastSelectedPosition, 0)
			return nil
		}).GoTo(GameList).
		On(Search, gaba.ExitCodeBack).
		Before(func(ctx *gaba.FSMContext) error {
			ctx.Set(KeySearchFilter, "")
			fullGames, _ := gaba.Get[[]romm.DetailedRom](ctx, KeyFullGamesList)
			ctx.Set(KeyCurrentGames, fullGames)
			return nil
		}).
		GoTo(GameList)

	builder.
		On(Settings, gaba.ExitCodeSuccess).
		Before(func(ctx *gaba.FSMContext) error {
			newConfig, _ := gaba.Get[*models.Config](ctx, KeyNewConfig)
			utils.SaveConfig(newConfig)
			ctx.Set(KeyConfig, newConfig)
			return nil
		}).GoTo(PlatformSelection).
		On(Settings, utils.ExitCodeEditMappings).GoTo(SettingsPlatformMapping).
		On(Settings, gaba.ExitCodeBack).GoTo(PlatformSelection)

	builder.
		On(SettingsPlatformMapping, gaba.ExitCodeSuccess).
		Before(func(ctx *gaba.FSMContext) error {
			mappings, _ := gaba.Get[map[string]models.DirectoryMapping](ctx, KeyNewMappings)
			config, _ := gaba.Get[*models.Config](ctx, KeyConfig)
			host, _ := gaba.Get[models.Host](ctx, KeyHost)
			config.DirectoryMappings = mappings
			utils.SaveConfig(config)
			ctx.Set(KeyConfig, config)
			ctx.Set(KeyPlatforms, utils.GetMappedPlatforms(host, mappings))
			return nil
		}).GoTo(Settings).
		On(SettingsPlatformMapping, gaba.ExitCodeBack).GoTo(Settings)

	return builder.Build()
}

func downloadGames(config models.Config, host models.Host, platform romm.Platform, selectedGames []romm.DetailedRom, allGames []romm.DetailedRom, searchFilter string) ui.DownloadOutput {
	downloadScreen := ui.NewDownloadScreen()

	result, err := downloadScreen.Draw(ui.DownloadInput{
		Config:        config,
		Host:          host,
		Platform:      platform,
		SelectedGames: selectedGames,
		AllGames:      allGames,
		SearchFilter:  searchFilter,
	})

	if err != nil {
		gaba.GetLogger().Error("Download failed", "error", err)
		return ui.DownloadOutput{
			AllGames:     allGames,
			Platform:     platform,
			SearchFilter: searchFilter,
		}
	}

	if result.ExitCode == gaba.ExitCodeSuccess && len(result.Value.DownloadedGames) > 0 {
		gaba.GetLogger().Info("Successfully downloaded games", "count", len(result.Value.DownloadedGames))
	}

	return result.Value
}
