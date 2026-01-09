package ui

import (
	"errors"
	"fmt"
	"grout/internal"
	"grout/internal/constants"
	"os"
	"strconv"
	"strings"
	"time"

	"grout/romm"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type loginInput struct {
	ExistingHost romm.Host
}

type loginOutput struct {
	Host   romm.Host
	Config *internal.Config
}

type loginAttemptResult struct {
	ErrorType string
	ErrorMsg  *goi18n.Message
	Success   bool
}

type LoginScreen struct{}

func newLoginScreen() *LoginScreen {
	return &LoginScreen{}
}

func (s *LoginScreen) draw(input loginInput) (ScreenResult[loginOutput], error) {
	host := input.ExistingHost

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_protocol", Other: "Protocol"}, nil),
			},
			Options: []gabagool.Option{
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "login_protocol_http", Other: "HTTP"}, nil), Value: "http://"},
				{DisplayName: i18n.Localize(&goi18n.Message{ID: "login_protocol_https", Other: "HTTPS"}, nil), Value: "https://"},
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
				Text: i18n.Localize(&goi18n.Message{ID: "login_username", Other: "Username"}, nil),
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					DisplayName:    host.Username,
					KeyboardPrompt: host.Username,
					Value:          host.Username,
				},
			},
		},
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_password", Other: "Password"}, nil),
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					Masked:         true,
					DisplayName:    host.Password,
					KeyboardPrompt: host.Password,
					Value:          host.Password,
				},
			},
		},
	}

	res, err := gabagool.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "login_title", Other: "Login to RomM"}, nil),
		gabagool.OptionListSettings{
			DisableBackButton: false,
			FooterHelpItems: []gabagool.FooterHelpItem{
				{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_quit", Other: "Quit"}, nil)},
				{ButtonName: icons.LeftRight, HelpText: i18n.Localize(&goi18n.Message{ID: "button_cycle", Other: "Cycle"}, nil)},
				{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_login", Other: "Login"}, nil)},
			},
		},
		items,
	)

	if err != nil {
		return withCode(loginOutput{}, gabagool.ExitCodeCancel), nil
	}

	loginSettings := res.Items

	newHost := romm.Host{
		RootURI: fmt.Sprintf("%s%s", loginSettings[0].Value(), loginSettings[1].Value()),
		Port: func(s string) int {
			if n, err := strconv.Atoi(s); err == nil {
				return n
			}
			return 0
		}(loginSettings[2].Value().(string)),
		Username: loginSettings[3].Options[0].Value.(string),
		Password: loginSettings[4].Options[0].Value.(string),
	}

	return success(loginOutput{Host: newHost}), nil
}

func LoginFlow(existingHost romm.Host) (*internal.Config, error) {
	screen := newLoginScreen()

	for {
		result, err := screen.draw(loginInput{ExistingHost: existingHost})
		if err != nil {
			gabagool.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "login_error_unexpected", Other: "Something unexpected happened!\nCheck the logs for more info."}, nil), gabagool.ProcessMessageOptions{}, func() (interface{}, error) {
				time.Sleep(3 * time.Second)
				return nil, nil
			})
			return nil, fmt.Errorf("unable to get login information: %w", err)
		}

		if result.ExitCode == gabagool.ExitCodeBack || result.ExitCode == gabagool.ExitCodeCancel {
			os.Exit(1)
		}

		host := result.Value.Host

		loginResult := attemptLogin(host)

		if loginResult.Success {
			config := &internal.Config{
				Hosts: []romm.Host{host},
			}
			return config, nil
		}

		gabagool.ConfirmationMessage(
			i18n.Localize(loginResult.ErrorMsg, nil),
			[]gabagool.FooterHelpItem{
				{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_continue", Other: "Continue"}, nil)},
			},
			gabagool.MessageOptions{},
		)
		existingHost = host
	}
}

func attemptLogin(host romm.Host) loginAttemptResult {
	validationClient := romm.NewClientFromHost(host, constants.ValidationTimeout)

	result, _ := gabagool.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "login_validating", Other: "Validating connection..."}, nil),
		gabagool.ProcessMessageOptions{},
		func() (interface{}, error) {
			err := validationClient.ValidateConnection()
			if err != nil {
				return classifyLoginError(err), nil
			}

			loginClient := romm.NewClientFromHost(host, constants.LoginTimeout)
			err = loginClient.Login(host.Username, host.Password)
			if err != nil {
				return classifyLoginError(err), nil
			}

			return loginAttemptResult{Success: true}, nil
		},
	)

	return result.(loginAttemptResult)
}

func classifyLoginError(err error) loginAttemptResult {
	if err == nil {
		return loginAttemptResult{Success: true}
	}

	var protocolErr *romm.ProtocolError
	if errors.As(err, &protocolErr) {
		if protocolErr.CorrectProtocol == "https" {
			return loginAttemptResult{
				ErrorType: "protocol",
				ErrorMsg:  &goi18n.Message{ID: "login_error_use_https", Other: "Protocol mismatch!\nPlease use HTTPS instead of HTTP."},
			}
		}
		return loginAttemptResult{
			ErrorType: "protocol",
			ErrorMsg:  &goi18n.Message{ID: "login_error_use_http", Other: "Protocol mismatch!\nPlease use HTTP instead of HTTPS."},
		}
	}

	switch {
	case errors.Is(err, romm.ErrInvalidHostname):
		return loginAttemptResult{
			ErrorType: "dns",
			ErrorMsg:  &goi18n.Message{ID: "login_error_invalid_hostname", Other: "Could not resolve hostname!\nPlease check the hostname is correct."},
		}
	case errors.Is(err, romm.ErrConnectionRefused):
		return loginAttemptResult{
			ErrorType: "connection",
			ErrorMsg:  &goi18n.Message{ID: "login_error_connection_refused", Other: "Could not connect to host!\nPlease check the hostname and port are correct."},
		}
	case errors.Is(err, romm.ErrTimeout):
		return loginAttemptResult{
			ErrorType: "timeout",
			ErrorMsg:  &goi18n.Message{ID: "login_error_timeout", Other: "Connection timed out!\nPlease check your network connection and that the host is reachable."},
		}
	case errors.Is(err, romm.ErrWrongProtocol):
		return loginAttemptResult{
			ErrorType: "protocol",
			ErrorMsg:  &goi18n.Message{ID: "login_error_wrong_protocol", Other: "Protocol mismatch!\nTry switching between http and https."},
		}
	case errors.Is(err, romm.ErrUnauthorized):
		return loginAttemptResult{
			ErrorType: "credentials",
			ErrorMsg:  &goi18n.Message{ID: "login_error_credentials", Other: "Invalid Username or Password."},
		}
	case errors.Is(err, romm.ErrForbidden):
		return loginAttemptResult{
			ErrorType: "forbidden",
			ErrorMsg:  &goi18n.Message{ID: "login_error_forbidden", Other: "Access Forbidden!\nCheck your username/password and try switching between http and https."},
		}
	case errors.Is(err, romm.ErrServerError):
		return loginAttemptResult{
			ErrorType: "server",
			ErrorMsg:  &goi18n.Message{ID: "login_error_server", Other: "RomM server error!\nPlease check the RomM server logs."},
		}
	default:
		gabagool.GetLogger().Warn("Unclassified login error", "error", err)
		return loginAttemptResult{
			ErrorType: "unknown",
			ErrorMsg:  &goi18n.Message{ID: "login_error_unexpected", Other: "Something unexpected happened!\nCheck the logs for more info."},
		}
	}
}

func removeScheme(rawURL string) string {
	if strings.HasPrefix(rawURL, "https://") {
		return strings.TrimPrefix(rawURL, "https://")
	}
	if strings.HasPrefix(rawURL, "http://") {
		return strings.TrimPrefix(rawURL, "http://")
	}
	return rawURL
}
