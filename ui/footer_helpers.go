package ui

import (
	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

func FooterContinue() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "A",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_continue", Other: "Continue"}, nil),
	}
}

func FooterSelect() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "A",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_select", Other: "Select"}, nil),
	}
}

func FooterConfirm() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "A",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_confirm", Other: "Confirm"}, nil),
	}
}

func FooterBack() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "B",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil),
	}
}

func FooterCancel() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "B",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_cancel", Other: "Cancel"}, nil),
	}
}

func FooterClose() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "B",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_close", Other: "Close"}, nil),
	}
}

func FooterSearch() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "X",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_search", Other: "Search"}, nil),
	}
}

func FooterSettings() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "X",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_settings", Other: "Settings"}, nil),
	}
}

func FooterOptions() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "X",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_options", Other: "Options"}, nil),
	}
}

func FooterSave() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: icons.Start,
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_save", Other: "Save"}, nil),
	}
}

func FooterCycle() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: icons.LeftRight,
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_cycle", Other: "Cycle"}, nil),
	}
}

func FooterQuit() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "B",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_quit", Other: "Quit"}, nil),
	}
}

func FooterBIOS() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "Y",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_bios", Other: "BIOS"}, nil),
	}
}

func FooterSaveSync() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "Y",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_save_sync", Other: "Sync"}, nil),
	}
}

func FooterDownload() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "A",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_download", Other: "Download"}, nil),
	}
}

func FooterMenu() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "Start",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_menu", Other: "Menu"}, nil),
	}
}

func FooterStartConfirm() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "Start",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_confirm", Other: "Confirm"}, nil),
	}
}

func FooterLogout() gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: "X",
		HelpText:   i18n.Localize(&goi18n.Message{ID: "button_logout", Other: "Logout"}, nil),
	}
}

func ContinueFooter() []gaba.FooterHelpItem {
	return []gaba.FooterHelpItem{FooterContinue()}
}

func OptionsListFooter() []gaba.FooterHelpItem {
	return []gaba.FooterHelpItem{
		FooterCancel(),
		FooterCycle(),
		FooterSave(),
	}
}

func BackSelectFooter() []gaba.FooterHelpItem {
	return []gaba.FooterHelpItem{
		FooterBack(),
		FooterSelect(),
	}
}
