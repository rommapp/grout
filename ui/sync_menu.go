package ui

import (
	"errors"
	"grout/internal"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SyncMenuInput struct {
	Config                *internal.Config
	Host                  romm.Host
	LastSelectedIndex     int
	LastVisibleStartIndex int
}

type SyncMenuOutput struct {
	Action                SyncMenuAction
	Config                *internal.Config
	Host                  romm.Host
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

	syncNowText := i18n.Localize(&goi18n.Message{ID: "sync_menu_sync_now", Other: "Sync Now"}, nil)
	syncedGamesText := i18n.Localize(&goi18n.Message{ID: "sync_menu_synced_games", Other: "Synced Games"}, nil)
	historyText := i18n.Localize(&goi18n.Message{ID: "sync_menu_history", Other: "View History"}, nil)

	items := []gaba.ItemWithOptions{
		{
			Item:    gaba.MenuItem{Text: syncNowText},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item:    gaba.MenuItem{Text: syncedGamesText},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
		{
			Item:    gaba.MenuItem{Text: historyText},
			Options: []gaba.Option{{Type: gaba.OptionTypeClickable}},
		},
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "sync_menu_title", Other: "Save Sync"}, nil),
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
		selectedText := items[result.Selected].Item.Text

		if selectedText == syncNowText {
			output.Action = SyncMenuActionSyncNow
			return output, nil
		}

		if selectedText == syncedGamesText {
			output.Action = SyncMenuActionSyncedGames
			return output, nil
		}

		if selectedText == historyText {
			output.Action = SyncMenuActionHistory
			return output, nil
		}
	}

	return output, nil
}
