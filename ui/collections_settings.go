package ui

import (
	"errors"
	"grout/internal"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type CollectionsSettingsInput struct {
	Config *internal.Config
}

type CollectionsSettingsOutput struct {
	Action     CollectionsSettingsAction
	SyncNeeded bool
}

type CollectionsSettingsScreen struct{}

func NewCollectionsSettingsScreen() *CollectionsSettingsScreen {
	return &CollectionsSettingsScreen{}
}

func (s *CollectionsSettingsScreen) Draw(input CollectionsSettingsInput) (CollectionsSettingsOutput, error) {
	config := input.Config
	output := CollectionsSettingsOutput{Action: CollectionsSettingsActionBack}

	prevRegular := config.ShowRegularCollections
	prevSmart := config.ShowSmartCollections
	prevVirtual := config.ShowVirtualCollections

	items := s.buildMenuItems(config)

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_collections", Other: "Collections Settings"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{
				FooterBack(),
				FooterCycle(),
				FooterSave(),
			},
			InitialSelectedIndex: 0,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Collections settings error", "error", err)
		return output, err
	}

	s.applySettings(config, result.Items)

	if (!prevRegular && config.ShowRegularCollections) ||
		(!prevSmart && config.ShowSmartCollections) ||
		(!prevVirtual && config.ShowVirtualCollections) {
		output.SyncNeeded = true
	}

	err = internal.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving collections settings", "error", err)
		return output, err
	}

	output.Action = CollectionsSettingsActionSaved
	return output, nil
}

func (s *CollectionsSettingsScreen) buildMenuItems(config *internal.Config) []gaba.ItemWithOptions {
	return []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_show_collections", Other: "Collections"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_show", Other: "Show"}, nil), Value: true},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_hide", Other: "Hide"}, nil), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowRegularCollections),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_show_smart_collections", Other: "Smart Collections"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_show", Other: "Show"}, nil), Value: true},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_hide", Other: "Hide"}, nil), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowSmartCollections),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_show_virtual_collections", Other: "Virtual Collections"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_show", Other: "Show"}, nil), Value: true},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_hide", Other: "Hide"}, nil), Value: false},
			},
			SelectedOption: boolToIndex(!config.ShowVirtualCollections),
		},
		{
			Item: gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "settings_collection_view", Other: "Collection View"}, nil)},
			Options: []gaba.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "collection_view_platform", Other: "Platform"}, nil), Value: internal.CollectionViewPlatform},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "collection_view_unified", Other: "Unified"}, nil), Value: internal.CollectionViewUnified},
			},
			SelectedOption: collectionViewToIndex(config.CollectionView),
		},
	}
}

func (s *CollectionsSettingsScreen) applySettings(config *internal.Config, items []gaba.ItemWithOptions) {
	for _, item := range items {
		selectedText := item.Item.Text

		switch selectedText {
		case i18n.Localize(&goi18n.Message{ID: "settings_show_collections", Other: "Collections"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.ShowRegularCollections = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_show_smart_collections", Other: "Smart Collections"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.ShowSmartCollections = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_show_virtual_collections", Other: "Virtual Collections"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
				config.ShowVirtualCollections = val
			}

		case i18n.Localize(&goi18n.Message{ID: "settings_collection_view", Other: "Collection View"}, nil):
			if val, ok := item.Options[item.SelectedOption].Value.(internal.CollectionView); ok {
				config.CollectionView = val
			}
		}
	}
}
