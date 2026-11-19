package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadNewWhenNoFiles(t *testing.T) {
	dir := t.TempDir()
	c, src, err := Load(dir)
	if err != nil || src != SourceNew {
		t.Fatalf("src=%v err=%v", src, err)
	}
	if c.ApiTimeout.Duration() != 30*time.Second || c.Language != "en" {
		t.Fatalf("defaults not applied: %+v", c)
	}
}

func TestDefaultsMatchV2(t *testing.T) {
	c := &Config{ApiTimeout: DurationSeconds(600 * time.Second)}
	applyDefaults(c)
	if c.ApiTimeout.Duration() != 30*time.Second {
		t.Errorf("api timeout over 300s not reset: %v", c.ApiTimeout.Duration())
	}
	if c.DownloadTimeout.Duration() != 60*time.Minute {
		t.Errorf("download timeout %v", c.DownloadTimeout.Duration())
	}
	if c.LogLevel != LogLevelError || c.ReleaseChannel != ReleaseChannelMatchRomM || c.DownloadedGames != DownloadedGamesModeDoNothing || c.CollectionView != CollectionViewPlatform || c.ArtKind != ArtKindDefault {
		t.Errorf("enum defaults: %+v", c)
	}
}

func TestDurationSecondsLegacyNanoseconds(t *testing.T) {
	var d DurationSeconds
	if err := d.UnmarshalJSON([]byte("1800000000000")); err != nil || d.Duration() != 30*time.Minute {
		t.Fatalf("nanoseconds: %v %v", d.Duration(), err)
	}
	if err := d.UnmarshalJSON([]byte("45")); err != nil || d.Duration() != 45*time.Second {
		t.Fatalf("seconds: %v %v", d.Duration(), err)
	}
	b, _ := d.MarshalJSON()
	if string(b) != "45" {
		t.Fatalf("marshal %s", b)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	in := &Config{Hosts: []Host{{RootURI: "https://romm.local", Token: "t"}}, ChannelPreferences: map[int]string{42: "default"}}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, src, err := Load(dir)
	if err != nil || src != SourceSettings {
		t.Fatalf("src=%v err=%v", src, err)
	}
	if out.Hosts[0].Token != "t" || out.ChannelPreferences[42] != "default" {
		t.Fatalf("round trip lost data: %+v", out)
	}
}

func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, &Config{}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Fatalf("dir holds %v", entries)
	}
}

func TestCorruptSettingsIsAnErrorAndUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	os.WriteFile(path, []byte("{\"hosts\": ["), 0644)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"hosts":[{"root_uri":"x"}]}`), 0644)
	if _, _, err := Load(dir); err == nil {
		t.Fatal("expected error")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "{\"hosts\": [" {
		t.Fatal("corrupt settings.json was overwritten")
	}
}

func TestLogLevelSlog(t *testing.T) {
	if LogLevelDebug.Slog() != slog.LevelDebug || LogLevelInfo.Slog() != slog.LevelInfo || LogLevelError.Slog() != slog.LevelError || LogLevel("x").Slog() != slog.LevelError {
		t.Fatal("slog levels")
	}
}
