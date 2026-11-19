package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestImportV2CarriesLoginAndOverrides(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{
		"hosts":[{"root_uri":"https://romm.local","port":8443,"token":"tok","device_id":"d1"}],
		"directory_mappings":{"gba":{"slug":"gba","relative_path":"GBA"}},
		"save_directory_mappings":{"gba":"mGBA"},
		"muos_rom_storage":"sdcard",
		"api_timeout":30000000000
	}`), 0644)
	os.WriteFile(filepath.Join(dir, "save_slots.json"), []byte(`{"12":"autosave","34":"desk","x":"bad"}`), 0644)

	c, src, err := Load(dir)
	if err != nil || src != SourceImportedV2 {
		t.Fatalf("src=%v err=%v", src, err)
	}
	if c.Hosts[0].Token != "tok" || c.Hosts[0].DeviceID != "d1" {
		t.Fatal("login not carried")
	}
	if c.DirectoryMappings["gba"].RelativePath != "GBA" || c.SaveDirectoryMappings["gba"] != "mGBA" || c.MuOSRomStorage != "sdcard" {
		t.Fatal("overrides not carried")
	}
	if c.ApiTimeout.Duration() != 30*time.Second {
		t.Fatalf("legacy nanoseconds: got %v", c.ApiTimeout.Duration())
	}
	want := map[int]string{12: "default", 34: "desk"}
	if !reflect.DeepEqual(c.ChannelPreferences, want) {
		t.Fatalf("channel prefs %v", c.ChannelPreferences)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatal("settings.json not written")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal("v2 config.json removed")
	}
}

func TestImportV2WithoutSlotsFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"language":"fr"}`), 0644)
	c, src, err := Load(dir)
	if err != nil || src != SourceImportedV2 || c.Language != "fr" || len(c.ChannelPreferences) != 0 {
		t.Fatalf("src=%v err=%v c=%+v", src, err, c)
	}
}

func TestImportV2RewritesOldPicklesSaveMappings(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"save_directory_mappings":{"gba":"pickles/sram/mGBA"}}`), 0644)
	c, _, err := Load(dir)
	if err != nil || c.SaveDirectoryMappings["gba"] != "../pickles/sram/mGBA" {
		t.Fatalf("err=%v got %q", err, c.SaveDirectoryMappings["gba"])
	}
}

func TestSecondLoadReadsSettings(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"language":"fr"}`), 0644)
	Load(dir)
	_, src, err := Load(dir)
	if err != nil || src != SourceSettings {
		t.Fatalf("src=%v err=%v", src, err)
	}
}

func TestSettingsWinsOverV2(t *testing.T) {
	dir := t.TempDir()
	Save(dir, &Config{Language: "de"})
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"language":"fr"}`), 0644)
	c, src, _ := Load(dir)
	if src != SourceSettings || c.Language != "de" {
		t.Fatalf("v2 re-imported: src=%v lang=%s", src, c.Language)
	}
}

func TestCorruptV2IsAnErrorAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0644)
	if _, _, err := Load(dir); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); !os.IsNotExist(err) {
		t.Fatal("settings.json written from corrupt v2 config")
	}
}
