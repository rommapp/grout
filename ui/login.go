package ui

import (
	"fmt"
	"grout/models"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/brandonkowalski/go-romm"
)

// LoginInput contains data needed to render the login screen
type LoginInput struct {
	ExistingHost models.Host
}

// LoginOutput contains the result of a successful login
type LoginOutput struct {
	Host   models.Host
	Config *models.Config
}

// LoginScreen handles user authentication to RomM
type LoginScreen struct{}

func NewLoginScreen() *LoginScreen {
	return &LoginScreen{}
}

func (s *LoginScreen) Draw(input LoginInput) (gabagool.ScreenResult[LoginOutput], error) {
	host := input.ExistingHost

	items := []gabagool.ItemWithOptions{
		{
			Item: gabagool.MenuItem{
				Text: "Protocol",
			},
			Options: []gabagool.Option{
				{DisplayName: "HTTP", Value: "http://"},
				{DisplayName: "HTTPS", Value: "https://"},
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
				Text: "Hostname",
			},
			Options: []gabagool.Option{
				{
					Type:           gabagool.OptionTypeKeyboard,
					DisplayName:    removeScheme(host.RootURI),
					KeyboardPrompt: removeScheme(host.RootURI),
					Value:          removeScheme(host.RootURI),
				},
			},
		},
		{
			Item: gabagool.MenuItem{
				Text: "Port (optional)",
			},
			Options: []gabagool.Option{
				{
					Type: gabagool.OptionTypeKeyboard,
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
				Text: "Username",
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
				Text: "Password",
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
		"Login to RomM",
		gabagool.OptionListSettings{
			DisableBackButton: false,
			FooterHelpItems: []gabagool.FooterHelpItem{
				{ButtonName: "B", HelpText: "Quit"},
				{ButtonName: "←→", HelpText: "Cycle"},
				{ButtonName: "Start", HelpText: "Login"},
			},
		},
		items,
	)

	// User pressed back/quit
	if err != nil {
		return gabagool.WithCode(LoginOutput{}, gabagool.ExitCodeCancel), nil
	}

	loginSettings := res.Items

	newHost := models.Host{
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

	return gabagool.Success(LoginOutput{Host: newHost}), nil
}

// LoginFlow handles the complete login flow including validation and retries
// This is a higher-level orchestrator that uses LoginScreen
func LoginFlow(existingHost models.Host) (*models.Config, error) {
	screen := NewLoginScreen()

	for {
		result, err := screen.Draw(LoginInput{ExistingHost: existingHost})
		if err != nil {
			gabagool.ProcessMessage("Something unexpected happened!\nCheck the logs for more info.", gabagool.ProcessMessageOptions{}, func() (interface{}, error) {
				time.Sleep(3 * time.Second)
				return nil, nil
			})
			return nil, fmt.Errorf("unable to get login information: %w", err)
		}

		// User quit
		if result.ExitCode == gabagool.ExitCodeBack || result.ExitCode == gabagool.ExitCodeCancel {
			os.Exit(1)
		}

		host := result.Value.Host

		// Attempt login
		loginResult := attemptLogin(host)

		switch {
		case loginResult.BadHost:
			gabagool.ConfirmationMessage("Could not connect to RomM!\nPlease check the hostname and port.",
				[]gabagool.FooterHelpItem{
					{ButtonName: "A", HelpText: "Continue"},
				},
				gabagool.MessageOptions{})
			existingHost = host // Retry with entered values
			continue

		case loginResult.BadCredentials:
			gabagool.ConfirmationMessage("Invalid Username or Password.",
				[]gabagool.FooterHelpItem{
					{ButtonName: "A", HelpText: "Continue"},
				},
				gabagool.MessageOptions{})
			existingHost = host // Retry with entered values
			continue
		}

		// Success
		config := &models.Config{
			Hosts: []models.Host{host},
		}
		return config, nil
	}
}

// loginAttemptResult holds the result of a login attempt
type loginAttemptResult struct {
	BadHost        bool
	BadCredentials bool
}

func attemptLogin(host models.Host) loginAttemptResult {
	rc := romm.NewClient(host.URL(), romm.WithTimeout(time.Second*15))

	result, _ := gabagool.ProcessMessage("Logging in...", gabagool.ProcessMessageOptions{}, func() (interface{}, error) {
		lr := rc.Login(host.Username, host.Password)
		if lr != nil {
			return loginAttemptResult{BadCredentials: true}, nil
		}
		return loginAttemptResult{}, nil
	})

	return result.(loginAttemptResult)
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
