package ui

import (
	"errors"

	"grout/cfw"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SettingsInput struct {
	Config                *settings.Config
	CFW                   cfw.CFW
	Host                  settings.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SettingsOutput struct {
	Action                SettingsAction
	Config                *settings.Config
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SettingsScreen struct{}

func NewSettingsScreen() *SettingsScreen {
	return &SettingsScreen{}
}

// settingsEntry is one row of the settings menu: what it says, and where
// choosing it goes.
type settingsEntry struct {
	// key identifies the row when the screen is read back. A label changes
	// with the language; this does not.
	key      string
	labelID  string
	fallback string
	action   SettingsAction
}

// settingsMenu is the menu in the order it is shown.
var settingsMenu = []settingsEntry{
	{"general", "settings_general", "General", SettingsActionGeneral},
	{"collections", "settings_collections", "Collections Settings", SettingsActionCollections},
	{"directory_mappings", "settings_edit_mappings", "Directory Mappings", SettingsActionPlatformMapping},
	{"save_sync", "settings_save_sync", "Save Sync", SettingsActionSaveSync},
	{"tools", "settings_tools", "Tools", SettingsActionTools},
	{"advanced", "settings_advanced", "Advanced", SettingsActionAdvanced},
	{"info", "settings_info", "Grout Info", SettingsActionInfo},
	{"check_updates", "update_check_for_updates", "Check for Updates", SettingsActionCheckUpdate},
}

func (s *SettingsScreen) Draw(input SettingsInput) (SettingsOutput, error) {
	output := SettingsOutput{Action: SettingsActionBack, Config: input.Config}

	items := make([]gaba.ItemWithOptions, 0, len(settingsMenu))
	for _, entry := range settingsMenu {
		items = append(items, gaba.ItemWithOptions{
			Item:    gaba.MenuItem{Text: localize(entry.labelID, entry.fallback), Metadata: entry.key},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		})
	}

	result, err := gaba.OptionsList(
		localize("settings_title", "Settings"),
		gaba.OptionListSettings{
			FooterHelpItems:      []gaba.FooterHelpItem{FooterBack(), FooterSelect()},
			InitialSelectedIndex: input.LastSelectedIndex,
			VisibleStartIndex:    input.LastVisibleStartIndex,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return SettingsOutput{Action: SettingsActionBack}, nil
		}
		return SettingsOutput{Action: SettingsActionBack}, err
	}

	output.LastSelectedIndex = result.Selected
	output.LastVisibleStartIndex = result.VisibleStartIndex

	if result.Action == gaba.ListActionSelected && result.Selected < len(items) {
		if key, ok := items[result.Selected].Item.Metadata.(string); ok {
			if action, found := settingsAction(key); found {
				output.Action = action
				return output, nil
			}
		}
	}

	output.Action = SettingsActionSaved
	return output, nil
}

func settingsAction(key string) (SettingsAction, bool) {
	for _, entry := range settingsMenu {
		if entry.key == key {
			return entry.action, true
		}
	}
	return SettingsActionBack, false
}

func boolToIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}
