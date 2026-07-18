package ui

import (
	"errors"
	"fmt"
	"grout/internal"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"grout/romm"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type loginOutput struct {
	Host      romm.Host
	Config    *internal.Config
	Cancelled bool
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

// drawServerInfo collects server connection details (protocol, hostname, port, SSL).
func (s *LoginScreen) drawServerInfo(host romm.Host) (romm.Host, bool, error) {
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
		i18n.Localize(&goi18n.Message{ID: "login_server_title", Other: "Server"}, nil),
		gabagool.OptionListSettings{
			DisableBackButton: false,
			FooterHelpItems: []gabagool.FooterHelpItem{
				{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_quit", Other: "Quit"}, nil)},
				{ButtonName: icons.LeftRight, HelpText: i18n.Localize(&goi18n.Message{ID: "button_cycle", Other: "Cycle"}, nil)},
				{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_continue", Other: "Continue"}, nil)},
			},
		},
		items,
	)

	if err != nil {
		return host, true, nil
	}

	settings := res.Items

	newHost := romm.Host{
		RootURI: fmt.Sprintf("%s%s", settings[0].Value(), settings[1].Value()),
		Port: func(s string) int {
			if n, err := strconv.Atoi(s); err == nil {
				return n
			}
			return 0
		}(settings[2].Value().(string)),
		InsecureSkipVerify: settings[3].Options[settings[3].SelectedOption].Value.(bool),
		ClientDeviceID:     host.ClientDeviceID,
		DeviceID:           host.DeviceID,
		DeviceName:         host.DeviceName,
	}

	return newHost, false, nil
}

const (
	authModeDevicePairing = "device_pairing"
	authModePairingCode   = "pairing_code"
)

// authSelection is what the auth screen hands back to the login flow.
type authSelection struct {
	Host       romm.Host
	Mode       string
	DeviceName string
	Cancelled  bool
}

// drawAuth collects the auth method and its inputs. On RomM 5.0+ the user
// picks Device Pairing (default) or Pairing Code; older servers only support
// the pairing code, so the picker is hidden.
func (s *LoginScreen) drawAuth(host romm.Host, supportsDeviceAuth bool) (authSelection, error) {
	pickerVisible := &atomic.Bool{}
	pickerVisible.Store(supportsDeviceAuth)

	deviceVisible := &atomic.Bool{}
	deviceVisible.Store(supportsDeviceAuth)

	pairingVisible := &atomic.Bool{}
	pairingVisible.Store(!supportsDeviceAuth)

	defaultDeviceName := host.DeviceName
	if defaultDeviceName == "" {
		if hostname, err := os.Hostname(); err == nil {
			defaultDeviceName = hostname
		}
	}

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_auth_method", Other: "Auth Method"}, nil),
			},
			Options: []gabagool.Option{
				{
					DisplayName: i18n.Localize(&goi18n.Message{ID: "login_auth_device_pairing", Other: "Pair with Another Device"}, nil),
					Value:       authModeDevicePairing,
					OnUpdate: func(v interface{}) {
						deviceVisible.Store(true)
						pairingVisible.Store(false)
					},
				},
				{
					DisplayName: i18n.Localize(&goi18n.Message{ID: "login_auth_pairing_code", Other: "Pairing Code"}, nil),
					Value:       authModePairingCode,
					OnUpdate: func(v interface{}) {
						deviceVisible.Store(false)
						pairingVisible.Store(true)
					},
				},
			},
			SelectedOption: 0,
			VisibleWhen:    pickerVisible,
		},
		{
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_device_name", Other: "Device Name"}, nil),
			},
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
			Item: gabagool.MenuItem{
				Text: i18n.Localize(&goi18n.Message{ID: "login_pairing_code", Other: "Pairing Code"}, nil),
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					DisplayName:    "",
					KeyboardPrompt: "",
					Value:          "",
				},
			},
			VisibleWhen: pairingVisible,
		},
	}

	res, err := gabagool.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "login_auth_title", Other: "Authentication"}, nil),
		gabagool.OptionListSettings{
			DisableBackButton: false,
			FooterHelpItems: []gabagool.FooterHelpItem{
				FooterBack(),
				{ButtonName: icons.LeftRight, HelpText: i18n.Localize(&goi18n.Message{ID: "button_cycle", Other: "Cycle"}, nil)},
				{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_login", Other: "Login"}, nil)},
			},
		},
		items,
	)

	if err != nil {
		return authSelection{Host: host, Cancelled: true}, nil
	}

	settings := res.Items

	sel := authSelection{Host: host, Mode: authModePairingCode}
	if supportsDeviceAuth {
		sel.Mode = settings[0].Options[settings[0].SelectedOption].Value.(string)
	}

	newHost := host
	newHost.Username = ""

	if sel.Mode == authModeDevicePairing {
		sel.DeviceName = settings[1].Options[0].Value.(string)
		if sel.DeviceName == "" {
			sel.DeviceName = defaultDeviceName
		}
		newHost.Token = ""
	} else {
		// The pairing code is stashed in Token and exchanged for a real token
		// during the login attempt (same convention as before).
		newHost.Token = settings[2].Options[0].Value.(string)
	}

	sel.Host = newHost
	return sel, nil
}

func LoginFlow(existingHost romm.Host) (*internal.Config, error) {
	screen := newLoginScreen()

	for {
		// Step 1: Server info
		host, cancelled, err := screen.drawServerInfo(existingHost)
		if err != nil {
			gabagool.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "login_error_unexpected", Other: "Something unexpected happened!\nCheck the logs for more info."}, nil), gabagool.ProcessMessageOptions{}, func() (interface{}, error) {
				time.Sleep(3 * time.Second)
				return nil, nil
			})
			return nil, fmt.Errorf("unable to get server information: %w", err)
		}
		if cancelled {
			gabagool.Close()
			os.Exit(0)
		}

		// Validate connection before asking for auth
		connResult := validateConnection(host)
		if !connResult.Result.Success {
			gabagool.ConfirmationMessage(
				i18n.Localize(connResult.Result.ErrorMsg, nil),
				ContinueFooter(),
				gabagool.MessageOptions{},
			)
			existingHost = host
			continue
		}

		// Step 2: Auth (loop until success or back)
		for {
			sel, authErr := screen.drawAuth(host, connResult.SupportsDeviceAuth)
			if authErr != nil {
				break
			}
			if sel.Cancelled {
				// Go back to server info
				existingHost = host
				break
			}

			loginOutput := attemptLogin(sel)

			if loginOutput.Result.Success {
				config := &internal.Config{
					Hosts: []romm.Host{loginOutput.Host},
				}
				_ = config.LoadPlatformsBinding(loginOutput.Host)
				return config, nil
			}

			// A cancelled device pairing has no message — loop back silently.
			if loginOutput.Result.ErrorMsg != nil {
				gabagool.ConfirmationMessage(
					i18n.Localize(loginOutput.Result.ErrorMsg, nil),
					ContinueFooter(),
					gabagool.MessageOptions{},
				)
			}
			host = loginOutput.Host
		}
	}
}

// connectionValidation reports server reachability plus whether the server is
// new enough (RomM 5.0+) to offer device-auth pairing.
type connectionValidation struct {
	Result             loginAttemptResult
	SupportsDeviceAuth bool
}

func validateConnection(host romm.Host) connectionValidation {
	validationClient := romm.NewClient(host.URL(), romm.WithInsecureSkipVerify(host.InsecureSkipVerify), romm.WithTimeout(internal.ValidationTimeout))

	result, _ := gabagool.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "login_validating_connection", Other: "Validating connection..."}, nil),
		gabagool.ProcessMessageOptions{},
		func() (connectionValidation, error) {
			if err := validationClient.ValidateConnection(); err != nil {
				return connectionValidation{Result: classifyLoginError(err)}, nil
			}
			supports := false
			if hb, err := validationClient.GetHeartbeat(); err == nil {
				supports = hb.SupportsDeviceAuth()
			}
			return connectionValidation{
				Result:             loginAttemptResult{Success: true},
				SupportsDeviceAuth: supports,
			}, nil
		},
	)

	return result
}

type loginAttemptOutput struct {
	Result loginAttemptResult
	Host   romm.Host
}

func attemptLogin(sel authSelection) loginAttemptOutput {
	if sel.Mode == authModeDevicePairing {
		return attemptDevicePairing(sel)
	}
	return attemptPairingCode(sel.Host)
}

// attemptDevicePairing runs the RomM 5.0+ device-auth pairing screen. A
// cancelled pairing returns no ErrorMsg so the flow loops back silently.
func attemptDevicePairing(sel authSelection) loginAttemptOutput {
	screen := NewDevicePairingScreen()
	result := screen.Execute(DevicePairingInput{Host: sel.Host, DeviceName: sel.DeviceName})

	switch result.Outcome {
	case DevicePairingSuccess:
		return loginAttemptOutput{Result: loginAttemptResult{Success: true}, Host: result.Host}
	case DevicePairingCancelled:
		return loginAttemptOutput{Result: loginAttemptResult{ErrorType: "cancelled"}, Host: result.Host}
	case DevicePairingDenied:
		return loginAttemptOutput{
			Result: loginAttemptResult{
				ErrorType: "denied",
				ErrorMsg:  &goi18n.Message{ID: "login_error_pairing_denied", Other: "Pairing was denied on the server."},
			},
			Host: result.Host,
		}
	case DevicePairingExpired:
		return loginAttemptOutput{
			Result: loginAttemptResult{
				ErrorType: "expired",
				ErrorMsg:  &goi18n.Message{ID: "login_error_pairing_expired", Other: "The pairing request expired.\nPlease try again."},
			},
			Host: result.Host,
		}
	default:
		if result.Err != nil {
			return loginAttemptOutput{Result: classifyLoginError(result.Err), Host: result.Host}
		}
		return loginAttemptOutput{
			Result: loginAttemptResult{
				ErrorType: "unknown",
				ErrorMsg:  &goi18n.Message{ID: "login_error_unexpected", Other: "Something unexpected happened!\nCheck the logs for more info."},
			},
			Host: result.Host,
		}
	}
}

// attemptPairingCode exchanges a typed pairing code for a token (pre-5.0 flow,
// still supported on 5.0+).
func attemptPairingCode(host romm.Host) loginAttemptOutput {
	if host.Token == "" {
		return loginAttemptOutput{
			Result: loginAttemptResult{
				ErrorType: "pairing",
				ErrorMsg:  &goi18n.Message{ID: "login_error_invalid_code", Other: "Invalid or expired pairing code.\nPlease try again."},
			},
			Host: host,
		}
	}

	result, _ := gabagool.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "login_validating", Other: "Logging in..."}, nil),
		gabagool.ProcessMessageOptions{},
		func() (loginAttemptOutput, error) {
			// Token field contains a pairing code — exchange it for a real token
			tokenResp, err := romm.ExchangeToken(host.URL(), host.Token, host.InsecureSkipVerify)
			if err != nil {
				return loginAttemptOutput{Result: classifyLoginError(err), Host: host}, nil
			}
			host.Token = tokenResp.RawToken
			host.TokenName = tokenResp.Name
			host.TokenExpiresAt = tokenResp.ExpiresAt

			if missing := romm.MissingSyncScopes(tokenResp.Scopes); len(missing) > 0 {
				gabagool.GetLogger().Warn("Paired token is missing scopes needed for save sync",
					"missing", missing, "granted", tokenResp.Scopes)
			}

			// Validate the token works
			client := romm.NewClientFromHost(host, internal.LoginTimeout)
			if err := client.ValidateToken(); err != nil {
				return loginAttemptOutput{Result: classifyLoginError(err), Host: host}, nil
			}

			// Fetch username for display purposes if not already known
			if host.Username == "" {
				if user, err := client.GetCurrentUser(); err == nil {
					host.Username = user.Username
				}
			}

			return loginAttemptOutput{Result: loginAttemptResult{Success: true}, Host: host}, nil
		},
	)

	return result
}

func classifyLoginError(err error) loginAttemptResult {
	if err == nil {
		return loginAttemptResult{Success: true}
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
	case errors.Is(err, romm.ErrUnauthorized):
		return loginAttemptResult{
			ErrorType: "credentials",
			ErrorMsg:  &goi18n.Message{ID: "login_error_credentials", Other: "Invalid credentials."},
		}
	case errors.Is(err, romm.ErrForbidden):
		return loginAttemptResult{
			ErrorType: "forbidden",
			ErrorMsg:  &goi18n.Message{ID: "login_error_forbidden", Other: "Access Forbidden!\nCheck your credentials and try switching between http and https."},
		}
	case errors.Is(err, romm.ErrServerError):
		return loginAttemptResult{
			ErrorType: "server",
			ErrorMsg:  &goi18n.Message{ID: "login_error_server", Other: "RomM server error!\nPlease check the RomM server logs."},
		}
	default:
		// Check if this is a token exchange error (API error with status code)
		errMsg := err.Error()
		if strings.Contains(errMsg, "status 404") || strings.Contains(errMsg, "status 429") {
			return loginAttemptResult{
				ErrorType: "pairing",
				ErrorMsg:  &goi18n.Message{ID: "login_error_invalid_code", Other: "Invalid or expired pairing code.\nPlease try again."},
			}
		}

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
