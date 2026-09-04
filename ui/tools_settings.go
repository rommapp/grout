package ui

import (
	"errors"

	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type ToolsSettingsInput struct {
	Config                *settings.Config
	Host                  settings.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type ToolsSettingsOutput struct {
	Action                ToolsSettingsAction
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type ToolsSettingsScreen struct{}

func NewToolsSettingsScreen() *ToolsSettingsScreen {
	return &ToolsSettingsScreen{}
}

// toolsDestinations is where each row that navigates goes. Rows that only hold
// a value are absent.
var toolsDestinations = map[string]ToolsSettingsAction{
	"sync_local_artwork": ToolsSettingsActionSyncLocalArtwork,
}

func (s *ToolsSettingsScreen) Draw(input ToolsSettingsInput) (ToolsSettingsOutput, error) {
	config := input.Config
	output := ToolsSettingsOutput{Action: ToolsSettingsActionBack}

	before := *config
	rows := toolsRows()

	result, err := gaba.OptionsList(
		localize("settings_tools", "Tools"),
		gaba.OptionListSettings{
			FooterHelpItems:      []gaba.FooterHelpItem{FooterBack(), FooterCycle(), FooterSave()},
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		settingItems(rows, *config),
	)
	if result != nil {
		output.LastSelectedIndex = result.Selected
		output.LastVisibleStartIndex = result.VisibleStartIndex
	}
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Tools settings error", "error", err)
		return output, err
	}

	if result.Action == gaba.ListActionSelected && result.Selected < len(rows) {
		if action, leads := toolsDestinations[rows[result.Selected].key]; leads {
			output.Action = action
			return output, nil
		}
	}

	applySettingRows(rows, config, result.Items)
	applyKidMode(before, *config)

	err = settings.SaveConfig(config)
	ApplyRuntimeSettings(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving tools settings", "error", err)
		return output, err
	}

	output.Action = ToolsSettingsActionSaved
	return output, nil
}

// applyKidMode moves the session's kid mode only when the user changed the
// setting.
//
// Kid mode can be unlocked for one run without changing what is stored, so
// pushing the stored value back on every save would re-lock a session that was
// deliberately unlocked, just for visiting this screen.
func applyKidMode(before, after settings.Config) {
	if after.KidMode != before.KidMode {
		settings.SetKidMode(after.KidMode)
	}
}

func toolsRows() []settingRow {
	return []settingRow{
		clickableRow("sync_local_artwork", "settings_sync_local_artwork", "Download Missing Art"),
		{
			key: "kid_mode", label: localize("settings_kid_mode", "Kid Mode"),
			options: []gaba.Option{
				{DisplayName: localize("option_disabled", "Disabled"), Value: false},
				{DisplayName: localize("option_enabled", "Enabled"), Value: true},
			},
			// The stored setting, not the session's. A parent who unlocked for
			// this run still needs to see that kid mode is on in order to turn
			// it off.
			get: func(c settings.Config) any { return c.KidMode },
			set: assign(func(c *settings.Config, v bool) { c.KidMode = v }),
		},
	}
}
