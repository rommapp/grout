package ui

import (
	"fmt"
	"grout/models"
	"grout/state"
	"grout/utils"

	"github.com/UncleJunVIP/gabagool/pkg/gabagool"
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
	logger := gabagool.GetLogger()

	appState := state.GetAppState()

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{
				Text: "Download Art",
			},
			Options: []gabagool.Option{
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

		// TODO add download timeout

		{
			Item: gabagool.MenuItem{
				Text: "Use Title As Filename",
			},
			Options: []gabagool.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: func() int {
				if appState.Config.UseTitleAsFilename {
					return 0
				}
				return 1
			}(),
		},

		{
			Item: gabagool.MenuItem{
				Text: "Unzip Downloads",
			},
			Options: []gabagool.Option{
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
			Item: gabagool.MenuItem{
				Text: "Group BIN / CUE",
			},
			Options: []gabagool.Option{
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
			Item: gabagool.MenuItem{
				Text: "Group Multi-Disc",
			},
			Options: []gabagool.Option{
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
			Item: gabagool.MenuItem{
				Text: "Log Level",
			},
			Options: []gabagool.Option{
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

	result, err := gabagool.OptionsList(
		"Grout Settings",
		gabagool.OptionListSettings{FooterHelpItems: []gabagool.FooterHelpItem{
			{ButtonName: "B", HelpText: "Cancel"},
			{ButtonName: "←→", HelpText: "Cycle"},
			{ButtonName: "Start", HelpText: "Save"},
		}},
		items,
	)

	if err != nil {
		fmt.Println("Error showing options list:", err)
		return
	}

	if result.IsSome() {
		newSettingOptions := result.Unwrap().Items

		for _, option := range newSettingOptions {
			if option.Item.Text == "Download Art" {
				appState.Config.DownloadArt = option.SelectedOption == 0
			} else if option.Item.Text == "Use Title As Filename" {
				appState.Config.UseTitleAsFilename = option.SelectedOption == 0
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
