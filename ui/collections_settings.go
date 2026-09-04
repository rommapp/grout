package ui

import (
	"errors"

	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type CollectionsSettingsInput struct {
	Config *settings.Config
}

type CollectionsSettingsOutput struct {
	Action CollectionsSettingsAction
	// SyncNeeded means a kind of collection was switched on that was off
	// before, so the cache has none of them and has to fetch them.
	SyncNeeded bool
}

type CollectionsSettingsScreen struct{}

func NewCollectionsSettingsScreen() *CollectionsSettingsScreen {
	return &CollectionsSettingsScreen{}
}

func (s *CollectionsSettingsScreen) Draw(input CollectionsSettingsInput) (CollectionsSettingsOutput, error) {
	config := input.Config
	output := CollectionsSettingsOutput{Action: CollectionsSettingsActionBack}

	before := *config
	rows := collectionsRows()

	result, err := gaba.OptionsList(
		localize("settings_collections", "Collections Settings"),
		gaba.OptionListSettings{
			FooterHelpItems: []gaba.FooterHelpItem{FooterBack(), FooterCycle(), FooterSave()},
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		settingItems(rows, *config),
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Collections settings error", "error", err)
		return output, err
	}

	applySettingRows(rows, config, result.Items)
	output.SyncNeeded = newlyShownCollections(before, *config)

	err = settings.SaveConfig(config)
	ApplyRuntimeSettings(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving collections settings", "error", err)
		return output, err
	}

	output.Action = CollectionsSettingsActionSaved
	return output, nil
}

// newlyShownCollections reports whether a kind of collection was switched on
// that was off before.
//
// Switching one off needs nothing: the cache keeps what it has and the screen
// stops showing it. Switching one on means the cache has never fetched that
// kind, so there is nothing to show until it does.
func newlyShownCollections(before, after settings.Config) bool {
	turnedOn := func(was, now bool) bool { return !was && now }

	return turnedOn(before.ShowRegularCollections, after.ShowRegularCollections) ||
		turnedOn(before.ShowSmartCollections, after.ShowSmartCollections) ||
		turnedOn(before.ShowVirtualCollections, after.ShowVirtualCollections)
}

func collectionsRows() []settingRow {
	return []settingRow{
		{
			key: "regular", label: localize("settings_show_collections", "Collections"),
			options: showHide(),
			get:     func(c settings.Config) any { return c.ShowRegularCollections },
			set:     assign(func(c *settings.Config, v bool) { c.ShowRegularCollections = v }),
		},
		{
			key: "smart", label: localize("settings_show_smart_collections", "Smart Collections"),
			options: showHide(),
			get:     func(c settings.Config) any { return c.ShowSmartCollections },
			set:     assign(func(c *settings.Config, v bool) { c.ShowSmartCollections = v }),
		},
		{
			key: "virtual", label: localize("settings_show_virtual_collections", "Virtual Collections"),
			options: showHide(),
			get:     func(c settings.Config) any { return c.ShowVirtualCollections },
			set:     assign(func(c *settings.Config, v bool) { c.ShowVirtualCollections = v }),
		},
		{
			key: "view", label: localize("settings_collection_view", "Collection View"),
			options: collectionViewOptions(),
			get:     func(c settings.Config) any { return c.CollectionView },
			set:     assign(func(c *settings.Config, v settings.CollectionView) { c.CollectionView = v }),
			def:     settings.CollectionViewPlatform,
		},
	}
}

func collectionViewOptions() []gaba.Option {
	return []gaba.Option{
		{DisplayName: localize("collection_view_platform", "Platform"), Value: settings.CollectionViewPlatform},
		{DisplayName: localize("collection_view_unified", "Unified"), Value: settings.CollectionViewUnified},
	}
}
