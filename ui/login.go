package ui

import (
	"fmt"
	"grout/client"
	"grout/models"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"github.com/UncleJunVIP/nextui-pak-shared-functions/common"
	"qlova.tech/sum"
)

type LoginResult struct {
	BadHost        bool
	BadCredentials bool
}

type Login struct {
	Host models.Host
}

func (l Login) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.Login
}

func HandleLogin(existing models.Host) *models.Config {
	config := &models.Config{}
	l := Login{}
	l.Host = existing
	host, code, err := l.Draw()
	if err != nil || code == 1 {
		gabagool.ProcessMessage("Something unexpected happened!\nCheck the logs for more info.", gabagool.ProcessMessageOptions{}, func() (interface{}, error) {
			time.Sleep(3 * time.Second)
			return nil, nil
		})
		common.LogStandardFatal("Unable to get login information", err)
	} else if code == 2 {
		os.Exit(1)
	}

	rc := client.NewRomMClient(host.(models.Host))

	loginRe, _ := gabagool.ProcessMessage("Logging in...", gabagool.ProcessMessageOptions{}, func() (interface{}, error) {
		if !rc.Heartbeat() {
			return LoginResult{BadHost: true}, nil
		}

		if !rc.Login() {
			return LoginResult{BadCredentials: true}, nil
		}

		return LoginResult{}, nil
	})

	if loginRe.Result.(LoginResult).BadHost {
		gabagool.ConfirmationMessage("Could not connect to RomM!\nPlease check the hostname and port.",
			[]gabagool.FooterHelpItem{
				{ButtonName: "A", HelpText: "Continue"},
			},
			gabagool.MessageOptions{})
		return HandleLogin(host.(models.Host))
	} else if loginRe.Result.(LoginResult).BadCredentials {
		gabagool.ConfirmationMessage("Invalid Username or Password.",
			[]gabagool.FooterHelpItem{
				{ButtonName: "A", HelpText: "Continue"},
			},
			gabagool.MessageOptions{})
		return HandleLogin(host.(models.Host))
	}

	config.Hosts = append(config.Hosts, host.(models.Host))

	return config
}

func (l Login) Draw() (newHost interface{}, exitCode int, e error) {
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
				if strings.Contains(l.Host.RootURI, "https") {
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
					DisplayName:    removeScheme(l.Host.RootURI),
					KeyboardPrompt: removeScheme(l.Host.RootURI),
					Value:          removeScheme(l.Host.RootURI),
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
						if l.Host.Port == 0 {
							return ""
						}
						return strconv.Itoa(l.Host.Port)
					}(),
					DisplayName: func() string {
						if l.Host.Port == 0 {
							return ""
						}
						return strconv.Itoa(l.Host.Port)
					}(),
					Value: func() string {
						if l.Host.Port == 0 {
							return ""
						}
						return strconv.Itoa(l.Host.Port)
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
					DisplayName:    l.Host.Username,
					KeyboardPrompt: l.Host.Username,
					Value:          l.Host.Username,
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
					DisplayName:    l.Host.Password,
					KeyboardPrompt: l.Host.Password,
					Value:          l.Host.Password,
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

	if err != nil {
		return nil, 1, err
	} else if res.IsNone() {
		return nil, 2, nil
	}

	loginSettings := res.Unwrap().Items

	var host models.Host

	host.RootURI = fmt.Sprintf("%s%s", loginSettings[0].Value(), loginSettings[1].Value())
	host.Port = func(s string) int {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
		return 0
	}(loginSettings[2].Value().(string))

	host.Username = loginSettings[3].Options[0].Value.(string)
	host.Password = loginSettings[4].Options[0].Value.(string)

	return host, 0, nil
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
