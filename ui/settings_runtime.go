package ui

import (
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
)

// ApplyRuntimeSettings pushes the settings that change how the running app
// behaves into the UI toolkit.
//
// Here rather than in the settings package so that writing a settings file does
// not require a display, which is what kept romm from cross-compiling.
func ApplyRuntimeSettings(config *settings.Config) {
	gaba.SetRawLogLevel(string(config.LogLevel))
	if err := i18n.SetWithCode(config.Language); err != nil {
		gaba.GetLogger().Error("Failed to set language", "error", err, "language", config.Language)
	}
}
