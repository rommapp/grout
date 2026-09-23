package ui

import (
	"errors"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type SyncMenuInput struct {
	Config                *settings.Config
	Host                  settings.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SyncMenuOutput struct {
	Action                SyncMenuAction
	Config                *settings.Config
	Host                  settings.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SyncMenuScreen struct{}

func NewSyncMenuScreen() *SyncMenuScreen {
	return &SyncMenuScreen{}
}

func (s *SyncMenuScreen) Draw(input SyncMenuInput) (SyncMenuOutput, error) {
	output := SyncMenuOutput{
		Action: SyncMenuActionBack,
		Config: input.Config,
		Host:   input.Host,
	}

	const (
		menuSyncNow = iota
		menuSyncedGames
		menuHistory
	)

	items := []gaba.ItemWithOptions{
		{
			Item:    gaba.MenuItem{Text: localize("sync_menu_sync_now", "Sync Now")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item:    gaba.MenuItem{Text: localize("sync_menu_synced_games", "Synced Games")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item:    gaba.MenuItem{Text: localize("sync_menu_history", "View History")},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
	}

	result, err := gaba.OptionsList(
		localize("sync_menu_title", "Save Sync"),
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
			return output, nil
		}
		return output, err
	}

	output.LastSelectedIndex = result.Selected
	output.LastVisibleStartIndex = result.VisibleStartIndex

	if result.Action == gaba.ListActionSelected {
		switch result.Selected {
		case menuSyncNow:
			output.Action = SyncMenuActionSyncNow
		case menuSyncedGames:
			output.Action = SyncMenuActionSyncedGames
		case menuHistory:
			output.Action = SyncMenuActionHistory
		}
	}

	return output, nil
}
