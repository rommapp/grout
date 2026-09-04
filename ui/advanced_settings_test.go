package ui

import (
	"testing"
	"time"

	"grout/settings"
)

func advancedRow(t *testing.T, key string) settingRow {
	t.Helper()
	for _, row := range advancedRows() {
		if row.key == key {
			return row
		}
	}
	t.Fatalf("no advanced settings row %q", key)
	return settingRow{}
}

// A row either changes a value or leads somewhere. One that did both would
// have its value read back on a press that was meant to navigate.
func TestAdvancedRows_AreEitherValueOrNavigation(t *testing.T) {
	for _, row := range advancedRows() {
		if row.leadsSomewhere() {
			if row.get != nil || row.set != nil {
				t.Errorf("%q navigates but also carries a value", row.key)
			}
			if _, leads := advancedDestinations[row.key]; !leads {
				t.Errorf("%q navigates but has no destination, so pressing it does nothing", row.key)
			}
			continue
		}
		if row.get == nil || row.set == nil {
			t.Errorf("%q holds a value but cannot read or write it", row.key)
		}
		if _, leads := advancedDestinations[row.key]; leads {
			t.Errorf("%q holds a value but is also listed as going somewhere", row.key)
		}
	}
}

func TestAdvancedRows_KeysAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, row := range advancedRows() {
		if row.key == "" {
			t.Errorf("%q has no key", row.label)
		}
		if seen[row.key] {
			t.Errorf("two rows share the key %q", row.key)
		}
		seen[row.key] = true
	}
}

// Opening the screen and saving without touching anything must leave every
// setting as it was.
func TestAdvancedSettings_RoundTrips(t *testing.T) {
	before := settings.Config{
		ApiTimeout:      settings.DurationSeconds(45 * time.Second),
		DownloadTimeout: settings.DurationSeconds(90 * time.Minute),
		ReleaseChannel:  settings.ReleaseChannelBeta,
		LogLevel:        settings.LogLevelDebug,
	}
	after := before

	rows := advancedRows()
	applySettingRows(rows, &after, settingItems(rows, before))

	if after.ApiTimeout != before.ApiTimeout {
		t.Errorf("ApiTimeout %v -> %v", before.ApiTimeout, after.ApiTimeout)
	}
	if after.DownloadTimeout != before.DownloadTimeout {
		t.Errorf("DownloadTimeout %v -> %v", before.DownloadTimeout, after.DownloadTimeout)
	}
	if after.ReleaseChannel != before.ReleaseChannel {
		t.Errorf("ReleaseChannel %v -> %v", before.ReleaseChannel, after.ReleaseChannel)
	}
	if after.LogLevel != before.LogLevel {
		t.Errorf("LogLevel %v -> %v", before.LogLevel, after.LogLevel)
	}
}

// The timeouts a screen offers used to be written twice, once as the options
// and once as a bare list used to find the selected one. Each row's default
// has to be somewhere in its own options or it opens on the wrong entry.
func TestAdvancedRows_OptionsCoverTheDefaults(t *testing.T) {
	for _, row := range advancedRows() {
		if row.leadsSomewhere() {
			continue
		}

		found := false
		for _, option := range row.options {
			if option.Value == row.def {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q defaults to %v, which none of its options offers", row.key, row.def)
		}
	}
}

// The defaults the screen names have to be the ones the settings package
// actually applies, or a config it has not reached opens on a different value
// than the app is using.
func TestAdvancedRows_DefaultsMatchTheSettingsPackage(t *testing.T) {
	fresh := freshConfig(t)

	tests := []struct {
		key  string
		want any
	}{
		{"download_timeout", fresh.DownloadTimeout.Duration()},
		{"api_timeout", fresh.ApiTimeout.Duration()},
		{"release_channel", fresh.ReleaseChannel},
		{"log_level", fresh.LogLevel},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := advancedRow(t, tt.key).def; got != tt.want {
				t.Errorf("row defaults to %v, settings defaults to %v", got, tt.want)
			}
		})
	}
}

// A value the options cannot show falls back to the package default rather
// than to whatever happens to be listed first.
func TestAdvancedSettings_UnrepresentableValueFallsBackToTheDefault(t *testing.T) {
	row := advancedRow(t, "api_timeout")

	// 20 seconds is not offered, and a hand-edited config can hold it.
	config := settings.Config{ApiTimeout: settings.DurationSeconds(20 * time.Second)}
	index := optionIndexOr(row.options, row.get(config), row.def)

	if row.options[index].Value != row.def {
		t.Errorf("opened on %v, want the %v default", row.options[index].Value, row.def)
	}
}
