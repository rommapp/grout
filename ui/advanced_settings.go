package ui

import (
	"errors"
	"grout/internal"
	"grout/internal/constants"
	"grout/romm"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type AdvancedSettingsInput struct {
	Config                *internal.Config
	Host                  romm.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type AdvancedSettingsOutput struct {
	RefreshCacheClicked   bool
	SyncArtworkClicked    bool
	MatchOrphansClicked   bool
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type AdvancedSettingsScreen struct{}

func NewAdvancedSettingsScreen() *AdvancedSettingsScreen {
	return &AdvancedSettingsScreen{}
}

func (s *AdvancedSettingsScreen) Draw(input AdvancedSettingsInput) (ScreenResult[AdvancedSettingsOutput], error) {
	config := input.Config
	output := AdvancedSettingsOutput{}

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_advanced", Other: "Advanced"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{
				FooterBack(),
				FooterCycle(),
				FooterSave(),
			},
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
			StatusBar:            StatusBar(),
			SmallTitle:           true,
		},
		items,
	)

	if result != nil {
		output.LastSelectedIndex = result.Selected
		output.LastVisibleStartIndex = result.VisibleStartIndex
	}

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		gaba.GetLogger().Error("Advanced settings error", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	if result.Action == gaba.ListActionSelected {
		selectedText := items[result.Selected].Item.Text

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_refresh_cache", Other: "Refresh Cache"}, nil) {
			output.RefreshCacheClicked = true
			return withCode(output, constants.ExitCodeRefreshCache), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_sync_artwork", Other: "Preload Artwork"}, nil) {
			output.SyncArtworkClicked = true
			return withCode(output, constants.ExitCodeSyncArtwork), nil
		}

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_match_orphans", Other: "Match Orphans By Hash"}, nil) {
			output.MatchOrphansClicked = true
			return withCode(output, constants.ExitCodeMatchOrphans), nil
		}
	}

	s.applySettings(config, result.Items)

	err = internal.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving advanced settings", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	return success(output), nil
}

func (s *AdvancedSettingsScreen) buildMenuItems(config *internal.Config) []gaba.ItemWithOptions {
	return []gaba.ItemWithOptions{
		{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_sync_artwork", Other: "Preload Artwork"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_refresh_cache", Other: "Refresh Cache"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_match_orphans", Other: "Match Orphans By Hash"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_download_timeout", Other: "Download Timeout"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_15_minutes", Other: "15 Minutes"}, nil), Value: 15 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_30_minutes", Other: "30 Minutes"}, nil), Value: 30 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_45_minutes", Other: "45 Minutes"}, nil), Value: 45 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_60_minutes", Other: "60 Minutes"}, nil), Value: 60 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_75_minutes", Other: "75 Minutes"}, nil), Value: 75 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_90_minutes", Other: "90 Minutes"}, nil), Value: 90 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_105_minutes", Other: "105 Minutes"}, nil), Value: 105 * time.Minute},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_120_minutes", Other: "120 Minutes"}, nil), Value: 120 * time.Minute},
			},
			SelectedOption: s.findDownloadTimeoutIndex(config.DownloadTimeout),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_api_timeout", Other: "API Timeout"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_15_seconds", Other: "15 Seconds"}, nil), Value: 15 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_30_seconds", Other: "30 Seconds"}, nil), Value: 30 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_45_seconds", Other: "45 Seconds"}, nil), Value: 45 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_60_seconds", Other: "60 Seconds"}, nil), Value: 60 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_75_seconds", Other: "75 Seconds"}, nil), Value: 75 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_90_seconds", Other: "90 Seconds"}, nil), Value: 90 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_120_seconds", Other: "120 Seconds"}, nil), Value: 120 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_180_seconds", Other: "180 Seconds"}, nil), Value: 180 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_240_seconds", Other: "240 Seconds"}, nil), Value: 240 * time.Second},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "time_300_seconds", Other: "300 Seconds"}, nil), Value: 300 * time.Second},
			},
			SelectedOption: s.findApiTimeoutIndex(config.ApiTimeout),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_kid_mode", Other: "Kid Mode"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "option_disabled", Other: "Disabled"}, nil), Value: false},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "option_enabled", Other: "Enabled"}, nil), Value: true},
			},
			SelectedOption: boolToIndex(config.KidMode),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_log_level", Other: "Log Level"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "log_level_debug", Other: "Debug"}, nil), Value: "DEBUG"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "log_level_error", Other: "Error"}, nil), Value: "ERROR"},
			},
			SelectedOption: logLevelToIndex(config.LogLevel),
		},
	}
}

func (s *AdvancedSettingsScreen) applySettings(config *internal.Config, items []gaba.ItemWithOptions) {
	for _, item := range items {
		selectedText := item.Item.Text

		switch selectedText {
		case i18n.Localize(&goi18n.Message{ID: "settings_download_timeout", Other: "Download Timeout"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(time.Duration); ok {
				config.DownloadTimeout = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_api_timeout", Other: "API Timeout"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(time.Duration); ok {
				config.ApiTimeout = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_log_level", Other: "Log Level"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
				config.LogLevel = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_kid_mode", Other: "Kid Mode"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.KidMode = val
				internal.SetKidMode(val)
			}
		}
	}
}

func (s *AdvancedSettingsScreen) findDownloadTimeoutIndex(timeout time.Duration) int {
	timeouts := []time.Duration{
		15 * time.Minute,
		30 * time.Minute,
		45 * time.Minute,
		60 * time.Minute,
		75 * time.Minute,
		90 * time.Minute,
		105 * time.Minute,
		120 * time.Minute,
	}
	for i, t := range timeouts {
		if t == timeout {
			return i
		}
	}
	return 0 // Default to 15 minutes
}

func (s *AdvancedSettingsScreen) findApiTimeoutIndex(timeout time.Duration) int {
	timeouts := []time.Duration{
		15 * time.Second,
		30 * time.Second,
		45 * time.Second,
		60 * time.Second,
		75 * time.Second,
		90 * time.Second,
		120 * time.Second,
		180 * time.Second,
		240 * time.Second,
		300 * time.Second,
	}
	for i, t := range timeouts {
		if t == timeout {
			return i
		}
	}
	return 0 // Default to 15 seconds
}
