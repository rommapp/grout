package ui

import (
	"errors"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
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
		localize("logout_confirm_message", "Are you sure you want to logout?"),
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: localize("button_cancel", "Cancel")},
			{ButtonName: "X", HelpText: localize("button_confirm", "Confirm")},
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
