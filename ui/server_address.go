package ui

import (
	"grout/auth"
	"grout/settings"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

type ServerAddressInput struct {
	Config *settings.Config
	Host   settings.Host
}

type ServerAddressOutput struct {
	Action ServerAddressAction
	Host   settings.Host
}

type ServerAddressAction int

const (
	ServerAddressActionSaved ServerAddressAction = iota
	ServerAddressActionBack
)

type ServerAddressScreen struct{}

func NewServerAddressScreen() *ServerAddressScreen {
	return &ServerAddressScreen{}
}

func (s *ServerAddressScreen) Draw(input ServerAddressInput) (ServerAddressOutput, error) {
	host := input.Host

	for {
		result, err := s.drawForm(host)
		if err != nil {
			return ServerAddressOutput{Action: ServerAddressActionBack, Host: input.Host}, err
		}

		if result.Action == ServerAddressActionBack {
			return result, nil
		}

		err = validateServerAddress(result.Host)
		if err == nil {
			return result, nil
		}
		showLoginError(failureMessage(err))

		// Re-display the form with the user's dirty edits
		host = result.Host
	}
}

func (s *ServerAddressScreen) drawForm(host settings.Host) (ServerAddressOutput, error) {
	updated, ok, err := serverForm{
		Title:      localize("settings_server_address", "Server Address"),
		Footer:     []gabagool.FooterHelpItem{FooterBack(), FooterCycle(), FooterSave()},
		SmallTitle: true,
		StatusBar:  StatusBar(),
	}.draw(host)
	if err != nil || !ok {
		return ServerAddressOutput{Action: ServerAddressActionBack, Host: host}, err
	}

	return ServerAddressOutput{Action: ServerAddressActionSaved, Host: updated}, nil
}

// validateServerAddress checks a new address is reachable and that the token
// grout already holds still works against it.
func validateServerAddress(host settings.Host) error {
	result, _ := gabagool.ProcessMessage(
		localize("server_address_validating", "Validating new server address..."),
		gabagool.ProcessMessageOptions{},
		func() (error, error) { return auth.CheckToken(host), nil },
	)

	return result
}
