package ui

import (
	"grout/models"

	gaba "github.com/UncleJunVIP/gabagool/pkg/gabagool"
	"qlova.tech/sum"
)

type MainMenu struct {
	Hosts       models.Hosts
	HostIndices map[string]int
}

func InitMainMenu(hosts models.Hosts) MainMenu {
	hostIndices := make(map[string]int)
	for idx, host := range hosts {
		hostIndices[host.DisplayName] = idx
	}

	return MainMenu{
		Hosts:       hosts,
		HostIndices: hostIndices,
	}
}

func (m MainMenu) Name() sum.Int[models.ScreenName] {
	return models.ScreenNames.MainMenu
}

func (m MainMenu) Draw() (host interface{}, exitCode int, e error) {
	var menuItems []gaba.MenuItem
	for _, host := range m.Hosts {
		menuItems = append(menuItems, gaba.MenuItem{
			Text:     host.DisplayName,
			Selected: false,
			Focused:  false,
			Metadata: host,
		})
	}

	options := gaba.DefaultListOptions("Grout", menuItems)
	options.EnableAction = true
	options.FooterHelpItems = []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: "Quit"},
		{ButtonName: "X", HelpText: "Settings"},
		{ButtonName: "A", HelpText: "Select"},
	}

	selection, err := gaba.List(options)
	if err != nil {
		return models.Host{}, -1, err
	}

	if selection.IsSome() && selection.Unwrap().ActionTriggered {
		return models.Host{}, 4, nil
	} else if selection.IsSome() && !selection.Unwrap().ActionTriggered && selection.Unwrap().SelectedIndex != -1 {
		return selection.Unwrap().SelectedItem.Metadata.(models.Host), 0, nil
	}

	return models.Host{}, 2, nil
}
