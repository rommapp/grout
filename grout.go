package main

import (
	"grout/models"
	"grout/state"
	"grout/ui"
	"grout/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/UncleJunVIP/certifiable"
	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
)

func init() {
	gaba.Init(gaba.Options{
		WindowTitle:          "Grout",
		PrimaryThemeColorHex: 0x007C77,
		ShowBackground:       true,
		IsNextUI:             utils.GetCFW() == models.NEXTUI,
		LogFilename:          "grout.log",
	})

	if !utils.IsConnectedToInternet() {
		_, err := gaba.ConfirmationMessage("No Internet Connection!\nMake sure you are connected to Wi-Fi.", []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: "Quit"},
		}, gaba.MessageOptions{})
		defer cleanup()
		common.LogStandardFatal("No Internet Connection", err)
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
		config = ui.HandleLogin(models.Host{})
		utils.SaveConfig(config)
	}

	if config.LogLevel != "" {
		gaba.SetRawLogLevel(config.LogLevel)
	}

	if len(config.DirectoryMappings) == 0 {
		pms := ui.InitPlatformMappingScreen(config.Hosts[0], true)
		mappings, code, err := pms.Draw()
		if err != nil {
			return
		}

		if code == 0 {
			config.DirectoryMappings = mappings.(map[string]models.DirectoryMapping)
			utils.SaveConfig(config)
		}
	}

	config.Hosts[0].Platforms = utils.GetMappedPlatforms(config.Hosts[0], config.DirectoryMappings)

	gaba.GetLogger().Debug("Configuration Loaded!", "config", config.ToLoggable())

	state.SetConfig(config)
}

func cleanup() {
	gaba.Close()
}

func main() {
	defer cleanup()

	logger := gaba.GetLogger()
	appState := state.GetAppState()

	logger.Debug("Starting Grout")

	var screen models.Screen

	quitOnBack := len(appState.Config.Hosts) == 1

	if quitOnBack {
		screen = ui.InitPlatformSelection(appState.Config.Hosts[0], quitOnBack)
	} else {
		screen = ui.InitMainMenu(appState.Config.Hosts)
	}

	for {
		res, code, _ := screen.Draw()

		switch screen.Name() {
		case ui.Screens.MainMenu:
			switch code {
			case 0:
				host := res.(models.Host)
				screen = ui.InitPlatformSelection(host, quitOnBack)
			case 4:
				screen = ui.InitSettingsScreen()
			case 1, 2:
				os.Exit(0)
			}
		case ui.Screens.Settings:
			if code != 404 {
				if len(appState.Config.Hosts) == 1 {
					screen = ui.InitPlatformSelection(appState.Config.Hosts[0], quitOnBack)
				} else {
					screen = ui.InitMainMenu(appState.Config.Hosts)
				}
			}
		case ui.Screens.PlatformSelection:
			state.SetLastSelectedPosition(0, 0)
			switch code {
			case 0:
				platform := res.(models.Platform)
				screen = ui.InitGamesList(platform, shared.Items{}, "")
			case 1, 2:
				if quitOnBack {
					os.Exit(0)
				}
				screen = ui.InitMainMenu(appState.Config.Hosts)
			case 4:
				screen = ui.InitSettingsScreen()
			case 404:
				screen = ui.InitMainMenu(appState.Config.Hosts)
			case -1:
				screen = ui.InitMainMenu(appState.Config.Hosts)
			}
		case ui.Screens.GameList:
			gl := screen.(ui.GameList)

			switch code {
			case 0:
				games := res.(shared.Items)
				screen = ui.InitDownloadScreen(gl.Platform, gl.Games, games, gl.SearchFilter)
			case 2:
				if gl.SearchFilter != "" {
					screen = ui.InitGamesList(gl.Platform, state.GetAppState().CurrentFullGamesList, "")
				} else {
					screen = ui.InitPlatformSelection(gl.Platform.Host, quitOnBack)
				}

			case 4:
				screen = ui.InitSearch(gl.Platform, gl.SearchFilter)

			case 404:
				if gl.SearchFilter != "" {
					screen = ui.InitGamesList(gl.Platform, state.GetAppState().CurrentFullGamesList, "")
				} else {
					screen = ui.InitPlatformSelection(gl.Platform.Host, quitOnBack)
				}
			}
		case ui.Screens.SearchBox:
			sb := screen.(ui.Search)
			switch code {
			case 0:
				query := res.(string)
				state.SetLastSelectedPosition(0, 0)
				screen = ui.InitGamesList(sb.Platform, state.GetAppState().CurrentFullGamesList, query)
			default:
				screen = ui.InitGamesList(sb.Platform, state.GetAppState().CurrentFullGamesList, "")
			}
		case ui.Screens.Download:
			ds := screen.(ui.DownloadScreen)
			switch code {
			case 0:
				downloadedGames := res.([]shared.Item)

				for _, game := range downloadedGames {
					isMultiDisc := utils.IsMultiDisc(ds.Platform, game)

					if filepath.Ext(game.Filename) == ".zip" {
						isBinCue := utils.HasBinCue(ds.Platform, game)

						if isMultiDisc && appState.Config.GroupMultiDisc {
							utils.GroupMultiDisk(ds.Platform, game)
						} else if appState.Config.GroupBinCue && isBinCue {
							utils.GroupBinCue(ds.Platform, game)
						} else if appState.Config.UnzipDownloads {
							utils.UnzipGame(ds.Platform, game)
						}
					} else if appState.Config.GroupMultiDisc && isMultiDisc {
						utils.GroupMultiDisk(ds.Platform, game)
					}
				}

				if appState.Config.DownloadArt {
					seenBaseNames := make(map[string]bool)

					// Create a pruned list for art downloads that only includes one instance of each multi-disk game
					prunedGamesForArt := make([]shared.Item, 0, len(downloadedGames))

					for _, game := range downloadedGames {
						// Get base name by trimming at "(Disk" or "(Disc"
						baseName := game.DisplayName
						diskIndex := strings.Index(baseName, "(Disk")
						discIndex := strings.Index(baseName, "(Disc")

						trimIndex := -1
						if diskIndex != -1 && discIndex != -1 {
							trimIndex = min(diskIndex, discIndex)
						} else if diskIndex != -1 {
							trimIndex = diskIndex
						} else if discIndex != -1 {
							trimIndex = discIndex
						}

						if trimIndex != -1 {
							baseName = baseName[:trimIndex]
						}
						baseName = strings.TrimSpace(baseName)

						// If we haven't seen this base name before, add it to the pruned list
						if !seenBaseNames[baseName] {
							seenBaseNames[baseName] = true
							prunedGamesForArt = append(prunedGamesForArt, game)
						}
					}

					screen = ui.InitDownloadArtScreen(ds.Platform, prunedGamesForArt, ds.SearchFilter)
				} else {
					screen = ui.InitGamesList(ds.Platform, state.GetAppState().CurrentFullGamesList, ds.SearchFilter)
				}
			case 1:
				screen = ui.InitGamesList(ds.Platform, state.GetAppState().CurrentFullGamesList, ds.SearchFilter)
			default:
				screen = ui.InitGamesList(ds.Platform, state.GetAppState().CurrentFullGamesList, ds.SearchFilter)
			}
		case ui.Screens.DownloadArt:
			da := screen.(ui.DownloadArtScreen)
			screen = ui.InitGamesList(da.Platform, state.GetAppState().CurrentFullGamesList, da.SearchFilter)
		}
	}
}
