package ui

import (
	"errors"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type LogoutConfirmationOutput struct {
	Action    LogoutConfirmationAction
	Confirmed bool
}

type LogoutConfirmationScreen struct{}

func NewLogoutConfirmationScreen() *LogoutConfirmationScreen {
	return &LogoutConfirmationScreen{}
}

func (s *LogoutConfirmationScreen) Draw() (LogoutConfirmationOutput, error) {
	output := LogoutConfirmationOutput{Action: LogoutConfirmationActionCancel}

	_, err := gaba.ConfirmationMessage(
		i18n.Localize(&goi18n.Message{ID: "logout_confirm_message", Other: "Are you sure you want to logout?"}, nil),
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_cancel", Other: "Cancel"}, nil)},
			{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_confirm", Other: "Confirm"}, nil)},
		},
		gaba.MessageOptions{
			ConfirmButton: buttons.VirtualButtonX,
		},
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil // B button - cancel
		}
		return output, err
	}

	// X button pressed - confirm logout
	output.Confirmed = true
	output.Action = LogoutConfirmationActionConfirm
	return output, nil
}
