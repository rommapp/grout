package ui

import (
	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

func footerItem(button, msgID, fallback string) gaba.FooterHelpItem {
	return gaba.FooterHelpItem{
		ButtonName: button,
		HelpText:   i18n.Localize(&goi18n.Message{ID: msgID, Other: fallback}, nil),
	}
}

func FooterContinue() gaba.FooterHelpItem { return footerItem("A", "button_continue", "Continue") }
func FooterDownload() gaba.FooterHelpItem { return footerItem("A", "button_download", "Download") }
func FooterBack() gaba.FooterHelpItem     { return footerItem("B", "button_back", "Back") }
func FooterCancel() gaba.FooterHelpItem   { return footerItem("B", "button_cancel", "Cancel") }
func FooterQuit() gaba.FooterHelpItem     { return footerItem("B", "button_quit", "Quit") }

func FooterSave() gaba.FooterHelpItem {
	return footerItem(icons.Start, "button_save", "Save")
}

func FooterCycle() gaba.FooterHelpItem {
	return footerItem(icons.LeftRight, "button_cycle", "Cycle")
}

func ContinueFooter() []gaba.FooterHelpItem {
	return []gaba.FooterHelpItem{FooterContinue()}
}

func OptionsListFooter() []gaba.FooterHelpItem {
	return []gaba.FooterHelpItem{FooterCancel(), FooterCycle(), FooterSave()}
}
