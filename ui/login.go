package ui

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"grout/auth"
	"grout/catalog"
	"grout/settings"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// ErrLoginCancelled means the user backed out of the first login screen. There
// is nothing behind it, so the caller decides what that means: on first launch
// it ends the app.
var ErrLoginCancelled = errors.New("login cancelled")

const (
	authModeDevicePairing = "device_pairing"
	authModePairingCode   = "pairing_code"
)

type LoginScreen struct{}

func newLoginScreen() *LoginScreen {
	return &LoginScreen{}
}

// attempt is the outcome of one try at logging in.
type attempt struct {
	Host settings.Host
	OK   bool
	// Message is what to show, or nil when there is nothing worth saying: a
	// success, or a pairing the user cancelled themselves.
	Message *goi18n.Message
}

func (s *LoginScreen) drawServer(host settings.Host) (settings.Host, bool, error) {
	return serverForm{
		Title: localize("login_server_title", "Server"),
		Footer: []gabagool.FooterHelpItem{
			FooterQuit(),
			FooterCycle(),
			{ButtonName: icons.Start, HelpText: localize("button_continue", "Continue")},
		},
	}.draw(host)
}

// authSelection is what the auth screen hands back to the login flow.
type authSelection struct {
	Host       settings.Host
	Mode       string
	DeviceName string
	Code       string
	Cancelled  bool
}

// drawAuth collects the auth method and its inputs. On RomM 5.0 and later the
// user picks device pairing or a pairing code; older servers only support the
// code, so the picker is hidden.
func (s *LoginScreen) drawAuth(host settings.Host, supportsDevicePairing bool) (authSelection, error) {
	// The picker chooses which of the two input rows is on screen. Without a
	// picker there is only the pairing code.
	pickerVisible := &atomic.Bool{}
	pickerVisible.Store(supportsDevicePairing)

	deviceVisible := &atomic.Bool{}
	deviceVisible.Store(supportsDevicePairing)

	pairingVisible := &atomic.Bool{}
	pairingVisible.Store(!supportsDevicePairing)

	defaultDeviceName := host.DeviceName
	if defaultDeviceName == "" {
		if hostname, err := os.Hostname(); err == nil {
			defaultDeviceName = hostname
		}
	}

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{Text: localize("login_auth_method", "Auth Method")},
			Options: []gabagool.Option{
				{
					DisplayName: localize("login_auth_device_pairing", "Pair with Another Device"),
					Value:       authModeDevicePairing,
					OnUpdate: func(any) {
						deviceVisible.Store(true)
						pairingVisible.Store(false)
					},
				},
				{
					DisplayName: localize("login_auth_pairing_code", "Pairing Code"),
					Value:       authModePairingCode,
					OnUpdate: func(any) {
						deviceVisible.Store(false)
						pairingVisible.Store(true)
					},
				},
			},
			VisibleWhen: pickerVisible,
		},
		{
			Item: gabagool.MenuItem{Text: localize("login_device_name", "Device Name")},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					DisplayName:    defaultDeviceName,
					KeyboardPrompt: defaultDeviceName,
					Value:          defaultDeviceName,
				},
			},
			VisibleWhen: deviceVisible,
		},
		{
			Item: gabagool.MenuItem{Text: localize("login_pairing_code", "Pairing Code")},
			Options: []gabagool.Option{
				{Type: gabagool.OptionTypeKeyboard, Value: ""},
			},
			VisibleWhen: pairingVisible,
		},
	}

	result, err := gabagool.OptionsList(
		localize("login_auth_title", "Authentication"),
		gabagool.OptionListSettings{
			FooterHelpItems: []gabagool.FooterHelpItem{
				FooterBack(),
				FooterCycle(),
				{ButtonName: icons.Start, HelpText: localize("button_login", "Login")},
			},
		},
		items,
	)
	if err != nil {
		if errors.Is(err, gabagool.ErrCancelled) {
			return authSelection{Host: host, Cancelled: true}, nil
		}
		return authSelection{Host: host, Cancelled: true}, err
	}

	fields := result.Items

	selection := authSelection{Host: host, Mode: authModePairingCode}
	if supportsDevicePairing {
		selection.Mode = fields[0].Options[fields[0].SelectedOption].Value.(string)
	}

	// Whatever the last login left behind is no longer the token being used.
	selection.Host.Username = ""
	selection.Host.Token = ""
	selection.Host.TokenName = ""
	selection.Host.TokenExpiresAt = ""

	if selection.Mode == authModeDevicePairing {
		selection.DeviceName, _ = fields[1].Options[0].Value.(string)
		if selection.DeviceName == "" {
			selection.DeviceName = defaultDeviceName
		}
	} else {
		selection.Code, _ = fields[2].Options[0].Value.(string)
	}

	return selection, nil
}

// LoginFlow walks the user from a server address to a working token. It returns
// ErrLoginCancelled if they back out of the first screen.
func LoginFlow(existingHost settings.Host) (*settings.Config, error) {
	screen := newLoginScreen()
	host := existingHost

	for {
		updated, ok, err := screen.drawServer(host)
		if err != nil {
			// The caller ends the app on this, so say so while there is still
			// a screen to say it on.
			showLoginError(failureMessage(err))
			return nil, fmt.Errorf("collecting server details: %w", err)
		}
		if !ok {
			return nil, ErrLoginCancelled
		}
		host = updated

		capabilities, err := connect(host)
		if err != nil {
			showLoginError(failureMessage(err))
			continue
		}

		for {
			selection, err := screen.drawAuth(host, capabilities.SupportsDevicePairing)
			if err != nil {
				showLoginError(failureMessage(err))
				return nil, fmt.Errorf("collecting credentials: %w", err)
			}
			if selection.Cancelled {
				// Back to the server screen with the address they typed.
				break
			}

			result := attemptLogin(selection)
			if result.OK {
				config := &settings.Config{Hosts: []settings.Host{result.Host}}
				// An older server has no binding to load, which is not a reason
				// to fail a working login.
				_ = catalog.LoadPlatformsBinding(config, result.Host)
				return config, nil
			}

			if result.Message != nil {
				showLoginError(result.Message)
			}
			host = result.Host
		}
	}
}

func showLoginError(message *goi18n.Message) {
	gabagool.ConfirmationMessage(i18n.Localize(message, nil), ContinueFooter(), gabagool.MessageOptions{})
}

// connect checks the server is reachable while showing a spinner.
func connect(host settings.Host) (auth.Capabilities, error) {
	type outcome struct {
		Capabilities auth.Capabilities
		Err          error
	}

	result, _ := gabagool.ProcessMessage(
		localize("login_validating_connection", "Validating connection..."),
		gabagool.ProcessMessageOptions{},
		func() (outcome, error) {
			capabilities, err := auth.Connect(host)
			return outcome{Capabilities: capabilities, Err: err}, nil
		},
	)

	return result.Capabilities, result.Err
}

func attemptLogin(selection authSelection) attempt {
	if selection.Mode == authModeDevicePairing {
		return attemptDevicePairing(selection)
	}
	return attemptPairingCode(selection.Host, selection.Code)
}

// attemptDevicePairing runs the RomM 5.0 pairing screen, where the user
// approves the device from a browser.
func attemptDevicePairing(selection authSelection) attempt {
	result := NewDevicePairingScreen().Execute(DevicePairingInput{
		Host:       selection.Host,
		DeviceName: selection.DeviceName,
	})

	switch result.Outcome {
	case auth.PairingApproved:
		return attempt{Host: result.Host, OK: true}
	case auth.PairingCancelled:
		// They backed out, so they already know why nothing happened.
		return attempt{Host: result.Host}
	case auth.PairingDenied:
		return attempt{Host: result.Host, Message: &goi18n.Message{
			ID: "login_error_pairing_denied", Other: "Pairing was denied on the server."}}
	case auth.PairingExpired:
		return attempt{Host: result.Host, Message: &goi18n.Message{
			ID: "login_error_pairing_expired", Other: "The pairing request expired.\nPlease try again."}}
	default:
		return attempt{Host: result.Host, Message: failureMessage(result.Err)}
	}
}

// attemptPairingCode exchanges a typed code for a token. This is the only
// option before RomM 5.0 and still offered after it.
func attemptPairingCode(host settings.Host, code string) attempt {
	if code == "" {
		return attempt{Host: host, Message: failureMessage(auth.ErrNoCode)}
	}

	result, _ := gabagool.ProcessMessage(
		localize("login_validating", "Logging in..."),
		gabagool.ProcessMessageOptions{},
		func() (attempt, error) {
			paired, err := auth.ExchangeCode(host, code)
			if err != nil {
				return attempt{Host: host, Message: failureMessage(err)}, nil
			}
			return attempt{Host: paired, OK: true}, nil
		},
	)

	return result
}

// failureMessage is what to tell the user about a login that did not work.
func failureMessage(err error) *goi18n.Message {
	switch auth.Classify(err) {
	case auth.FailureHostname:
		return &goi18n.Message{ID: "login_error_invalid_hostname",
			Other: "Could not resolve hostname!\nPlease check the hostname is correct."}
	case auth.FailureConnection:
		return &goi18n.Message{ID: "login_error_connection_refused",
			Other: "Could not connect to host!\nPlease check the hostname and port are correct."}
	case auth.FailureTimeout:
		return &goi18n.Message{ID: "login_error_timeout",
			Other: "Connection timed out!\nPlease check your network connection and that the host is reachable."}
	case auth.FailureCredentials:
		return &goi18n.Message{ID: "login_error_credentials", Other: "Invalid credentials."}
	case auth.FailureForbidden:
		return &goi18n.Message{ID: "login_error_forbidden",
			Other: "Access Forbidden!\nCheck your credentials and try switching between http and https."}
	case auth.FailureServer:
		return &goi18n.Message{ID: "login_error_server",
			Other: "RomM server error!\nPlease check the RomM server logs."}
	case auth.FailureNoCode:
		return &goi18n.Message{ID: "login_error_no_code",
			Other: "Enter the pairing code from your RomM profile."}
	case auth.FailureInvalidCode:
		return &goi18n.Message{ID: "login_error_invalid_code",
			Other: "Invalid or expired pairing code.\nPlease try again."}
	}

	return &goi18n.Message{ID: "login_error_unexpected",
		Other: "Something unexpected happened!\nCheck the logs for more info."}
}
