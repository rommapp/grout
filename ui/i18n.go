package ui

import (
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// localize renders a message in the active language, falling back to the
// English text when there is no translation.
func localize(id, fallback string) string {
	return i18n.Localize(&goi18n.Message{ID: id, Other: fallback}, nil)
}

// localizeWith renders a message that takes template values.
func localizeWith(id, fallback string, data map[string]any) string {
	return i18n.Localize(&goi18n.Message{ID: id, Other: fallback}, data)
}
