package ui

import (
	"errors"
	"grout/constants"
	"grout/models"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

type SettingsInput struct {
	Config *models.Config
	CFW    constants.CFW
	Host   models.Host
}

type SettingsOutput struct {
	Config              *models.Config
	EditMappingsClicked bool
}

type SettingsScreen struct{}

func NewSettingsScreen() *SettingsScreen {
	return &SettingsScreen{}
}

var (
	apiTimeoutOptions = []struct {
		Display string
		Value   time.Duration
	}{
		{"15 Seconds", 15 * time.Second},
		{"30 Seconds", 30 * time.Second},
		{"45 Seconds", 45 * time.Second},
		{"60 Seconds", 60 * time.Second},
		{"75 Seconds", 75 * time.Second},
		{"90 Seconds", 90 * time.Second},
		{"120 Seconds", 120 * time.Second},
		{"180 Seconds", 180 * time.Second},
		{"240 Seconds", 240 * time.Second},
		{"300 Seconds", 300 * time.Second},
	}

	downloadTimeoutOptions = []struct {
		Display string
		Value   time.Duration
	}{
		{"15 Minutes", 15 * time.Minute},
		{"30 Minutes", 30 * time.Minute},
		{"45 Minutes", 45 * time.Minute},
		{"60 Minutes", 60 * time.Minute},
		{"75 Minutes", 75 * time.Minute},
		{"90 Minutes", 90 * time.Minute},
		{"105 Minutes", 105 * time.Minute},
		{"120 Minutes", 120 * time.Minute},
	}
)

func (s *SettingsScreen) Draw(input SettingsInput) (ScreenResult[SettingsOutput], error) {
	config := input.Config
	output := SettingsOutput{Config: config}

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		"Grout Settings",
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{
				{ButtonName: "B", HelpText: "Cancel"},
				{ButtonName: "←→", HelpText: "Cycle"},
				{ButtonName: "Start", HelpText: "Save"},
			},
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return Back(SettingsOutput{}), nil
		}
		return WithCode(SettingsOutput{}, gaba.ExitCodeError), err
	}

	if result.Selected == 0 {
		output.EditMappingsClicked = true
		return WithCode(output, constants.ExitCodeEditMappings), nil
	}

	s.applySettings(config, result.Items, input.CFW)

	output.Config = config
	return Success(output), nil
}

func (s *SettingsScreen) buildMenuItems(config *models.Config) []gaba.ItemWithOptions {
	return []gaba.ItemWithOptions{
		{
			Item:    gaba.MenuItem{Text: "Edit Directory Mappings"},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item: gaba.MenuItem{Text: "Download Art"},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: boolToIndex(!config.DownloadArt),
		},
		{
			Item: gaba.MenuItem{Text: "Unzip Downloads"},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: boolToIndex(!config.UnzipDownloads),
		},
		{
			Item: gaba.MenuItem{Text: "Show Game Details"},
			Options: []gaba.Option{
				{DisplayName: "True", Value: true},
				{DisplayName: "False", Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowGameDetails),
		},
		{
			Item:           gaba.MenuItem{Text: "API Timeout"},
			Options:        s.buildApiTimeoutOptions(),
			SelectedOption: s.findApiTimeoutIndex(config.ApiTimeout),
		},
		{
			Item:           gaba.MenuItem{Text: "Download Timeout"},
			Options:        s.buildDownloadTimeoutOptions(),
			SelectedOption: s.findDownloadTimeoutIndex(config.DownloadTimeout),
		},
		{
			Item: gaba.MenuItem{Text: "Log Level"},
			Options: []gaba.Option{
				{DisplayName: "Debug", Value: "DEBUG"},
				{DisplayName: "Error", Value: "ERROR"},
			},
			SelectedOption: logLevelToIndex(config.LogLevel),
		},
	}
}

func (s *SettingsScreen) buildApiTimeoutOptions() []gaba.Option {
	options := make([]gaba.Option, len(apiTimeoutOptions))
	for i, opt := range apiTimeoutOptions {
		options[i] = gaba.Option{DisplayName: opt.Display, Value: opt.Value}
	}
	return options
}

func (s *SettingsScreen) buildDownloadTimeoutOptions() []gaba.Option {
	options := make([]gaba.Option, len(downloadTimeoutOptions))
	for i, opt := range downloadTimeoutOptions {
		options[i] = gaba.Option{DisplayName: opt.Display, Value: opt.Value}
	}
	return options
}

func (s *SettingsScreen) findApiTimeoutIndex(timeout time.Duration) int {
	for i, opt := range apiTimeoutOptions {
		if opt.Value == timeout {
			return i
		}
	}
	return 0
}

func (s *SettingsScreen) findDownloadTimeoutIndex(timeout time.Duration) int {
	for i, opt := range downloadTimeoutOptions {
		if opt.Value == timeout {
			return i
		}
	}
	return 0
}

func (s *SettingsScreen) applySettings(config *models.Config, items []gaba.ItemWithOptions, cfw constants.CFW) {
	// Adjust index offset based on whether MuOS removed the art option
	offset := 0
	if cfw == constants.MuOS {
		offset = -1
	}

	for _, item := range items {
		switch item.Item.Text {
		case "Download Art":
			config.DownloadArt = item.SelectedOption == 0
		case "Unzip Downloads":
			config.UnzipDownloads = item.SelectedOption == 0
		case "Show Game Details":
			config.ShowGameDetails = item.SelectedOption == 0
		case "API Timeout":
			idx := item.SelectedOption
			if idx < len(apiTimeoutOptions) {
				config.ApiTimeout = apiTimeoutOptions[idx].Value
			}
		case "Download Timeout":
			idx := item.SelectedOption
			if idx < len(downloadTimeoutOptions) {
				config.DownloadTimeout = downloadTimeoutOptions[idx].Value
			}
		case "Log Level":
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.LogLevel = val
			}
		}
	}
	_ = offset // Reserved for future use
}

func boolToIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

func logLevelToIndex(level string) int {
	switch level {
	case "DEBUG":
		return 0
	case "ERROR":
		return 1
	default:
		return 0
	}
}
