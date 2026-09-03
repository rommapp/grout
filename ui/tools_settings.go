package ui

import (
	"errors"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type ToolsSettingsInput struct {
	Config                *settings.Config
	Host                  settings.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type ToolsSettingsOutput struct {
	Action                  ToolsSettingsAction
	SyncLocalArtworkClicked bool
	LastSelectedIndex       int
	LastVisibleStartIndex   int
}

type ToolsSettingsScreen struct{}

func NewToolsSettingsScreen() *ToolsSettingsScreen {
	return &ToolsSettingsScreen{}
}

func (s *ToolsSettingsScreen) Draw(input ToolsSettingsInput) (ToolsSettingsOutput, error) {
	config := input.Config
	output := ToolsSettingsOutput{Action: ToolsSettingsActionBack}

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_tools", Other: "Tools"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{
				FooterBack(),
				FooterCycle(),
				FooterSave(),
			},
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
		gaba.GetLogger().Error("Tools settings error", "error", err)
		return output, err
	}

	if result.Action == gaba.ListActionSelected {
		selectedText := items[result.Selected].Item.Text

		if selectedText == i18n.Localize(&goi18n.Message{ID: "settings_sync_local_artwork", Other: "Download Missing Art"}, nil) {
			output.SyncLocalArtworkClicked = true
			output.Action = ToolsSettingsActionSyncLocalArtwork
			return output, nil
		}
	}

	s.applySettings(config, result.Items)

	err = settings.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving tools settings", "error", err)
		return output, err
	}

	output.Action = ToolsSettingsActionSaved
	return output, nil
}

func (s *ToolsSettingsScreen) buildMenuItems(config *settings.Config) []gaba.ItemWithOptions {
	return []gaba.ItemWithOptions{
		{
			Item:    gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_sync_local_artwork", Other: "Download Missing Art"}, nil)},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_kid_mode", Other: "Kid Mode"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "option_disabled", Other: "Disabled"}, nil), Value: false},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "option_enabled", Other: "Enabled"}, nil), Value: true},
			},
			SelectedOption: boolToIndex(config.KidMode),
		},
	}
}

func (s *ToolsSettingsScreen) applySettings(config *settings.Config, items []gaba.ItemWithOptions) {
	for _, item := range items {
		selectedText := item.Item.Text

		switch selectedText {
		case i18n.Localize(&goi18n.Message{ID: "settings_kid_mode", Other: "Kid Mode"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.KidMode = val
				settings.SetKidMode(val)
			}
		}
	}
}
