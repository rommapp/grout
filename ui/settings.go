package ui

import (
	"fmt"
	"grout/models"
	"grout/state"
	"grout/utils"
	"grout/web"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	shared "github.com/UncleJunVIP/nextui-pak-shared-functions/models"
	"qlova.tech/sum"
)

type SettingsScreen struct {
}

func InitSettingsScreen() SettingsScreen {
	return SettingsScreen{}
}

func (s SettingsScreen) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.Settings
}

func (s SettingsScreen) Draw() (settings interface{}, exitCode int, e error) {
	logger := gaba.GetLoggerInstance()

	appState := state.GetAppState()

	items := []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{
				Text: "Download Art",
			},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: func() int {
				if appState.Config.DownloadArt {
					return 0
				}
				return 1
			}(),
		},
		{
			Item: gaba.MenuItem{
				Text: "Art Type",
			},
			Options: []gaba.Option{
				{DisplayName: "Box Art", Value: "BOX_ART"},
				{DisplayName: "Title Screen", Value: "TITLE_SCREEN"},
				{DisplayName: "Logos", Value: "LOGOS"},
				{DisplayName: "Screenshots", Value: "SCREENSHOTS"},
			},
			SelectedOption: func() int {
				switch appState.Config.ArtDownloadType {
				case shared.ArtDownloadTypes.BOX_ART:
					return 0
				case shared.ArtDownloadTypes.TITLE_SCREEN:
					return 1
				case shared.ArtDownloadTypes.LOGOS:
					return 2
				case shared.ArtDownloadTypes.SCREENSHOTS:
					return 3
				default:
					return 0
				}
			}(),
		},
		{
			Item: gaba.MenuItem{
				Text: "Unzip Downloads",
			},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: func() int {
				if appState.Config.UnzipDownloads {
					return 0
				}
				return 1
			}(),
		},
		{
			Item: gaba.MenuItem{
				Text: "Group BIN / CUE",
			},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: func() int {
				if appState.Config.GroupBinCue {
					return 0
				}
				return 1
			}(),
		},
		{
			Item: gaba.MenuItem{
				Text: "Group Multi-Disc",
			},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: func() int {
				if appState.Config.GroupMultiDisc {
					return 0
				}
				return 1
			}(),
		},
		{
			Item: gaba.MenuItem{
				Text: "Log Level",
			},
			Options: []gaba.Option{
				{DisplayName: "Debug", Value: "DEBUG"},
				{DisplayName: "Error", Value: "ERROR"},
			},
			SelectedOption: func() int {
				switch appState.Config.LogLevel {
				case "DEBUG":
					return 0
				case "ERROR":
					return 1
				}
				return 0
			}(),
		},
	}

	if utils.CacheFolderExists() {
		items = append(items, gaba.ItemWithOptions{
			Item: gaba.MenuItem{
				Text: "Empty Cache",
			},
			Options: []gaba.Option{
				{
					DisplayName: "",
					Value:       "empty",
					Type:        gaba.OptionTypeClickable,
				},
			},
		})
	}

	items = append(items, gaba.ItemWithOptions{
		Item: gaba.MenuItem{
			Text: "Launch Configuration API",
		},
		Options: []gaba.Option{
			{
				Type: gaba.OptionTypeClickable,
			},
		},
	})

	footerHelpItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: "Cancel"},
		{ButtonName: "←→", HelpText: "Cycle"},
		{ButtonName: "Start", HelpText: "Save"},
	}

	result, err := gaba.OptionsList(
		"Grout Settings",
		items,
		footerHelpItems,
	)

	if err != nil {
		fmt.Println("Error showing options list:", err)
		return
	}

	if result.IsSome() {
		if result.Unwrap().SelectedItem.Item.Text == "Launch Configuration API" {
			web.QRScreen("Shutdown Configuration API")
			config, err := utils.LoadConfig()
			if err == nil {
				state.SetConfig(config)
				utils.DeleteCache()
			}
			return result, 404, nil
		}

		if result.Unwrap().SelectedItem.Item.Text == "Empty Cache" {
			_ = utils.DeleteCache()

			_, _ = gaba.ProcessMessage(fmt.Sprintf("Cache Emptied!"),
				gaba.ProcessMessageOptions{ShowThemeBackground: true}, func() (interface{}, error) {
					return nil, nil
				})
			return result, 404, nil
		}

		newSettingOptions := result.Unwrap().Items

		for _, option := range newSettingOptions {
			if option.Item.Text == "Download Art" {
				appState.Config.DownloadArt = option.SelectedOption == 0
			} else if option.Item.Text == "Art Type" {
				artTypeValue := option.Options[option.SelectedOption].Value.(string)
				switch artTypeValue {
				case "BOX_ART":
					appState.Config.ArtDownloadType = shared.ArtDownloadTypes.BOX_ART
				case "TITLE_SCREEN":
					appState.Config.ArtDownloadType = shared.ArtDownloadTypes.TITLE_SCREEN
				case "LOGOS":
					appState.Config.ArtDownloadType = shared.ArtDownloadTypes.LOGOS
				case "SCREENSHOTS":
					appState.Config.ArtDownloadType = shared.ArtDownloadTypes.SCREENSHOTS
				}
			} else if option.Item.Text == "Unzip Downloads" {
				appState.Config.UnzipDownloads = option.SelectedOption == 0
			} else if option.Item.Text == "Group BIN / CUE" {
				appState.Config.GroupBinCue = option.SelectedOption == 0
			} else if option.Item.Text == "Group Multi-Disc" {
				appState.Config.GroupMultiDisc = option.SelectedOption == 0
			} else if option.Item.Text == "Log Level" {
				logLevelValue := option.Options[option.SelectedOption].Value.(string)
				appState.Config.LogLevel = logLevelValue
			}
		}

		err := utils.SaveConfig(appState.Config)
		if err != nil {
			logger.Error("Error saving config", "error", err)
			return nil, 0, err
		}

		state.UpdateAppState(appState)

		return result, 0, nil
	}

	return nil, 2, nil
}
