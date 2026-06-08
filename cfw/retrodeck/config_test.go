package retrodeck

import (
	"os"
	"testing"
)

const testConfigJSON = `{
  "version": "0.10.8b",
  "paths": {
    "rd_home_path": "/home/user/retrodeck",
    "roms_path": "/home/user/retrodeck/roms",
    "saves_path": "/home/user/retrodeck/saves",
    "bios_path": "/home/user/retrodeck/bios",
    "downloaded_media_path": "/home/user/retrodeck/ES-DE/downloaded_media",
    "videos_path": "/home/user/retrodeck/videos",
    "states_path": "/home/user/retrodeck/states"
  },
  "options": {
    "cloud_saves": "false"
  }
}`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "retrodeck-*.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestParseConfig(t *testing.T) {
	path := writeTempConfig(t, testConfigJSON)

	paths, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	tests := []struct {
		field string
		got   string
		want  string
	}{
		{"RDHomePath", paths.RDHomePath, "/home/user/retrodeck"},
		{"RomsPath", paths.RomsPath, "/home/user/retrodeck/roms"},
		{"SavesPath", paths.SavesPath, "/home/user/retrodeck/saves"},
		{"BiosPath", paths.BiosPath, "/home/user/retrodeck/bios"},
		{"DownloadedMediaPath", paths.DownloadedMediaPath, "/home/user/retrodeck/ES-DE/downloaded_media"},
		{"VideosPath", paths.VideosPath, "/home/user/retrodeck/videos"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.field, tt.got, tt.want)
		}
	}
}

func TestParseConfig_IgnoresUnknownFields(t *testing.T) {
	path := writeTempConfig(t, testConfigJSON)

	// states_path is in the JSON but not in Paths — must not error
	_, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig should tolerate unknown fields: %v", err)
	}
}

func TestParseConfig_FileNotFound(t *testing.T) {
	_, err := ParseConfig("/nonexistent/path/retrodeck.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	path := writeTempConfig(t, `{ not valid json `)

	_, err := ParseConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadConfig(t *testing.T) {
	path := writeTempConfig(t, testConfigJSON)
	t.Setenv(configPathEnv, path)

	paths, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if paths.RomsPath != "/home/user/retrodeck/roms" {
		t.Errorf("RomsPath = %q, want %q", paths.RomsPath, "/home/user/retrodeck/roms")
	}
}

func TestLoadConfig_EnvVarNotSet(t *testing.T) {
	t.Setenv(configPathEnv, "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when env var not set, got nil")
	}
}
