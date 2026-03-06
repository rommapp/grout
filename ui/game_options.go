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
	Action      GameOptionsAction
	Config      *internal.Config
	Host        romm.Host
	Game        romm.Rom
	NewSlotName string // Set when a new slot is created (for targeted upload)
}

type GameOptionsScreen struct{}

func NewGameOptionsScreen() *GameOptionsScreen {
	return &GameOptionsScreen{}
}

func (s *GameOptionsScreen) Draw(input GameOptionsInput) (GameOptionsOutput, error) {
	config := input.Config
	output := GameOptionsOutput{Action: GameOptionsActionBack, Config: config, Host: input.Host, Game: input.Game}

	// Fetch save summary to determine available slots
	var slotNames []string
	if input.Host.DeviceID != "" {
		client := romm.NewClientFromHost(input.Host, config.ApiTimeout)
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "synced_games_loading_detail", Other: "Loading save details..."}, nil),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (any, error) {
				summary, err := client.GetSaveSummary(input.Game.ID)
				if err == nil {
					for _, slot := range summary.Slots {
						name := "default"
						if slot.Slot != nil {
							name = *slot.Slot
						}
						slotNames = append(slotNames, name)
					}
				}
				return nil, nil
			},
		)
	}

	oldSlotPref := config.GetSlotPreference(input.Game.ID)

	items := s.buildMenuItems(config, input.Game, input.Host.DeviceID != "", slotNames)

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

	if err = internal.SaveSlotPreferences(config); err != nil {
		gaba.GetLogger().Error("Error saving slot preferences", "error", err)
		return output, err
	}

	newSlotPref := config.GetSlotPreference(input.Game.ID)
	if newSlotPref != oldSlotPref {
		output.Action = GameOptionsActionSyncNow
		// Check if this is a brand-new slot (not on server yet) for targeted upload
		isNewSlot := true
		for _, name := range slotNames {
			if name == newSlotPref {
				isNewSlot = false
				break
			}
		}
		if isNewSlot {
			output.NewSlotName = newSlotPref
		}
	} else {
		output.Action = GameOptionsActionSaved
	}
	return output, nil
}

func (s *GameOptionsScreen) buildMenuItems(config *internal.Config, game romm.Rom, deviceRegistered bool, slotNames []string) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0)

	if deviceRegistered {
		saveSlotText := i18n.Localize(&goi18n.Message{ID: "game_options_save_slot", Other: "Save Slot"}, nil)
		slotOpts := BuildSlotOptions(config, game.ID, slotNames)

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: saveSlotText},
			Options:        slotOpts.Options,
			SelectedOption: slotOpts.SelectedIdx,
		})
	}

	return items
}

func (s *GameOptionsScreen) applySettings(config *internal.Config, game romm.Rom, items []gaba.ItemWithOptions) {
	saveSlotText := i18n.Localize(&goi18n.Message{ID: "game_options_save_slot", Other: "Save Slot"}, nil)

	for _, item := range items {
		if item.Item.Text == saveSlotText {
			if item.SelectedOption >= 0 && item.SelectedOption < len(item.Options) {
				selectedOpt := item.Options[item.SelectedOption]
				// Empty string values come from the "New Slot..." keyboard option
				// when the user dismisses the keyboard without typing. Intentionally
				// treated as a no-op so the preference remains unchanged.
				if selectedSlot, ok := selectedOpt.Value.(string); ok && selectedSlot != "" {
					config.SetSlotPreference(game.ID, selectedSlot)
				}
			}
		}
	}
}
