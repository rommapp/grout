package ui

import (
	"errors"
	"fmt"
	"grout/internal"
	"grout/romm"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type ServerAddressInput struct {
	Config *internal.Config
	Host   romm.Host
}

type ServerAddressOutput struct {
	Action ServerAddressAction
	Host   romm.Host
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

		// Validate the new connection
		validationResult := validateServerAddress(result.Host)
		if validationResult.Success {
			return result, nil
		}

		gabagool.ConfirmationMessage(
			i18n.Localize(validationResult.ErrorMsg, nil),
			ContinueFooter(),
			gabagool.MessageOptions{},
		)

		// Re-display the form with the user's dirty edits
		host = result.Host
	}
}

func (s *ServerAddressScreen) drawForm(host romm.Host) (ServerAddressOutput, error) {
	output := ServerAddressOutput{Action: ServerAddressActionBack, Host: host}

	sslVisible := &atomic.Bool{}
	sslVisible.Store(strings.Contains(host.RootURI, "https"))

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_protocol", Other: "Protocol"}, nil),
			},
			Options: []gabagool.Option{
				{
					DisplayName: i18n.Localize(&goi18n.Message{ID: "login_protocol_http", Other: "HTTP"}, nil),
					Value:       "http://",
					OnUpdate:    func(v interface{}) { sslVisible.Store(false) },
				},
				{
					DisplayName: i18n.Localize(&goi18n.Message{ID: "login_protocol_https", Other: "HTTPS"}, nil),
					Value:       "https://",
					OnUpdate:    func(v interface{}) { sslVisible.Store(true) },
				},
			},
			SelectedOption: func() int {
				if strings.Contains(host.RootURI, "https") {
					return 1
				}
				return 0
			}(),
		},
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_hostname", Other: "Hostname"}, nil),
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					KeyboardLayout: gabagool.KeyboardLayoutURL,
					URLShortcuts: []gabagool.URLShortcut{
						{Value: "romm.", SymbolValue: "romm."},
						{Value: ".com", SymbolValue: ".com"},
						{Value: ".org", SymbolValue: ".org"},
						{Value: ".net", SymbolValue: ".net"},
						{Value: ".local", SymbolValue: ".ts.net"},
					},
					DisplayName:    removeScheme(host.RootURI),
					KeyboardPrompt: removeScheme(host.RootURI),
					Value:          removeScheme(host.RootURI),
				},
			},
		},
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_port", Other: "Port (optional)"}, nil),
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					KeyboardLayout: gabagool.KeyboardLayoutNumeric,
					KeyboardPrompt: func() string {
						if host.Port == 0 {
							return ""
						}
						return strconv.Itoa(host.Port)
					}(),
					DisplayName: func() string {
						if host.Port == 0 {
							return ""
						}
						return strconv.Itoa(host.Port)
					}(),
					Value: func() string {
						if host.Port == 0 {
							return ""
						}
						return strconv.Itoa(host.Port)
					}(),
				},
			},
		},
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_ssl_certificates", Other: "SSL Certificates"}, nil),
			},
			Options: []gabagool.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "login_ssl_verify", Other: "Verify"}, nil), Value: false},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "login_ssl_skip", Other: "Skip Verification"}, nil), Value: true},
			},
			SelectedOption: func() int {
				if host.InsecureSkipVerify {
					return 1
				}
				return 0
			}(),
			VisibleWhen: sslVisible,
		},
	}

	res, err := gabagool.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "settings_server_address", Other: "Server Address"}, nil),
		gabagool.OptionListSettings{
			FooterHelpItems: []gabagool.FooterHelpItem{
				FooterBack(),
				{ButtonName: icons.LeftRight, HelpText: i18n.Localize(&goi18n.Message{ID: "button_cycle", Other: "Cycle"}, nil)},
				FooterSave(),
			},
			StatusBar:     StatusBar(),
			UseSmallTitle: true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gabagool.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	settings := res.Items

	newHost := host
	newHost.RootURI = fmt.Sprintf("%s%s", settings[0].Value(), settings[1].Value())
	newHost.Port = func(s string) int {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
		return 0
	}(settings[2].Value().(string))
	newHost.InsecureSkipVerify = settings[3].Options[settings[3].SelectedOption].Value.(bool)

	output.Host = newHost
	output.Action = ServerAddressActionSaved
	return output, nil
}

func validateServerAddress(host romm.Host) loginAttemptResult {
	validationClient := romm.NewClientFromHost(host, internal.ValidationTimeout)

	result, _ := gabagool.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "server_address_validating", Other: "Validating new server address..."}, nil),
		gabagool.ProcessMessageOptions{},
		func() (interface{}, error) {
			err := validationClient.ValidateConnection()
			if err != nil {
				return classifyLoginError(err), nil
			}

			client := romm.NewClientFromHost(host, internal.LoginTimeout)
			if err := client.ValidateToken(); err != nil {
				return classifyLoginError(err), nil
			}

			return loginAttemptResult{Success: true}, nil
		},
	)

	return result.(loginAttemptResult)
}
