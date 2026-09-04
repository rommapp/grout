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

// advancedEntry is one row of the screen. A row either holds a value the user
// can change or leads somewhere; never both.
type advancedEntry struct {
	// key identifies the row when the screen is read back, since a label
	// changes with the language.
	key      string
	labelID  string
	fallback string
	// action is where a row that leads somewhere goes.
	action AdvancedSettingsAction
	// options, get and set belong to a row holding a value. def is what the
	// settings package would fill in, so a config it has not reached opens on
	// the same answer the rest of the app uses.
	options []gaba.Option
	get     func(settings.Config) any
	set     func(*settings.Config, any)
	def     any
}

// leadsSomewhere reports whether choosing this row navigates rather than
// changing a value.
func (e advancedEntry) leadsSomewhere() bool { return e.options == nil }

func (s *AdvancedSettingsScreen) Draw(input AdvancedSettingsInput) (AdvancedSettingsOutput, error) {
	config := input.Config
	output := AdvancedSettingsOutput{Action: AdvancedSettingsActionBack}

	rows := advancedRows()
	items := advancedItems(rows, *config)

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
		row := rows[result.Selected]
		if row.leadsSomewhere() {
			if row.action == AdvancedSettingsActionResetInputMapping {
				return s.resetInputMapping(output)
			}
			output.Action = row.action
			return output, nil
		}
	}

	applyAdvanced(rows, config, result.Items)

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

func advancedItems(rows []advancedEntry, config settings.Config) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(rows))

	for _, row := range rows {
		item := gaba.ItemWithOptions{
			Item: gaba.MenuItem{Text: localize(row.labelID, row.fallback), Metadata: row.key},
		}
		if row.leadsSomewhere() {
			item.Options = []gaba.Option{{Type: gaba.OptionTypeClickable}}
		} else {
			item.Options = row.options
			item.SelectedOption = optionIndexOr(row.options, row.get(config), row.def)
		}
		items = append(items, item)
	}

	return items
}

func applyAdvanced(rows []advancedEntry, config *settings.Config, items []gaba.ItemWithOptions) {
	byKey := make(map[string]advancedEntry, len(rows))
	for _, row := range rows {
		byKey[row.key] = row
	}

	for _, item := range items {
		key, _ := item.Item.Metadata.(string)
		row, ok := byKey[key]
		if !ok || row.set == nil {
			continue
		}
		if item.SelectedOption < 0 || item.SelectedOption >= len(item.Options) {
			continue
		}
		row.set(config, item.Options[item.SelectedOption].Value)
	}
}

func advancedRows() []advancedEntry {
	rows := []advancedEntry{
		{key: "sync_artwork", labelID: "settings_sync_artwork", fallback: "Preload Artwork",
			action: AdvancedSettingsActionSyncArtwork},
		{key: "rebuild_cache", labelID: "settings_rebuild_cache", fallback: "Rebuild Cache",
			action: AdvancedSettingsActionRebuildCache},
		{
			key: "download_timeout", labelID: "settings_download_timeout", fallback: "Download Timeout",
			options: downloadTimeoutOptions(),
			get:     func(c settings.Config) any { return c.DownloadTimeout.Duration() },
			set: assignAdvanced(func(c *settings.Config, v time.Duration) {
				c.DownloadTimeout = settings.DurationSeconds(v)
			}),
			def: 60 * time.Minute,
		},
		{
			key: "api_timeout", labelID: "settings_api_timeout", fallback: "API Timeout",
			options: apiTimeoutOptions(),
			get:     func(c settings.Config) any { return c.ApiTimeout.Duration() },
			set: assignAdvanced(func(c *settings.Config, v time.Duration) {
				c.ApiTimeout = settings.DurationSeconds(v)
			}),
			def: 30 * time.Second,
		},
		{key: "server_address", labelID: "settings_server_address", fallback: "Server Address",
			action: AdvancedSettingsActionServerAddress},
		{
			key: "release_channel", labelID: "settings_release_channel", fallback: "Release Channel",
			options: releaseChannelOptions(),
			get:     func(c settings.Config) any { return c.ReleaseChannel },
			set:     assignAdvanced(func(c *settings.Config, v settings.ReleaseChannel) { c.ReleaseChannel = v }),
			def:     settings.ReleaseChannelMatchRomM,
		},
		{
			key: "log_level", labelID: "settings_log_level", fallback: "Log Level",
			options: logLevelOptions(),
			get:     func(c settings.Config) any { return c.LogLevel },
			set:     assignAdvanced(func(c *settings.Config, v settings.LogLevel) { c.LogLevel = v }),
			def:     settings.LogLevelError,
		},
		{key: "input_mapping", labelID: "settings_input_mapping", fallback: "Input Mapping",
			action: AdvancedSettingsActionInputMapping},
	}

	// Only worth offering when there is a saved mapping to undo.
	if _, err := os.Stat(settings.InputMappingFileName); err == nil {
		rows = append(rows, advancedEntry{
			key: "reset_input_mapping", labelID: "settings_reset_input_mapping", fallback: "Reset Input Mapping",
			action: AdvancedSettingsActionResetInputMapping,
		})
	}

	return rows
}

// assignAdvanced stores a value of the type a row deals in, ignoring anything
// else, so a mistyped option cannot write nonsense into the config.
func assignAdvanced[T any](set func(*settings.Config, T)) func(*settings.Config, any) {
	return func(config *settings.Config, value any) {
		if typed, ok := value.(T); ok {
			set(config, typed)
		}
	}
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
