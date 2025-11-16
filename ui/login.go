package ui

import (
	"fmt"
	"grout/models"
	"strconv"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"qlova.tech/sum"
)

type Login struct {
}

func InitLogin() Login {
	return Login{}
}

func (l Login) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.Login
}

func (l Login) Draw() (newHost interface{}, exitCode int, e error) {
	items := []gaba.ItemWithOptions{
		{
			Item: gaba.MenuItem{
				Text: "Protocol",
			},
			Options: []gaba.Option{
				{DisplayName: "HTTP", Value: "http://"},
				{DisplayName: "HTTPS", Value: "https://"},
			},
		},
		{
			Item: gaba.MenuItem{
				Text: "Hostname",
			},
			Options: []gaba.Option{
				{
					Type:           gaba.OptionTypeKeyboard,
					KeyboardPrompt: "",
				},
			},
		},
		{
			Item: gaba.MenuItem{
				Text: "Port (optional)",
			},
			Options: []gaba.Option{
				{
					Type:           gaba.OptionTypeKeyboard,
					KeyboardPrompt: "",
				},
			},
		},
		{
			Item: gaba.MenuItem{
				Text: "Username",
			},
			Options: []gaba.Option{
				{
					Type:           gaba.OptionTypeKeyboard,
					KeyboardPrompt: "",
				},
			},
		},
		{
			Item: gaba.MenuItem{
				Text: "Password",
			},
			Options: []gaba.Option{
				{
					Type:           gaba.OptionTypeKeyboard,
					KeyboardPrompt: "",
					Masked:         true,
				},
			},
		},
	}

	footerHelpItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: "Quit"},
		{ButtonName: "Start", HelpText: "Login"},
	}

	res, err := gaba.OptionsList(
		"Login to RomM",
		items,
		footerHelpItems,
	)

	if err != nil {
		return nil, 1, err
	} else if res.IsNone() {
		return nil, 2, nil
	}

	loginSettings := res.Unwrap().Items

	var host models.Host

	host.RootURI = fmt.Sprintf("%s%s", loginSettings[0].Options[0].Value, loginSettings[1].Options[0].Value)
	host.Port = func(s string) int {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
		return 0
	}(loginSettings[2].Options[0].Value.(string))

	host.Username = loginSettings[3].Options[0].Value.(string)
	host.Password = loginSettings[4].Options[0].Value.(string)

	return host, 0, nil
}
