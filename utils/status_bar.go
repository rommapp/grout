package utils

import (
	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

var defaultStatusBar = gaba.StatusBarOptions{
	Enabled:    true,
	ShowTime:   true,
	TimeFormat: gaba.TimeFormat24Hour,
	Icons: []gaba.StatusBarIcon{
		{
			Text: icons.WiFi,
		},
	},
}

func StatusBar() gaba.StatusBarOptions {
	return defaultStatusBar
}

func AddIcon(icon gaba.StatusBarIcon) {
	defaultStatusBar.Icons = append([]gaba.StatusBarIcon{icon}, defaultStatusBar.Icons...)
}
