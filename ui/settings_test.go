package ui

import (
	"os"
	"path/filepath"
	"testing"

	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// freshConfig is what a first launch holds: an empty file with the settings
// package's own defaults filled in.
func freshConfig(t *testing.T) *settings.Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := settings.LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("loading a fresh config: %v", err)
	}
	return config
}

// The menu is read back by key, so a row without one goes nowhere and two
// rows sharing one send the user to the same screen.
func TestSettingsMenu_KeysAreUniqueAndPresent(t *testing.T) {
	seen := make(map[string]bool)
	for _, entry := range settingsMenu {
		if entry.key == "" {
			t.Errorf("%q has no key", entry.fallback)
		}
		if seen[entry.key] {
			t.Errorf("two rows share the key %q", entry.key)
		}
		seen[entry.key] = true
	}
}

// Every row goes somewhere different. Two sharing an action would make one of
// them unreachable.
func TestSettingsMenu_ActionsAreDistinct(t *testing.T) {
	seen := make(map[SettingsAction]string)
	for _, entry := range settingsMenu {
		if other, ok := seen[entry.action]; ok {
			t.Errorf("%q and %q both go to the same screen", other, entry.key)
		}
		seen[entry.action] = entry.key
	}
}

func TestSettingsAction(t *testing.T) {
	for _, entry := range settingsMenu {
		action, found := settingsAction(entry.key)
		if !found {
			t.Errorf("%q is in the menu but does not resolve", entry.key)
			continue
		}
		if action != entry.action {
			t.Errorf("%q resolved to the wrong screen", entry.key)
		}
	}

	if _, found := settingsAction("not_a_row"); found {
		t.Error("an unrecognised key must not resolve to a screen")
	}
}

func TestOptionIndexOr(t *testing.T) {
	options := []gaba.Option{{Value: "a"}, {Value: "b"}, {Value: "c"}}

	if got := optionIndexOr(options, "b", "c"); got != 1 {
		t.Errorf("got %d, want the value's own index", got)
	}
	if got := optionIndexOr(options, "missing", "c"); got != 2 {
		t.Errorf("got %d, want the fallback's index", got)
	}
	// Neither present is a mistake in the caller, and landing on the first
	// entry is the least surprising thing to do about it.
	if got := optionIndexOr(options, "missing", "also missing"); got != 0 {
		t.Errorf("got %d, want the first option", got)
	}
}

// A screen must be able to show the value a fresh config holds. When it
// cannot, it opens on something else and saving writes that instead, which is
// how a setting resets itself just by being looked at.
func TestOptionLists_CoverTheSettingsDefaults(t *testing.T) {
	fresh := freshConfig(t)

	tests := []struct {
		name    string
		options []gaba.Option
		value   any
	}{
		{"release channel", releaseChannelOptions(), fresh.ReleaseChannel},
		{"log level", logLevelOptions(), fresh.LogLevel},
		{"collection view", collectionViewOptions(), fresh.CollectionView},
		{"save backups", backupLimitOptions(), fresh.SaveBackupLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, option := range tt.options {
				if option.Value == tt.value {
					return
				}
			}
			t.Errorf("no option carries the default %v, so the row opens on something else", tt.value)
		})
	}
}

// The log level the screen falls back to has to be the one the rest of the app
// uses, or a config the defaults never reached logs at a different level than
// the screen says it does.
func TestLogLevelFallbackMatchesTheSettingsDefault(t *testing.T) {
	fresh := freshConfig(t)

	options := logLevelOptions()
	unset := optionIndexOr(options, settings.LogLevel(""), settings.LogLevelError)

	if options[unset].Value != fresh.LogLevel {
		t.Errorf("an unset log level opens on %v, but settings defaults it to %v",
			options[unset].Value, fresh.LogLevel)
	}
}
