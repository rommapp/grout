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
		defaultLabel := i18n.Localize(&goi18n.Message{ID: "common_default", Other: "Default"}, nil)
		newSlotLabel := i18n.Localize(&goi18n.Message{ID: "game_options_new_slot", Other: "New Slot..."}, nil)

		options := make([]gaba.Option, 0, len(slotNames)+2)

		if len(slotNames) == 0 {
			options = append(options, gaba.Option{DisplayName: defaultLabel, Value: "default"})
		} else {
			for _, name := range slotNames {
				displayName := name
				if name == "default" {
					displayName = defaultLabel
				}
				options = append(options, gaba.Option{DisplayName: displayName, Value: name})
			}
		}

		options = append(options, gaba.Option{
			DisplayName:    newSlotLabel,
			Value:          "",
			Type:           gaba.OptionTypeKeyboard,
			KeyboardPrompt: "",
		})

		currentPref := config.GetSlotPreference(game.ID)
		selectedIdx := 0
		for i, opt := range options {
			if val, ok := opt.Value.(string); ok && val == currentPref {
				selectedIdx = i
				break
			}
		}

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: saveSlotText},
			Options:        options,
			SelectedOption: selectedIdx,
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
				if selectedSlot, ok := selectedOpt.Value.(string); ok && selectedSlot != "" {
					config.SetSlotPreference(game.ID, selectedSlot)
				}
			}
		}
	}
}
