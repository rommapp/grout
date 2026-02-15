package ui

import (
	"errors"
	"grout/internal"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type GameOptionsInput struct {
	Config *internal.Config
	Host   romm.Host
	Game   romm.Rom
}

type GameOptionsOutput struct {
	Action GameOptionsAction
	Config *internal.Config
	Host   romm.Host
	Game   romm.Rom
}

type GameOptionsScreen struct{}

func NewGameOptionsScreen() *GameOptionsScreen {
	return &GameOptionsScreen{}
}

func (s *GameOptionsScreen) Draw(input GameOptionsInput) (GameOptionsOutput, error) {
	config := input.Config
	output := GameOptionsOutput{Action: GameOptionsActionBack, Config: config, Host: input.Host, Game: input.Game}

	items := s.buildMenuItems(config, input.Game)

	showQRText := i18n.Localize(&goi18n.Message{ID: "game_options_show_qr", Other: "Show QR Code"}, nil)
	items = append(items, gaba.ItemWithOptions{
		Item:           gaba.MenuItem{Text: showQRText},
		Options:        []gaba.Option{{DisplayName: "", Value: "show_qr", Type: gaba.OptionTypeClickable}},
		SelectedOption: 0,
	})

	title := i18n.Localize(&goi18n.Message{ID: "game_options_title", Other: "Game Options"}, nil)

	result, err := gaba.OptionsList(
		title,
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
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
		gaba.GetLogger().Error("Game options screen error", "error", err)
		return output, err
	}

	if result.Action == gaba.ListActionSelected {
		if result.Selected >= 0 && result.Selected < len(result.Items) {
			selectedItem := result.Items[result.Selected]
			if selectedItem.Item.Text == showQRText {
				output.Action = GameOptionsActionShowQR
				return output, nil
			}
		}
	}

	s.applySettings(config, input.Game, result.Items)

	err = internal.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving game options", "error", err)
		return output, err
	}

	output.Action = GameOptionsActionSaved
	return output, nil
}

func (s *GameOptionsScreen) buildMenuItems(config *internal.Config, game romm.Rom) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0)

	return items
}

func (s *GameOptionsScreen) applySettings(config *internal.Config, game romm.Rom, items []gaba.ItemWithOptions) {
}
