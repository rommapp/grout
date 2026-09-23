package ui

import (
	"errors"
	"fmt"
	"os"
	"time"

	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type AdvancedSettingsInput struct {
	Config                *settings.Config
	Host                  settings.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type AdvancedSettingsOutput struct {
	Action                AdvancedSettingsAction
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type AdvancedSettingsScreen struct{}

func NewAdvancedSettingsScreen() *AdvancedSettingsScreen {
	return &AdvancedSettingsScreen{}
}

func (s *AdvancedSettingsScreen) Draw(input AdvancedSettingsInput) (AdvancedSettingsOutput, error) {
	config := input.Config
	output := AdvancedSettingsOutput{Action: AdvancedSettingsActionBack}

	rows := advancedRows()
	items := settingItems(rows, *config)

	result, err := gaba.OptionsList(
		localize("settings_advanced", "Advanced"),
		gaba.OptionListSettings{
			FooterHelpItems:      []gaba.FooterHelpItem{FooterBack(), FooterCycle(), FooterSave()},
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)
	if result != nil {
		output.LastSelectedIndex = result.Selected
		output.LastVisibleStartIndex = result.VisibleStartIndex
	}
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Advanced settings error", "error", err)
		return output, err
	}

	if result.Action == gaba.ListActionSelected && result.Selected < len(rows) {
		if action, leads := advancedDestinations[rows[result.Selected].key]; leads {
			if action == AdvancedSettingsActionResetInputMapping {
				return s.resetInputMapping(output)
			}
			output.Action = action
			return output, nil
		}
	}

	applySettingRows(rows, config, result.Items)

	err = settings.SaveConfig(config)
	ApplyRuntimeSettings(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving advanced settings", "error", err)
		return output, err
	}

	output.Action = AdvancedSettingsActionSaved
	return output, nil
}

// resetInputMapping deletes the device's saved button layout so the built-in
// one is used again.
//
// The toolkit reads the mapping once at startup, so the change only takes
// effect on the next run. The caller ends the app; saying so here is the only
// warning the user gets.
func (s *AdvancedSettingsScreen) resetInputMapping(output AdvancedSettingsOutput) (AdvancedSettingsOutput, error) {
	if err := os.Remove(settings.InputMappingFileName); err != nil {
		gaba.GetLogger().Error("Failed to delete input mapping", "error", err)
		gaba.ConfirmationMessage(
			localize("input_mapping_reset_failed", "Could not reset the input mapping.\nCheck the logs for more info."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		output.Action = AdvancedSettingsActionBack
		return output, nil
	}

	gaba.SetInputMappingBytes(nil)
	gaba.ConfirmationMessage(
		localize("input_mapping_reset", "Input mapping reset.\nGrout needs to restart to apply changes."),
		[]gaba.FooterHelpItem{{ButtonName: "A", HelpText: localize("button_exit", "Exit")}},
		gaba.MessageOptions{},
	)

	output.Action = AdvancedSettingsActionResetInputMapping
	return output, nil
}

// advancedDestinations is where each row that navigates goes. Rows that only
// hold a value are absent.
var advancedDestinations = map[string]AdvancedSettingsAction{
	"sync_artwork":        AdvancedSettingsActionSyncArtwork,
	"rebuild_cache":       AdvancedSettingsActionRebuildCache,
	"server_address":      AdvancedSettingsActionServerAddress,
	"input_mapping":       AdvancedSettingsActionInputMapping,
	"reset_input_mapping": AdvancedSettingsActionResetInputMapping,
}

func advancedRows() []settingRow {
	rows := []settingRow{
		clickableRow("sync_artwork", "settings_sync_artwork", "Preload Artwork"),
		clickableRow("rebuild_cache", "settings_rebuild_cache", "Rebuild Cache"),
		{
			key: "download_timeout", label: localize("settings_download_timeout", "Download Timeout"),
			options: downloadTimeoutOptions(),
			get:     func(c settings.Config) any { return c.DownloadTimeout.Duration() },
			set: assign(func(c *settings.Config, v time.Duration) {
				c.DownloadTimeout = settings.DurationSeconds(v)
			}),
			def: 60 * time.Minute,
		},
		{
			key: "api_timeout", label: localize("settings_api_timeout", "API Timeout"),
			options: apiTimeoutOptions(),
			get:     func(c settings.Config) any { return c.ApiTimeout.Duration() },
			set: assign(func(c *settings.Config, v time.Duration) {
				c.ApiTimeout = settings.DurationSeconds(v)
			}),
			def: 30 * time.Second,
		},
		clickableRow("server_address", "settings_server_address", "Server Address"),
		{
			key: "release_channel", label: localize("settings_release_channel", "Release Channel"),
			options: releaseChannelOptions(),
			get:     func(c settings.Config) any { return c.ReleaseChannel },
			set:     assign(func(c *settings.Config, v settings.ReleaseChannel) { c.ReleaseChannel = v }),
			def:     settings.ReleaseChannelMatchRomM,
		},
		{
			key: "log_level", label: localize("settings_log_level", "Log Level"),
			options: logLevelOptions(),
			get:     func(c settings.Config) any { return c.LogLevel },
			set:     assign(func(c *settings.Config, v settings.LogLevel) { c.LogLevel = v }),
			def:     settings.LogLevelError,
		},
		clickableRow("input_mapping", "settings_input_mapping", "Input Mapping"),
	}

	// Only worth offering when there is a saved mapping to undo.
	if _, err := os.Stat(settings.InputMappingFileName); err == nil {
		rows = append(rows, clickableRow("reset_input_mapping", "settings_reset_input_mapping", "Reset Input Mapping"))
	}

	return rows
}

func downloadTimeoutOptions() []gaba.Option {
	minutes := []int{15, 30, 45, 60, 75, 90, 105, 120}

	options := make([]gaba.Option, 0, len(minutes))
	for _, m := range minutes {
		options = append(options, gaba.Option{
			DisplayName: localize(fmt.Sprintf("time_%d_minutes", m), fmt.Sprintf("%d Minutes", m)),
			Value:       time.Duration(m) * time.Minute,
		})
	}
	return options
}

func apiTimeoutOptions() []gaba.Option {
	seconds := []int{15, 30, 45, 60, 75, 90, 120, 180, 240, 300}

	options := make([]gaba.Option, 0, len(seconds))
	for _, s := range seconds {
		options = append(options, gaba.Option{
			DisplayName: localize(fmt.Sprintf("time_%d_seconds", s), fmt.Sprintf("%d Seconds", s)),
			Value:       time.Duration(s) * time.Second,
		})
	}
	return options
}

func releaseChannelOptions() []gaba.Option {
	return []gaba.Option{
		{DisplayName: localize("release_match_romm", "Match RomM"), Value: settings.ReleaseChannelMatchRomM},
		{DisplayName: localize("release_stable", "Stable"), Value: settings.ReleaseChannelStable},
		{DisplayName: localize("release_beta", "Beta"), Value: settings.ReleaseChannelBeta},
	}
}

func logLevelOptions() []gaba.Option {
	return []gaba.Option{
		{DisplayName: localize("log_level_debug", "Debug"), Value: settings.LogLevelDebug},
		{DisplayName: localize("log_level_info", "Info"), Value: settings.LogLevelInfo},
		{DisplayName: localize("log_level_error", "Error"), Value: settings.LogLevelError},
	}
}
