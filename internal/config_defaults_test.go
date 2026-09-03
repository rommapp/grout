package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"grout/library"
)

func writeConfig(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Load and save used to default different fields. Log level and release
// channel were only set on save, so a config lacking them loaded with an empty
// log level and grout never applied one.
func TestLoadConfigFrom_AppliesEveryDefault(t *testing.T) {
	path := writeConfig(t, t.TempDir(), `{}`)

	config, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}

	checks := []struct {
		field     string
		got, want any
	}{
		{"Language", config.Language, "en"},
		{"LogLevel", config.LogLevel, LogLevelError},
		{"ReleaseChannel", config.ReleaseChannel, ReleaseChannelMatchRomM},
		{"DownloadedGames", config.DownloadedGames, DownloadedGamesModeDoNothing},
		{"CollectionView", config.CollectionView, CollectionViewPlatform},
		{"ArtKind", config.ArtKind, library.ArtKindDefault},
		{"AdditionalDownloads.Thumbnail", config.AdditionalDownloads.Thumbnail, library.ArtKindNone},
		{"AdditionalDownloads.Marquee", config.AdditionalDownloads.Marquee, library.ArtKindNone},
		{"ApiTimeout", config.ApiTimeout.Duration(), 30 * time.Second},
		{"DownloadTimeout", config.DownloadTimeout.Duration(), 60 * time.Minute},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
}

// Whatever load produces, save must accept unchanged. Any field the two
// disagree about shows up as a value that changes on a round trip.
func TestConfig_LoadSaveRoundTripIsStable(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{}`)

	loaded, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}
	before := *loaded

	if err := SaveConfigTo(loaded, path); err != nil {
		t.Fatalf("SaveConfigTo: %v", err)
	}

	reloaded, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	for _, c := range []struct {
		field                string
		before, after, saved any
	}{
		{"Language", before.Language, reloaded.Language, loaded.Language},
		{"LogLevel", before.LogLevel, reloaded.LogLevel, loaded.LogLevel},
		{"ReleaseChannel", before.ReleaseChannel, reloaded.ReleaseChannel, loaded.ReleaseChannel},
		{"ArtKind", before.ArtKind, reloaded.ArtKind, loaded.ArtKind},
		{"ApiTimeout", before.ApiTimeout, reloaded.ApiTimeout, loaded.ApiTimeout},
		{"DownloadTimeout", before.DownloadTimeout, reloaded.DownloadTimeout, loaded.DownloadTimeout},
	} {
		if c.before != c.after {
			t.Errorf("%s changed across a save/load round trip: %v then %v", c.field, c.before, c.after)
		}
	}
}

// Values the user chose must survive.
func TestLoadConfigFrom_KeepsStoredValues(t *testing.T) {
	path := writeConfig(t, t.TempDir(), `{
		"language": "fr",
		"log_level": "debug",
		"art_kind": "Box3D",
		"api_timeout": 45
	}`)

	config, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}
	if config.Language != "fr" {
		t.Errorf("Language = %q, want fr", config.Language)
	}
	if config.ArtKind != library.ArtKindBox3D {
		t.Errorf("ArtKind = %q, want Box3D", config.ArtKind)
	}
	if config.ApiTimeout.Duration() != 45*time.Second {
		t.Errorf("ApiTimeout = %v, want 45s", config.ApiTimeout.Duration())
	}
}

// The settings picker only offers up to 300s, so a larger stored value cannot
// be displayed and is reset rather than shown wrong.
func TestLoadConfigFrom_ClampsAnUnrepresentableTimeout(t *testing.T) {
	path := writeConfig(t, t.TempDir(), `{"api_timeout": 9000}`)

	config, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}
	if config.ApiTimeout.Duration() != 30*time.Second {
		t.Errorf("ApiTimeout = %v, want it reset to 30s", config.ApiTimeout.Duration())
	}
}

func TestLoadConfigFrom_MissingFile(t *testing.T) {
	if _, err := LoadConfigFrom(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected an error for a missing config")
	}
}

func TestLoadConfigFrom_MalformedJSON(t *testing.T) {
	path := writeConfig(t, t.TempDir(), `{not json`)
	if _, err := LoadConfigFrom(path); err == nil {
		t.Fatal("expected an error for malformed json")
	}
}

// A truncated config.json fails to parse, which grout treats as a first launch
// and discards hosts, credentials and directory mappings. The write must
// therefore leave either the old file or the new one, never a partial.
func TestSaveConfigTo_LeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{"language":"fr"}`)

	config, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}
	config.Language = "de"
	if err := SaveConfigTo(config, path); err != nil {
		t.Fatalf("SaveConfigTo: %v", err)
	}

	// The written file must be complete and parseable.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("saved config does not parse: %v", err)
	}
	if check["language"] != "de" {
		t.Errorf("language = %v, want de", check["language"])
	}

	// And no temp file may be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// Slot preferences live beside the config they belong to, so a test config in a
// temp directory does not read the developer's real save_slots.json.
func TestSlotPreferences_LiveBesideTheConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `{}`)

	if err := os.WriteFile(filepath.Join(dir, SlotPreferencesFileName), []byte(`{"42":"autosave"}`), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}
	if got := config.GetSlotPreference(42); got != "autosave" {
		t.Errorf("GetSlotPreference(42) = %q, want autosave", got)
	}
}

// Clearing every preference removes the file rather than leaving an empty
// object behind.
func TestSaveSlotPreferencesTo_RemovesTheFileWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SlotPreferencesFileName)
	if err := os.WriteFile(path, []byte(`{"42":"autosave"}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SaveSlotPreferencesTo(&Config{}, path); err != nil {
		t.Fatalf("SaveSlotPreferencesTo: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected the file to be removed, stat err = %v", err)
	}
}
