package ui

import (
	"os"
	"testing"

	"grout/resources"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

// Localize panics with no bundle behind it, so anything in this package that
// puts words on screen needs one before it can be tested. The real locale
// files are used, which also means a message whose English text drifts from
// the fallback written at the call site shows up here.
func TestMain(m *testing.M) {
	localeFiles, err := resources.GetLocaleMessageFiles()
	if err == nil {
		_ = i18n.InitI18NFromBytes(localeFiles)
	} else {
		_ = i18n.InitI18NFromBytes(nil)
	}

	os.Exit(m.Run())
}
